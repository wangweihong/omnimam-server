package agentgrant

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const defaultTTL = time.Hour

// Codec 签发和解析不落库的短期 Agent 授权引用。
// 授权正文使用 AES-GCM 加密认证，避免稳定资源标识和授权边界暴露到 Task 参数。
type Codec struct {
	aead cipher.AEAD
	ttl  time.Duration
}

// NewCodec 使用已有服务身份密钥构造短期授权编解码器。
func NewCodec(secret string, ttl time.Duration) (*Codec, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("agent grant secret must be at least 32 bytes")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("construct agent grant cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("construct agent grant AEAD: %w", err)
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Codec{aead: aead, ttl: ttl}, nil
}

// Issue 加密 claims，并把授权类型前缀作为 GCM additional data 绑定。
func (c *Codec) Issue(prefix string, claims any) (string, error) {
	if c == nil || c.aead == nil {
		return "", fmt.Errorf("agent grant codec is required")
	}
	if strings.TrimSpace(prefix) == "" {
		return "", fmt.Errorf("agent grant prefix is required")
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode agent grant: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate agent grant nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, raw, []byte(prefix))
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Resolve 验证前缀和认证密文，并解析到指定 claims。
func (c *Codec) Resolve(reference, prefix string, claims any) error {
	if c == nil || c.aead == nil {
		return fmt.Errorf("agent grant codec is required")
	}
	if !strings.HasPrefix(reference, prefix) {
		return fmt.Errorf("agent grant reference has an invalid prefix")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(reference, prefix))
	if err != nil || len(sealed) < c.aead.NonceSize()+c.aead.Overhead() {
		return fmt.Errorf("agent grant reference is invalid")
	}
	nonce, ciphertext := sealed[:c.aead.NonceSize()], sealed[c.aead.NonceSize():]
	raw, err := c.aead.Open(nil, nonce, ciphertext, []byte(prefix))
	if err != nil {
		return fmt.Errorf("agent grant authentication failed")
	}
	if err := json.Unmarshal(raw, claims); err != nil {
		return fmt.Errorf("decode agent grant: %w", err)
	}
	return nil
}

// Window 返回当前签发时间和短期过期时间。
func (c *Codec) Window() (time.Time, time.Time) {
	now := time.Now().UTC()
	return now, now.Add(c.ttl)
}

// ModelAccessClaims 是不含 Provider 地址、配置和凭证的模型授权边界。
type ModelAccessClaims struct {
	OwnerUserID   string    `json:"owner_user_id"`
	AgentID       string    `json:"agent_id"`
	Usage         string    `json:"usage"`
	SourceType    string    `json:"source_type"`
	SourceRef     string    `json:"source_ref"`
	ModelID       string    `json:"model_id"`
	ConfigVersion int64     `json:"config_version"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// InvocationClaims 是一次 Invocation Task 可解析的执行授权边界。
type InvocationClaims struct {
	OwnerUserID             string    `json:"owner_user_id"`
	AgentID                 string    `json:"agent_id"`
	SessionID               string    `json:"session_id"`
	InvocationID            string    `json:"invocation_id"`
	RuntimeBindingID        string    `json:"runtime_binding_id"`
	InvocationType          string    `json:"invocation_type"`
	ExpectedResourceVersion int64     `json:"expected_resource_version"`
	StudioApplicationID     string    `json:"studio_application_id,omitempty"`
	WorkspaceID             string    `json:"workspace_id,omitempty"`
	BaseRevision            int64     `json:"base_revision,omitempty"`
	BaseCommitSHA           string    `json:"base_commit_sha,omitempty"`
	BlueprintVersion        string    `json:"blueprint_version,omitempty"`
	PromptKind              string    `json:"prompt_kind,omitempty"`
	ModelAccessGrantRef     string    `json:"model_access_grant_ref"`
	IssuedAt                time.Time `json:"issued_at"`
	ExpiresAt               time.Time `json:"expires_at"`
}

// RuntimeGitAccessClaims 是 Coding Runtime 的加密 Git 凭据载荷；它只能存在于短期授权引用中。
type RuntimeGitAccessClaims struct {
	OwnerUserID         string    `json:"owner_user_id"`
	AgentID             string    `json:"agent_id"`
	RuntimeID           string    `json:"runtime_id"`
	AgentGeneration     int64     `json:"agent_generation"`
	StudioApplicationID string    `json:"studio_application_id"`
	WorkspaceID         string    `json:"workspace_id"`
	GitLabProjectID     string    `json:"gitlab_project_id"`
	CloneURL            string    `json:"clone_url"`
	Username            string    `json:"username"`
	Token               string    `json:"token"`
	RemoteTokenID       int64     `json:"remote_token_id"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

// DeprecatedWorkspaceClaims is retained only for decoding old transient values during process drain.
type DeprecatedWorkspaceClaims struct {
	OwnerUserID         string    `json:"owner_user_id"`
	StudioApplicationID string    `json:"studio_application_id"`
	WorkspaceID         string    `json:"workspace_id"`
	AgentID             string    `json:"agent_id"`
	SessionID           string    `json:"session_id"`
	InvocationID        string    `json:"invocation_id"`
	InitialRevision     int64     `json:"initial_revision"`
	AllowedActions      []string  `json:"allowed_actions"`
	AllowedPathScopes   []string  `json:"allowed_path_scopes"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

// ValidateWindow 拒绝未来签发、过期或非法时间窗口。
func ValidateWindow(issuedAt, expiresAt time.Time) error {
	now := time.Now().UTC()
	if issuedAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(issuedAt) {
		return fmt.Errorf("agent grant validity window is invalid")
	}
	if issuedAt.After(now.Add(time.Minute)) {
		return fmt.Errorf("agent grant is not active")
	}
	if !expiresAt.After(now) {
		return fmt.Errorf("agent grant has expired")
	}
	return nil
}

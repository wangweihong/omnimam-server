package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

type OpenAICompatibleConfig struct {
	ID            string
	BaseURL       string
	APIKey        string
	CredentialRef string
	Capabilities  []Capability
	HTTPClient    *http.Client
}

type OpenAICompatibleAdapter struct {
	cfg OpenAICompatibleConfig
}

func NewOpenAICompatibleAdapter(cfg OpenAICompatibleConfig) *OpenAICompatibleAdapter {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	return &OpenAICompatibleAdapter{cfg: cfg}
}

func NewDeepSeekAdapter(apiKeyOrRef string) *OpenAICompatibleAdapter {
	return NewOpenAICompatibleAdapter(OpenAICompatibleConfig{
		ID:            "deepseek",
		BaseURL:       "https://api.deepseek.com",
		CredentialRef: apiKeyOrRef,
		Capabilities:  []Capability{"llm.chat", "query.parse", "prompt.generate", "asset.tagging"},
	})
}

func (a *OpenAICompatibleAdapter) ID() string {
	return a.cfg.ID
}

func (a *OpenAICompatibleAdapter) Capabilities() []Capability {
	return a.cfg.Capabilities
}

func (a *OpenAICompatibleAdapter) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	for k, v := range req.Params {
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	endpoint := strings.TrimRight(a.cfg.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey := a.apiKey(); apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := a.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.Errorf("provider request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	var decoded map[string]any
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, errors.WithStack(err)
	}
	content := extractOpenAIContent(decoded)
	return &InvokeResult{Content: content, Raw: decoded}, nil
}

func (a *OpenAICompatibleAdapter) apiKey() string {
	if a.cfg.APIKey != "" {
		return a.cfg.APIKey
	}
	ref := strings.TrimSpace(a.cfg.CredentialRef)
	if strings.HasPrefix(ref, "env:") {
		return os.Getenv(strings.TrimPrefix(ref, "env:"))
	}
	return ref
}

func extractOpenAIContent(decoded map[string]any) string {
	choices, ok := decoded["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	message, ok := first["message"].(map[string]any)
	if !ok {
		return ""
	}
	content, _ := message["content"].(string)
	return content
}

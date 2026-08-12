package options

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/pkg/app"
	cliflag "github.com/wangweihong/omnimam/backend/pkg/cli/flag"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"
)

var (
	_ app.PrintableOptions    = &Options{}
	_ app.CompleteableOptions = &Options{}
)

// Options runs a http server.
type Options struct {
	Name string `json:"name"`

	GenericServerRunOptions *genericoptions.ServerRunOptions       `json:"server"       mapstructure:"server"`
	Log                     *log.Options                           `json:"log"          mapstructure:"log"`
	FeatureOptions          *genericoptions.FeatureOptions         `json:"feature"      mapstructure:"feature"`
	InsecureServing         *genericoptions.InsecureServingOptions `json:"insecure"     mapstructure:"insecure"`
	SecureServing           *genericoptions.SecureServingOptions   `json:"secure"       mapstructure:"secure"`
	//PostgresSQLOptions      *genericoptions.PostgresSQLOptions     `json:"postgres" mapstructure:"postgres"`
	DatabaseOptions             *genericoptions.DatabaseOptions `json:"database"     mapstructure:"database"`
	AssetUploadOptions          *AssetUploadOptions             `json:"asset-upload" mapstructure:"asset-upload"`
	ApplicationPlatformOptions  *ApplicationPlatformOptions     `json:"application-platform" mapstructure:"application-platform"`
	AuthOptions                 *AuthOptions                    `json:"auth"             mapstructure:"auth"`
	WorkflowRuntimeOptions      *WorkflowRuntimeOptions         `json:"workflow-runtime" mapstructure:"workflow-runtime"`
	InfrastructureClientOptions *InfrastructureClientOptions    `json:"infrastructure-client" mapstructure:"infrastructure-client"`
	SSEOptions                  *SSEOptions                     `json:"sse" mapstructure:"sse"`
	MCPOptions                  *MCPOptions                     `json:"mcp" mapstructure:"mcp"`
	AppStudioOptions            *AppStudioOptions               `json:"appstudio" mapstructure:"appstudio"`
}

type AppStudioOptions struct {
	WebhookBaseURL string `json:"webhook-base-url" mapstructure:"webhook-base-url"`
}

func NewAppStudioOptions() *AppStudioOptions {
	return &AppStudioOptions{WebhookBaseURL: "http://127.0.0.1:8080"}
}

func (o *AppStudioOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.WebhookBaseURL, "appstudio.webhook-base-url", o.WebhookBaseURL, "public API base URL reachable by GitLab project hooks")
}

// AuthOptions 配置 Identity Access Token 签名和 OPAQUE 稳定部署密钥。
type AuthOptions struct {
	JWTSecret         string `json:"jwt-secret" mapstructure:"jwt-secret"`
	OpaqueServerSetup string `json:"opaque-server-setup" mapstructure:"opaque-server-setup"`
}

func NewAuthOptions() *AuthOptions {
	return &AuthOptions{}
}

// MCPOptions 控制已发布 MCP 传输限制、Origin、协议缓存和短期 Task/Upload 映射策略。
type MCPOptions struct {
	Enabled          bool          `json:"enabled" mapstructure:"enabled"`
	AllowedOrigins   []string      `json:"allowed-origins" mapstructure:"allowed-origins"`
	MaxRequestBytes  int64         `json:"max-request-bytes" mapstructure:"max-request-bytes"`
	RequestTimeout   time.Duration `json:"request-timeout" mapstructure:"request-timeout"`
	DiscoverTTL      time.Duration `json:"discover-ttl" mapstructure:"discover-ttl"`
	ResourceTTL      time.Duration `json:"resource-ttl" mapstructure:"resource-ttl"`
	TaskTTL          time.Duration `json:"task-ttl" mapstructure:"task-ttl"`
	TaskPollInterval time.Duration `json:"task-poll-interval" mapstructure:"task-poll-interval"`
	UploadTTL        time.Duration `json:"upload-ttl" mapstructure:"upload-ttl"`
	PublicBaseURL    string        `json:"public-base-url" mapstructure:"public-base-url"`
	RequestRate      int           `json:"request-rate-per-second" mapstructure:"request-rate-per-second"`
	RequestBurst     int           `json:"request-burst" mapstructure:"request-burst"`
	ToolRate         int           `json:"tool-rate-per-second" mapstructure:"tool-rate-per-second"`
	ToolBurst        int           `json:"tool-burst" mapstructure:"tool-burst"`
	MaxUploadBytes   int64         `json:"max-upload-bytes" mapstructure:"max-upload-bytes"`
	MaxLimiterScopes int           `json:"max-limiter-scopes" mapstructure:"max-limiter-scopes"`
}

func NewMCPOptions() *MCPOptions {
	return &MCPOptions{
		Enabled: true, MaxRequestBytes: 1 << 20, RequestTimeout: 30 * time.Second,
		DiscoverTTL: 5 * time.Minute, ResourceTTL: time.Minute, TaskTTL: 24 * time.Hour,
		TaskPollInterval: 2 * time.Second, UploadTTL: time.Hour,
		PublicBaseURL: "http://127.0.0.1:8080",
		RequestRate:   20, RequestBurst: 40, ToolRate: 10, ToolBurst: 20,
		MaxUploadBytes: 2 << 30, MaxLimiterScopes: 20000,
	}
}

func (o *MCPOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.Enabled, "mcp.enabled", o.Enabled, "enable the released MCP POST /mcp endpoint")
	fs.StringSliceVar(&o.AllowedOrigins, "mcp.allowed-origins", o.AllowedOrigins, "allowed browser origins; same-origin is always accepted")
	fs.Int64Var(&o.MaxRequestBytes, "mcp.max-request-bytes", o.MaxRequestBytes, "maximum MCP JSON-RPC request body size")
	fs.DurationVar(&o.RequestTimeout, "mcp.request-timeout", o.RequestTimeout, "timeout for one MCP request")
	fs.DurationVar(&o.DiscoverTTL, "mcp.discover-ttl", o.DiscoverTTL, "private server discovery cache lifetime")
	fs.DurationVar(&o.ResourceTTL, "mcp.resource-ttl", o.ResourceTTL, "private Resource projection cache lifetime")
	fs.DurationVar(&o.TaskTTL, "mcp.task-ttl", o.TaskTTL, "MCP Task Binding lifetime")
	fs.DurationVar(&o.TaskPollInterval, "mcp.task-poll-interval", o.TaskPollInterval, "suggested MCP Task poll interval")
	fs.DurationVar(&o.UploadTTL, "mcp.upload-ttl", o.UploadTTL, "reported controlled UploadSession lifetime")
	fs.StringVar(&o.PublicBaseURL, "mcp.public-base-url", o.PublicBaseURL, "public API base URL used for controlled upload/content links")
	fs.IntVar(&o.RequestRate, "mcp.request-rate-per-second", o.RequestRate, "per-principal MCP request rate per API instance")
	fs.IntVar(&o.RequestBurst, "mcp.request-burst", o.RequestBurst, "per-principal MCP request burst per API instance")
	fs.IntVar(&o.ToolRate, "mcp.tool-rate-per-second", o.ToolRate, "per-principal and Tool call rate per API instance")
	fs.IntVar(&o.ToolBurst, "mcp.tool-burst", o.ToolBurst, "per-principal and Tool call burst per API instance")
	fs.Int64Var(&o.MaxUploadBytes, "mcp.max-upload-bytes", o.MaxUploadBytes, "maximum size admitted for one MCP upload session")
	fs.IntVar(&o.MaxLimiterScopes, "mcp.max-limiter-scopes", o.MaxLimiterScopes, "maximum in-memory MCP limiter scopes per API instance")
}

// SSEOptions 控制用户事件保留、流式轮询、心跳和单实例连接上限。
type SSEOptions struct {
	Retention             time.Duration `json:"retention" mapstructure:"retention"`
	PollInterval          time.Duration `json:"poll-interval" mapstructure:"poll-interval"`
	HeartbeatInterval     time.Duration `json:"heartbeat-interval" mapstructure:"heartbeat-interval"`
	MaxConnectionsPerUser int           `json:"max-connections-per-user" mapstructure:"max-connections-per-user"`
}

func NewSSEOptions() *SSEOptions {
	return &SSEOptions{Retention: 24 * time.Hour, PollInterval: time.Second, HeartbeatInterval: 15 * time.Second, MaxConnectionsPerUser: 8}
}

func (o *SSEOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.Retention, "sse.retention", o.Retention, "retention duration for replayable user events")
	fs.DurationVar(&o.PollInterval, "sse.poll-interval", o.PollInterval, "database poll interval for active SSE streams")
	fs.DurationVar(&o.HeartbeatInterval, "sse.heartbeat-interval", o.HeartbeatInterval, "heartbeat interval for active SSE streams")
	fs.IntVar(&o.MaxConnectionsPerUser, "sse.max-connections-per-user", o.MaxConnectionsPerUser, "maximum active SSE streams per user on one API instance")
}

type AssetUploadOptions struct {
	ChunkTempDir      string `json:"chunk-temp-dir"      mapstructure:"chunk-temp-dir"`
	ChunkCleanupHours int    `json:"chunk-cleanup-hours" mapstructure:"chunk-cleanup-hours"`
}

// ApplicationPlatformOptions 配置 Application Platform 周期性运行参数。
type ApplicationPlatformOptions struct {
	EngineHealthInterval time.Duration `json:"engine-health-interval" mapstructure:"engine-health-interval"`
}

func NewApplicationPlatformOptions() *ApplicationPlatformOptions {
	return &ApplicationPlatformOptions{EngineHealthInterval: 30 * time.Second}
}

func (o *ApplicationPlatformOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.EngineHealthInterval, "application-platform.engine-health-interval", o.EngineHealthInterval,
		"interval for Task Center managed EngineInstance health checks; zero disables automatic checks")
}

// WorkflowRuntimeOptions configures the internal Conductor boundary used by Task Center.
type WorkflowRuntimeOptions struct {
	Enabled           bool          `json:"enabled" mapstructure:"enabled"`
	BaseURL           string        `json:"base-url" mapstructure:"base-url"`
	AuthKey           string        `json:"auth-key" mapstructure:"auth-key"`
	AuthSecret        string        `json:"auth-secret" mapstructure:"auth-secret"`
	HTTPTimeout       time.Duration `json:"http-timeout" mapstructure:"http-timeout"`
	PollInterval      time.Duration `json:"poll-interval" mapstructure:"poll-interval"`
	ReconcileInterval time.Duration `json:"reconcile-interval" mapstructure:"reconcile-interval"`
}

func NewWorkflowRuntimeOptions() *WorkflowRuntimeOptions {
	return &WorkflowRuntimeOptions{HTTPTimeout: 30 * time.Second, PollInterval: 250 * time.Millisecond, ReconcileInterval: 15 * time.Second}
}

func (o *WorkflowRuntimeOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.Enabled, "workflow-runtime.enabled", o.Enabled, "enable the Conductor workflow runtime")
	fs.StringVar(&o.BaseURL, "workflow-runtime.base-url", o.BaseURL, "Conductor API base URL")
	fs.StringVar(&o.AuthKey, "workflow-runtime.auth-key", o.AuthKey, "Conductor authentication key")
	fs.StringVar(&o.AuthSecret, "workflow-runtime.auth-secret", o.AuthSecret, "Conductor authentication secret")
	fs.DurationVar(&o.HTTPTimeout, "workflow-runtime.http-timeout", o.HTTPTimeout, "Conductor HTTP timeout")
	fs.DurationVar(&o.PollInterval, "workflow-runtime.poll-interval", o.PollInterval, "Conductor worker poll interval")
	fs.DurationVar(&o.ReconcileInterval, "workflow-runtime.reconcile-interval", o.ReconcileInterval, "runtime projection reconcile interval")
}

// InfrastructureClientOptions configures Task Worker access to the Infrastructure service.
type InfrastructureClientOptions struct {
	BaseURL string `json:"base-url" mapstructure:"base-url"`
	Token   string `json:"token" mapstructure:"token"`
}

func NewInfrastructureClientOptions() *InfrastructureClientOptions {
	return &InfrastructureClientOptions{}
}

func (o *InfrastructureClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BaseURL, "infrastructure-client.base-url", o.BaseURL, "Infrastructure service base URL")
	fs.StringVar(&o.Token, "infrastructure-client.token", o.Token, "Infrastructure service token")
}

func NewAssetUploadOptions() *AssetUploadOptions {
	return &AssetUploadOptions{ChunkCleanupHours: 24}
}

func (o *AssetUploadOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ChunkTempDir, "asset-upload.chunk-temp-dir", o.ChunkTempDir, "resumable upload chunk temp dir")
	fs.IntVar(
		&o.ChunkCleanupHours,
		"asset-upload.chunk-cleanup-hours",
		o.ChunkCleanupHours,
		"hours before unused resumable upload chunk dirs are cleaned",
	)
}

// NewOptions creates a new Options object with default parameters.
func NewOptions() *Options {
	s := Options{
		Name: "api-server",

		Log:                     log.NewOptions(),
		InsecureServing:         genericoptions.NewInsecureServingOptions(),
		SecureServing:           genericoptions.NewSecureServingOptions(),
		FeatureOptions:          genericoptions.NewFeatureOptions(),
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		//PostgresSQLOptions:      genericoptions.NewPostgresSQLOptions(),
		DatabaseOptions:             genericoptions.NewDatabaseOptions(),
		AssetUploadOptions:          NewAssetUploadOptions(),
		ApplicationPlatformOptions:  NewApplicationPlatformOptions(),
		AuthOptions:                 NewAuthOptions(),
		WorkflowRuntimeOptions:      NewWorkflowRuntimeOptions(),
		InfrastructureClientOptions: NewInfrastructureClientOptions(),
		SSEOptions:                  NewSSEOptions(),
		MCPOptions:                  NewMCPOptions(),
		AppStudioOptions:            NewAppStudioOptions(),
	}

	return &s
}

// Flags returns flags for a specific server by section name.
func (o *Options) Flags() (fss cliflag.NamedFlagSets) {
	o.Log.AddFlags(fss.FlagSet("logs"))
	// 这里会将以下标志集归类到generic server集合中
	o.GenericServerRunOptions.AddFlags(fss.FlagSet("generic server"))
	o.InsecureServing.AddFlags(fss.FlagSet("server"))
	o.SecureServing.AddFlags(fss.FlagSet("server"))
	o.FeatureOptions.AddFlags(fss.FlagSet("feature"))
	//o.PostgresSQLOptions.AddFlags(fss.FlagSet("database"))
	o.DatabaseOptions.AddFlags(fss.FlagSet("database"))
	o.AssetUploadOptions.AddFlags(fss.FlagSet("asset upload"))
	o.ApplicationPlatformOptions.AddFlags(fss.FlagSet("application platform"))
	o.WorkflowRuntimeOptions.AddFlags(fss.FlagSet("workflow runtime"))
	o.InfrastructureClientOptions.AddFlags(fss.FlagSet("infrastructure client"))
	o.SSEOptions.AddFlags(fss.FlagSet("sse"))
	o.MCPOptions.AddFlags(fss.FlagSet("mcp"))
	o.AppStudioOptions.AddFlags(fss.FlagSet("appstudio"))
	fs := fss.FlagSet("authentication")
	fs.StringVar(&o.AuthOptions.JWTSecret, "auth.jwt-secret", o.AuthOptions.JWTSecret, "signing secret for Identity access tokens")
	fs.StringVar(&o.AuthOptions.OpaqueServerSetup, "auth.opaque-server-setup", o.AuthOptions.OpaqueServerSetup, "stable OPAQUE server key material encoded as hex")

	fs = fss.FlagSet("misc")
	fs.StringVar(&o.Name, "misc.name", o.Name, "name of server")
	return fss
}

func (o *Options) String() string {
	// hide annoying cert data in log
	cert := o.SecureServing.ServerCert.CopyAndHide()
	data, _ := json.Marshal(o)
	o.SecureServing.ServerCert = *cert

	return string(data)
}

// Complete fills in any fields not set that are required to have valid data.
// 补全指定的选项.
func (o *Options) Complete() error {
	if err := o.SecureServing.Complete(); err != nil {
		return err
	}
	if o.AssetUploadOptions == nil {
		o.AssetUploadOptions = NewAssetUploadOptions()
	}
	if o.AssetUploadOptions.ChunkCleanupHours <= 0 {
		o.AssetUploadOptions.ChunkCleanupHours = 24
	}
	if o.ApplicationPlatformOptions == nil {
		o.ApplicationPlatformOptions = NewApplicationPlatformOptions()
	}
	if o.AuthOptions == nil {
		o.AuthOptions = NewAuthOptions()
	}
	if len([]byte(o.AuthOptions.JWTSecret)) < 32 {
		return fmt.Errorf("auth JWT secret must be at least 32 bytes")
	}
	if o.WorkflowRuntimeOptions == nil {
		o.WorkflowRuntimeOptions = NewWorkflowRuntimeOptions()
	}
	if o.InfrastructureClientOptions == nil {
		o.InfrastructureClientOptions = NewInfrastructureClientOptions()
	}
	if o.WorkflowRuntimeOptions.Enabled {
		if strings.TrimSpace(o.InfrastructureClientOptions.BaseURL) == "" {
			return fmt.Errorf("infrastructure base url is required")
		}
		if len(o.InfrastructureClientOptions.Token) < 32 {
			return fmt.Errorf("infrastructure service token must be at least 32 bytes")
		}
	}
	if o.SSEOptions == nil {
		o.SSEOptions = NewSSEOptions()
	}
	if o.MCPOptions == nil {
		o.MCPOptions = NewMCPOptions()
	}
	if o.MCPOptions.MaxRequestBytes < 1024 || o.MCPOptions.MaxRequestBytes > 16<<20 ||
		o.MCPOptions.RequestTimeout <= 0 || o.MCPOptions.DiscoverTTL <= 0 || o.MCPOptions.ResourceTTL <= 0 ||
		o.MCPOptions.DiscoverTTL > time.Hour || o.MCPOptions.ResourceTTL > 5*time.Minute ||
		o.MCPOptions.TaskTTL <= 0 || o.MCPOptions.TaskPollInterval < 100*time.Millisecond || o.MCPOptions.UploadTTL <= 0 ||
		o.MCPOptions.RequestRate < 1 || o.MCPOptions.RequestBurst < 1 || o.MCPOptions.ToolRate < 1 ||
		o.MCPOptions.ToolBurst < 1 || o.MCPOptions.MaxUploadBytes < 1 || o.MCPOptions.MaxLimiterScopes < 1 {
		return fmt.Errorf("MCP limits, timeout, cache TTLs, task TTL, poll interval, and upload TTL are invalid")
	}
	if o.AppStudioOptions == nil {
		o.AppStudioOptions = NewAppStudioOptions()
	}
	webhookBaseURL, err := normalizeHTTPOrigin(o.AppStudioOptions.WebhookBaseURL, "AppStudio webhook base URL")
	if err != nil {
		return err
	}
	o.AppStudioOptions.WebhookBaseURL = webhookBaseURL
	publicBaseURL, err := normalizeMCPBaseURL(o.MCPOptions.PublicBaseURL)
	if err != nil {
		return err
	}
	o.MCPOptions.PublicBaseURL = publicBaseURL
	for index, origin := range o.MCPOptions.AllowedOrigins {
		normalized, err := normalizeMCPBaseURL(origin)
		if err != nil {
			return fmt.Errorf("invalid MCP allowed origin: %w", err)
		}
		o.MCPOptions.AllowedOrigins[index] = normalized
	}
	if o.SSEOptions.Retention <= 0 || o.SSEOptions.PollInterval <= 0 || o.SSEOptions.HeartbeatInterval <= 0 || o.SSEOptions.MaxConnectionsPerUser <= 0 {
		return fmt.Errorf("SSE retention, intervals, and connection limit must be positive")
	}
	return nil
}

func normalizeHTTPOrigin(raw, name string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an absolute HTTP origin", name)
	}
	parsed.Path = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeMCPBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("MCP public base URL and allowed origins must be absolute HTTP origins")
	}
	if parsed.Scheme == "http" && !isLocalMCPHost(parsed.Hostname()) {
		return "", fmt.Errorf("remote MCP public base URL and allowed origins must use HTTPS")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isLocalMCPHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

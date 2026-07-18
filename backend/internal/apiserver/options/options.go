package options

import (
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
	DatabaseOptions            *genericoptions.DatabaseOptions `json:"database"     mapstructure:"database"`
	AssetUploadOptions         *AssetUploadOptions             `json:"asset-upload" mapstructure:"asset-upload"`
	ApplicationPlatformOptions *ApplicationPlatformOptions     `json:"application-platform" mapstructure:"application-platform"`
	AuthOptions                *AuthOptions                    `json:"auth"             mapstructure:"auth"`
	WorkflowRuntimeOptions     *WorkflowRuntimeOptions         `json:"workflow-runtime" mapstructure:"workflow-runtime"`
}

// AuthOptions 控制用户系统完成前的开发态认证兼容路径。
type AuthOptions struct {
	AllowAnonymousDevelopment bool `json:"allow-anonymous-development" mapstructure:"allow-anonymous-development"`
}

func NewAuthOptions() *AuthOptions { return &AuthOptions{} }

type AssetUploadOptions struct {
	ChunkTempDir      string `json:"chunk-temp-dir"      mapstructure:"chunk-temp-dir"`
	ChunkCleanupHours int    `json:"chunk-cleanup-hours" mapstructure:"chunk-cleanup-hours"`
}

// ApplicationPlatformOptions configures the immutable provider capability snapshot loaded at startup.
type ApplicationPlatformOptions struct {
	ProviderCapabilityDirectory string        `json:"provider-capability-directory" mapstructure:"provider-capability-directory"`
	EngineHealthInterval        time.Duration `json:"engine-health-interval" mapstructure:"engine-health-interval"`
}

func NewApplicationPlatformOptions() *ApplicationPlatformOptions {
	return &ApplicationPlatformOptions{ProviderCapabilityDirectory: "./provider-capabilities", EngineHealthInterval: 30 * time.Second}
}

func (o *ApplicationPlatformOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ProviderCapabilityDirectory, "application-platform.provider-capability-directory", o.ProviderCapabilityDirectory,
		"directory containing immutable ProviderCapability YAML manifests")
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
		DatabaseOptions:            genericoptions.NewDatabaseOptions(),
		AssetUploadOptions:         NewAssetUploadOptions(),
		ApplicationPlatformOptions: NewApplicationPlatformOptions(),
		AuthOptions:                NewAuthOptions(),
		WorkflowRuntimeOptions:     NewWorkflowRuntimeOptions(),
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
	fs := fss.FlagSet("authentication")
	fs.BoolVar(&o.AuthOptions.AllowAnonymousDevelopment, "auth.allow-anonymous-development", o.AuthOptions.AllowAnonymousDevelopment,
		"allow anonymous development authentication for unfinished user management")

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
	if o.WorkflowRuntimeOptions == nil {
		o.WorkflowRuntimeOptions = NewWorkflowRuntimeOptions()
	}
	if o.ApplicationPlatformOptions.ProviderCapabilityDirectory == "" {
		o.ApplicationPlatformOptions.ProviderCapabilityDirectory = "./provider-capabilities"
	}

	return nil
}

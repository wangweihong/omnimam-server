package options

import (
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
	ProviderCapabilityDirectory string `json:"provider-capability-directory" mapstructure:"provider-capability-directory"`
}

func NewApplicationPlatformOptions() *ApplicationPlatformOptions {
	return &ApplicationPlatformOptions{ProviderCapabilityDirectory: "./provider-capabilities"}
}

func (o *ApplicationPlatformOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ProviderCapabilityDirectory, "application-platform.provider-capability-directory", o.ProviderCapabilityDirectory,
		"directory containing immutable ProviderCapability YAML manifests")
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
	if o.ApplicationPlatformOptions.ProviderCapabilityDirectory == "" {
		o.ApplicationPlatformOptions.ProviderCapabilityDirectory = "./provider-capabilities"
	}

	return nil
}

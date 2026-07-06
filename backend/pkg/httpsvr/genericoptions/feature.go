package genericoptions

import (
	"fmt"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"

	"github.com/spf13/pflag"
)

// FeatureOptions contains configuration items related to server features.
type FeatureOptions struct {
	// profile
	EnableProfiling     bool   `json:"profiling"            mapstructure:"profiling"`            // 是否安装/debug/prof/* apis
	StandAloneProfiling bool   `json:"standalone-profiling" mapstructure:"standalone-profiling"` // prof api是否采用独立的服务
	ProfileAddress      string `json:"profile-address"      mapstructure:"profile-address"`      // prof地址,采取独立服务时需指定
	// metrics
	EnableMetrics bool `json:"enable-metrics"       mapstructure:"enable-metrics"` // 是否启动/metrics apis
}

// NewFeatureOptions creates a FeatureOptions object with default parameters.
func NewFeatureOptions() *FeatureOptions {
	defaults := httpsvr.NewConfig()

	return &FeatureOptions{
		EnableMetrics:       defaults.EnableMetrics,
		StandAloneProfiling: defaults.Profiling.StandAloneProfiling,
		EnableProfiling:     defaults.Profiling.EnableProfiling,
		ProfileAddress:      defaults.Profiling.ProfileAddress,
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
func (o *FeatureOptions) ApplyTo(c *httpsvr.Config) error {
	c.Profiling = &httpsvr.FeatureProfilingInfo{
		EnableProfiling:     o.EnableProfiling,
		StandAloneProfiling: o.StandAloneProfiling,
		ProfileAddress:      o.ProfileAddress,
	}
	c.EnableMetrics = o.EnableMetrics

	return nil
}

// Validate is used to parse and validate the parameters entered by the user at
// the command line when the program starts.
func (o *FeatureOptions) Validate() []error {
	var errs []error

	if o.EnableProfiling && o.StandAloneProfiling {
		if o.ProfileAddress == "" {
			errs = append(errs, fmt.Errorf("feature.profiling-address  must not be empty when"+
				"feature.enable-profiling and feature.standalone-profiling enable"))
		}
	}
	return errs
}

// AddFlags adds flags related to features for a specific apis server to the
// specified FlagSet.
func (o *FeatureOptions) AddFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}

	fs.BoolVar(&o.EnableProfiling, "feature.enable-profiling", o.EnableProfiling,
		"Enable profiling")
	fs.BoolVar(&o.StandAloneProfiling, "feature.standalone-profiling", o.StandAloneProfiling,
		"if false,Enable profiling via web interface host:port/debug/pprof/. "+
			"Otherwise profiling enable via a standalone server")
	fs.StringVar(&o.ProfileAddress, "feature.profiling-address", o.ProfileAddress,
		"standalone profiling server address.")

	fs.BoolVar(&o.EnableMetrics, "feature.enable-metrics", o.EnableMetrics,
		"Enables metrics on the apiserver at /metrics")
}

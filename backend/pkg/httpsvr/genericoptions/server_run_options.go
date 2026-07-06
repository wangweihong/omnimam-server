package genericoptions

import (
	"fmt"
	"strings"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"

	"github.com/wangweihong/gotoolbox/pkg/debug"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	"github.com/spf13/pflag"

	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"
)

// ServerRunOptions contains the options while running a generic apis server.
type ServerRunOptions struct {
	Mode        string   `json:"mode"        mapstructure:"mode"`        // GIN服务模式
	Version     bool     `json:"version"     mapstructure:"version"`     // 开启版本模式
	Healthz     bool     `json:"healthz"     mapstructure:"healthz"`     // 开启healthz服务
	Middlewares []string `json:"middlewares" mapstructure:"middlewares"` // 安装的通用中间件

	RuntimeDebug    bool   `json:"runtime-debug"     mapstructure:"runtime-debug"`     // 开启运行时调试
	RuntimeDebugDir string `json:"runtime-debug-dir" mapstructure:"runtime-debug-dir"` // 调试输出目录
}

// NewServerRunOptions creates a new ServerRunOptions object with default parameters.
func NewServerRunOptions() *ServerRunOptions {
	defaults := httpsvr.NewConfig()

	return &ServerRunOptions{
		Mode:            defaults.Mode,
		Healthz:         defaults.Healthz,
		Middlewares:     defaults.Middlewares,
		Version:         defaults.Version,
		RuntimeDebug:    defaults.RuntimeDebug.Enable,
		RuntimeDebugDir: defaults.RuntimeDebug.OutputDir,
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
func (s *ServerRunOptions) ApplyTo(c *httpsvr.Config) error {
	c.Mode = s.Mode
	c.Healthz = s.Healthz
	c.Middlewares = s.Middlewares
	c.Version = s.Version
	c.RuntimeDebug = &debug.RuntimeDebugInfo{
		Enable:    s.RuntimeDebug,
		OutputDir: s.RuntimeDebugDir,
	}

	return nil
}

// Validate checks validation of ServerRunOptions.
func (s *ServerRunOptions) Validate() []error {
	errors := []error{}

	if !sets.NewString("debug", "test", "release").Has(s.Mode) {
		errors = append(errors, fmt.Errorf("server.mode must be `debug`,`test` or `release`"))
	}

	rm, repeated := sliceutil.StringSlice(s.Middlewares).GetRepeat()
	if repeated {
		errors = append(errors, fmt.Errorf("middleware `%v` is repeated", maputil.StringInt(rm).Keys()))
	}

	supportedMiddleware := sets.NewString(genericmiddleware.MiddlewareNames...)
	if !supportedMiddleware.HasAll(s.Middlewares...) {
		invalidMiddleware := sets.NewString(s.Middlewares...).Difference(supportedMiddleware)
		errors = append(errors, fmt.Errorf("middleware `%v` is not supported", invalidMiddleware.List()))
	}

	if s.RuntimeDebug {
		if s.RuntimeDebugDir == "" {
			errors = append(errors, fmt.Errorf("set `RuntimeDebugDir` when enable runtime debug"))
		}
	}

	return errors
}

// AddFlags adds flags for a specific APIServer to the specified FlagSet.
func (s *ServerRunOptions) AddFlags(fs *pflag.FlagSet) {
	// Note: the weird ""+ in below lines seems to be the only way to get gofmt to
	// arrange these text blocks sensibly. Grrr.
	fs.StringVar(&s.Mode, "server.mode", s.Mode, ""+
		"Start the server in a specified server mode. Supported server mode: debug, test, release.")

	fs.BoolVar(&s.Healthz, "server.healthz", s.Healthz, ""+
		"Add self readiness check and install /healthz router.")

	fs.BoolVar(&s.Version, "server.version", s.Version, ""+
		"Install /version router.")

	fs.StringSliceVar(&s.Middlewares, "server.middlewares", s.Middlewares, ""+
		"List of allowed middleware for server, comma separated. If this list is empty,no middlewares will be used."+
		"Support middleware: "+strings.Join(genericmiddleware.MiddlewareNames, ","))

	fs.BoolVar(&s.RuntimeDebug, "server.runtime-debug", s.RuntimeDebug, ""+
		"Enable debugging during runtime.")

	fs.StringVar(&s.RuntimeDebugDir, "server.runtime-debug-dir", s.RuntimeDebugDir, ""+
		"Directory runtime debug data saved")
}

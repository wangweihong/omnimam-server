package genericoptions

import (
	"fmt"
	"net"
	"strconv"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"

	"github.com/spf13/pflag"
)

// InsecureServingOptions are for creating an unauthenticated, unauthorized, insecure port.
type InsecureServingOptions struct {
	BindAddress string `json:"bind-address" mapstructure:"bind-address"`
	BindPort    int    `json:"bind-port"    mapstructure:"bind-port"`
	Required    bool   `json:"required"     mapstructure:"required"`
}

// NewInsecureServingOptions is for creating an unauthenticated, unauthorized, insecure port.
func NewInsecureServingOptions() *InsecureServingOptions {
	return &InsecureServingOptions{
		BindAddress: "127.0.0.1",
		BindPort:    8080,
		Required:    true,
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
func (s *InsecureServingOptions) ApplyTo(c *httpsvr.Config) error {
	c.InsecureServing = &httpsvr.InsecureServingInfo{
		Address:  net.JoinHostPort(s.BindAddress, strconv.Itoa(s.BindPort)),
		Required: s.Required,
	}

	return nil
}

// Validate is used to parse and validate the parameters entered by the user at
// the command line when the program starts.
func (s *InsecureServingOptions) Validate() []error {
	var errors []error

	if s.Required {
		if s.BindPort < 1 || s.BindPort > 65535 {
			errors = append(
				errors,
				fmt.Errorf(
					"--insecure.bind-port %v must be between 1 and 65535",
					s.BindPort,
				),
			)
		}
	}
	return errors
}

// AddFlags adds flags related to features for a specific server to the
// specified FlagSet.
func (s *InsecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindAddress, "insecure.bind-address", s.BindAddress, ""+
		"The IP address on which to serve the --insecure.bind-port "+
		"(set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")
	fs.IntVar(&s.BindPort, "insecure.bind-port", s.BindPort, ""+
		"The port on which to serve unsecured, unauthenticated access. 0 for turning off insecure (HTTP) port.")
	fs.BoolVar(&s.Required, "insecure.required", s.Required,
		"Whether require insecure server, if not require, turning off insecure (HTTP) port",
	)
}

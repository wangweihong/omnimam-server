package genericoptions

import (
	"fmt"
	"path"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	"github.com/wangweihong/omnimam/backend/pkg/app"

	"github.com/spf13/pflag"
)

var _ app.CompleteableOptions = &SecureServingOptions{}

// SecureServingOptions contains configuration items related to HTTPS server startup.
type SecureServingOptions struct {
	BindAddress string `json:"bind-address" mapstructure:"bind-address"`
	// BindPort is ignored when Listener is set, will serve HTTPS even with 0.
	BindPort int `json:"bind-port"    mapstructure:"bind-port"`
	// Required set to true means that BindPort cannot be zero.
	Required bool `json:"required"     mapstructure:"required"`
	// ServerCert is the TLS cert info for serving secure t`raffic
	ServerCert tls.GeneratableKeyCert `json:"tls"          mapstructure:"tls"`
}

// NewSecureServingOptions creates a SecureServingOptions object with default parameters.
func NewSecureServingOptions() *SecureServingOptions {
	return &SecureServingOptions{
		BindAddress: "0.0.0.0",
		BindPort:    8443,
		Required:    false,
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
func (s *SecureServingOptions) ApplyTo(c *httpsvr.Config) error {
	// SecureServing is required to serve https
	c.SecureServing = &httpsvr.SecureServingInfo{
		BindAddress: s.BindAddress,
		BindPort:    s.BindPort,
		CertKey:     s.ServerCert.CertData,
		Required:    s.Required,
	}

	return nil
}

// Validate is used to parse and validate the parameters entered by the user at
// the command line when the program starts.
func (s *SecureServingOptions) Validate() []error {
	if s == nil {
		return nil
	}

	errors := []error{}

	if s.Required {
		if s.BindPort < 1 || s.BindPort > 65535 {
			errors = append(
				errors,
				fmt.Errorf(
					"--secure.bind-port %v must be between 1 and 65535",
					s.BindPort,
				),
			)
		}

		if err := s.ServerCert.Validate(); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// AddFlags adds flags related to HTTPS server for a specific APIServer to the
// specified FlagSet.
func (s *SecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&s.Required, "secure.required", s.Required,
		"Whether require secure server, if not require, turning off secure (HTTPs) port",
	)
	fs.StringVar(&s.BindAddress, "secure.bind-address", s.BindAddress, ""+
		"The IP address on which to listen for the --secure.bind-port port. The "+
		"associated interface(s) must be reachable by the rest of the engine, and by CLI/web "+
		"clients. If blank, all interfaces will be used (0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")
	desc := "The port on which to serve HTTPS with authentication and authorization."

	fs.IntVar(&s.BindPort, "secure.bind-port", s.BindPort, desc)

	fs.StringVar(&s.ServerCert.CertDirectory, "secure.tls.cert-dir", s.ServerCert.CertDirectory, ""+
		"The directory where the TLS certs are located. "+
		"If --secure.tls.cert-key.cert-file and --secure.tls.cert-key.private-key-file are provided, "+
		"this flag will be ignored.")

	fs.StringVar(&s.ServerCert.PairName, "secure.tls.pair-name", s.ServerCert.PairName, ""+
		"The name which will be used with --secure.tls.cert-dir to make a cert and key filenames. "+
		"It becomes <cert-dir>/<pair-name>.crt and <cert-dir>/<pair-name>.key")

	fs.StringVar(&s.ServerCert.CertKey.CertFile, "secure.tls.cert-key.cert-file", s.ServerCert.CertKey.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")

	fs.StringVar(&s.ServerCert.CertKey.KeyFile, "secure.tls.cert-key.private-key-file",
		s.ServerCert.CertKey.KeyFile, ""+
			"File containing the default x509 private key matching --secure.tls.cert-key.cert-file.")

	fs.StringVar(&s.ServerCert.CertData.Cert, "secure.tls.cert-data", s.ServerCert.CertData.Cert, ""+
		"Data of default x509 Certificate for gRPC server.")

	fs.StringVar(&s.ServerCert.CertData.Key, "secure.tls.key-data",
		s.ServerCert.CertData.Key, ""+
			"Data of default x509 private key matching --secure.tls.cert-data.")
}

// Complete fills in any fields not set that are required to have valid data.
// Complete fills in any fields not set that are required to have valid data.
func (s *SecureServingOptions) Complete() error {
	if s == nil {
		return nil
	}

	// if not required, do nothing
	if !s.Required {
		return nil
	}

	if len(s.ServerCert.CertData.Cert) != 0 || len(s.ServerCert.CertData.Key) != 0 {
		return nil
	}

	keyCert := &s.ServerCert.CertKey
	var err error
	if len(keyCert.CertFile) != 0 || len(keyCert.KeyFile) != 0 {
		s.ServerCert.CertData.Cert, s.ServerCert.CertData.Key, err = tls.LoadDataFromFile(
			keyCert.CertFile,
			keyCert.KeyFile,
		)
		if err != nil {
			return err
		}
	}

	if len(s.ServerCert.CertDirectory) > 0 {
		if len(s.ServerCert.PairName) == 0 {
			return fmt.Errorf("pair-name is required if cert-dir is set")
		}
		keyCert.CertFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".crt")
		keyCert.KeyFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".key")
		s.ServerCert.CertData.Cert, s.ServerCert.CertData.Key, err = tls.LoadDataFromFile(
			keyCert.CertFile,
			keyCert.KeyFile,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

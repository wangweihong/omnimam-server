package grpcoptions

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	"github.com/spf13/pflag"
)

// TCPOptions are for creating an generic gRPC server.
type TCPOptions struct {
	Required    bool   `json:"required"     mapstructure:"required"`
	BindAddress string `json:"bind-address" mapstructure:"bind-address"`
	BindPort    int    `json:"bind-port"    mapstructure:"bind-port"`
	TlsEnable   bool   `json:"tls-enable"   mapstructure:"tls-enable"`
	// ServerCert is the TLS cert info for serving secure traffic
	ServerCert tls.MTLSCert `json:"tls"          mapstructure:"tls"`
}

// NewTCPOptions is for creating an generic tcp listen gRPC server.
func NewTCPOptions() *TCPOptions {
	return &TCPOptions{
		BindAddress: "0.0.0.0",
		BindPort:    8081,
		TlsEnable:   false,
		Required:    true,
	}
}

// Validate is used to parse and validate the parameters entered by the user at
// the command line when the program starts.
func (s *TCPOptions) Validate() []error {
	var errors []error

	if s.BindAddress == "" {
		errors = append(errors, fmt.Errorf("gRPC bind address `--tcp.bind-address` is empty"))
	}

	// BindPort = 0 means random port. maybe should support it ??
	if s.BindPort < 1 || s.BindPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--tcp.bind-port %v must be between 1 and 65535",
				s.BindPort,
			),
		)
	}

	if s.TlsEnable {
		if err := s.ServerCert.Validate(); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// AddFlags adds flags related to features for a specific apis server to the
// specified FlagSet.
func (s *TCPOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&s.Required, "tcp.required", s.Required,
		"Whether require tcp server, if not require, turning off tcp server")

	fs.StringVar(&s.BindAddress, "tcp.bind-address", s.BindAddress, ""+
		"The IP address on which to serve the --grpc.bind-port(set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")

	fs.IntVar(&s.BindPort, "tcp.bind-port", s.BindPort, ""+
		"The port on which gRPC server to serve.  0 for turning off insecure (HTTP) port.")

	fs.BoolVar(&s.TlsEnable, "tcp.tls-enable", s.TlsEnable,
		"Whether enabled gRPC tls verified.",
	)

	fs.StringVar(&s.ServerCert.CertDirectory, "tcp.tls.cert-dir", s.ServerCert.CertDirectory, ""+
		"The directory where the TLS certs are located. "+
		"If --tcp.tls.cert-key.cert-file and --tcp.tls.cert-key.private-key-file are provided, "+
		"this flag will be ignored.")

	fs.StringVar(&s.ServerCert.PairName, "tcp.tls.pair-name", s.ServerCert.PairName, ""+
		"The name which will be used with --tcp.tls.cert-dir to make a cert and key filenames. "+
		"It becomes <cert-dir>/<pair-name>.crt and <cert-dir>/<pair-name>.key")

	fs.StringVar(&s.ServerCert.CertKey.CertFile, "tcp.tls.cert-file", s.ServerCert.CertKey.CertFile, ""+
		"File containing the default x509 Certificate for gRPC server. (CA cert, if any, concatenated "+
		"after server cert).")

	fs.StringVar(&s.ServerCert.CertKey.KeyFile, "tcp.tls.cert-key",
		s.ServerCert.CertKey.KeyFile, ""+
			"File containing the default x509 private key matching --tcp.tls.cert-file.")

	fs.StringVar(&s.ServerCert.CertData.Cert, "tcp.tls.cert-data", s.ServerCert.CertData.Cert, ""+
		"Data of default x509 Certificate for gRPC server.")

	fs.StringVar(&s.ServerCert.CertData.Key, "tcp.tls.key-data",
		s.ServerCert.CertData.Key, ""+
			"Data of default x509 private key matching --tcp.tls.cert-data.")

	fs.StringVar(&s.ServerCert.ClientCAData, "tcp.tls.client-ca-data",
		s.ServerCert.ClientCAData, ""+
			"Data of default x509 Certificate for gRPC server validate connect client if valid.")

	fs.StringVar(&s.ServerCert.ClientCAPath, "tcp.tls.client-ca-path",
		s.ServerCert.ClientCAPath, ""+
			"File containing the  data of x509 Certificate for gRPC server validate connect client if valid.")
}

// Complete fills in any fields not set that are required to have valid data.
func (s *TCPOptions) Complete() error {
	if s == nil {
		return nil
	}

	if !s.TlsEnable {
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
			return fmt.Errorf("load cert from dir %v fail:%w", s.ServerCert.CertDirectory, err)
		}
	}

	if s.ServerCert.ClientCAData == "" && s.ServerCert.ClientCAPath != "" {
		pemClientCA, err := ioutil.ReadFile(s.ServerCert.ClientCAPath)
		if err != nil {
			return fmt.Errorf("client ca %v load fail:%w", s.ServerCert.ClientCAPath, err)
		}

		s.ServerCert.ClientCAData = string(pemClientCA)
	}

	return nil
}

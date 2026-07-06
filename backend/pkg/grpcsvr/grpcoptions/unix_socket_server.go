package grpcoptions

import "github.com/spf13/pflag"

// UnixSocketOptions are for creating an generic unix socket gRPC server.
type UnixSocketOptions struct {
	Socket string `json:"socket" mapstructure:"socket"`
}

// Validate is used to parse and validate the parameters entered by the user at
// the command line when the program starts.
func (s *UnixSocketOptions) Validate() []error {
	var errs []error
	return errs
}

// NewUnixSocketOptions is for creating an generic unix socket listen gRPC server.
func NewUnixSocketOptions() *UnixSocketOptions {
	return &UnixSocketOptions{}
}

func (s *UnixSocketOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Socket, "unix.socket", s.Socket, ""+
		"The Unix socket gRPC server listen on. If empty, it won't provide unix socket gRPC server")
}

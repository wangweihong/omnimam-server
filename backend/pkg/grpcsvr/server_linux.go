//go:build !windows
// +build !windows

package grpcsvr

import (
	"fmt"
	"net"
)

func (s *GRPCServer) buildUnixListen() (net.Listener, error) {
	listen, err := net.Listen("unix", s.UnixSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on unix://%s : %w", s.UnixSocket, err)
	}
	return listen, nil
}

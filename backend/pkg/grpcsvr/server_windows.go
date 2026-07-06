//go:build windows
// +build windows

package grpcsvr

import (
	"fmt"
	"net"
)

func (s *GRPCServer) buildUnixListen() (net.Listener, error) {
	// in windows, run net.Listen("unix"...) will case an error "A socket operation encountered a dead network."
	// This error message causes confusion for "What really happens".
	// Use an human-readable error instead.
	return nil, fmt.Errorf("unix socket unimplemented in windows system.")
}

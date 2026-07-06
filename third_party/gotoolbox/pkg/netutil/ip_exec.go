//go:build !windows
// +build !windows

package netutil

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/executil"
)

// ping 10.30.15.201 -w 5 -c 1
func Pinger(address string, timeout int) error {
	if address == "" || net.ParseIP(address) == nil {
		return fmt.Errorf("invalid ip:%v", address)
	}

	binary := "ping"
	if !IsIpv4Addr(address) {
		binary = "ping6"
	}
	_, err := executil.ExecuteTimeout(binary, []string{address, "-w", strconv.Itoa(timeout), "-c", "1"}, time.Duration(timeout+5)*time.Second)
	return err
}

func Ping(address string, timeout int) bool {
	err := Pinger(address, timeout)
	return err == nil
}

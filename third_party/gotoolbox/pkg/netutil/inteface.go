package netutil

import (
	"net"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

func GetDefaultInterface() (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			if strings.HasPrefix(iface.Name, "e") ||
				strings.HasPrefix(iface.Name, "br") {
				return &iface, nil
			}
		}
	}

	return nil, errors.New("no suitable network interface found")
}

var interfacePrefix = []string{
	"e",
	"br",
}

func GetInterfaceAndIP() (string, net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}
	for _, iface := range ifaces {
		if stringutil.HasAnyPrefix(iface.Name, interfacePrefix...) {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok {
					if p4 := ipnet.IP.To4(); len(p4) == net.IPv4len {
						return iface.Name, ipnet.IP, nil
					}
				}
			}
		}
	}
	return "", nil, errors.New("cannot find interface with ip")
}

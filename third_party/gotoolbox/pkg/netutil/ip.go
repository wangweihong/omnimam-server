package netutil

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	"github.com/go-ping/ping"
)

func IsIpv4Addr(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return true
		case ':':
			return false
		}
	}
	return false
}

func GetLocalIPs(wantIpv6 bool, condition func(net.Interface) bool) ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(ifaces))
	for _, iface := range ifaces {
		if !condition(iface) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok {
				if p4 := ipnet.IP.To4(); len(p4) == net.IPv4len {
					ips = append(ips, ipnet.IP)
				} else if len(ipnet.IP) == net.IPv6len && wantIpv6 {
					ips = append(ips, ipnet.IP)
				}
			}
		}
	}

	return ips, nil
}

func GetLocalIPsV2(wantIpv6 bool, skipCondition func(net.Interface) bool) ([]string, error) {
	ips, err := GetLocalIPs(wantIpv6, skipCondition)
	if err != nil {
		return nil, err
	}
	return sliceutil.Strings(ips), nil
}

func GetIPAddrs(wantIpv6 bool) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0)
	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "e") ||
			strings.HasPrefix(iface.Name, "br") {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok {
					if p4 := ipnet.IP.To4(); len(p4) == net.IPv4len {
						ips = append(ips, ipnet.IP.String())
					} else if len(ipnet.IP) == net.IPv6len && wantIpv6 {
						ips = append(ips, ipnet.IP.String())
					}
				}
			}
		}
	}

	return ips, nil
}

func GetIPAddr(wantIpv6 bool, ifacePrefix []string) (string, error) {
	ips, err := GetIPAddrs(wantIpv6)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", errors.New("local ips is empty")
	}

	return ips[0], nil
}

func GetIPAddrNotError(wantIpv6 bool) string {
	ips, err := GetIPAddrs(wantIpv6)
	if err != nil {
		return ""
	}
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

// Deprecated: use github.com/wangweihong/gotoolbox/pkg/urlutil.Domain() instead
func ParseAddrFromURL(rawurl string) (string, error) {

	if !strings.HasPrefix(rawurl, "http://") && !strings.HasPrefix(rawurl, "https://") {
		rawurl = "http://" + rawurl
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}

	if !strings.Contains(u.Host, ":") {
		return u.Host, nil
	}

	ip, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}

	return ip, nil
}

func GetLocalIP() (string, error) {
	ips, err := GetIPAddrs(false)
	if len(ips) == 0 || err != nil {
		return "", errors.Errorf("cannot get ip, err:%v", err)
	}

	return ips[0], nil
}

func PingV2(ip string) bool {
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		fmt.Printf("Failed to create pinger for IP %s: %v\n", ip, err)
		return false
	}
	pinger.Count = 1
	pinger.Timeout = time.Second
	err = pinger.Run()
	if err != nil {
		fmt.Printf("Failed to ping IP %s: %v\n", ip, err)
		return false
	}
	stats := pinger.Statistics()
	return stats.PacketsRecv > 0
}

func GetIPRange(startIP, endIP string) ([]string, error) {
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)
	if start == nil || end == nil {
		return nil, errors.Errorf("invalid ip address")
	}

	start = start.To4()
	end = end.To4()
	if start == nil || end == nil {
		return nil, errors.Errorf("only support ipv4")
	}

	startInt := binary.BigEndian.Uint32(start)
	endInt := binary.BigEndian.Uint32(end)

	if startInt > endInt {
		return nil, errors.Errorf("start ip must start before end ip")
	}

	count := endInt - startInt + 1

	if count > 1000000 {
		return nil, errors.Errorf("ip range too large")
	}
	ipList := make([]string, 0, count)
	for current := startInt; current <= endInt; current++ {
		ipBytes := make(net.IP, 4)
		binary.BigEndian.PutUint32(ipBytes, current)
		ipList = append(ipList, ipBytes.String())
	}

	return ipList, nil
}

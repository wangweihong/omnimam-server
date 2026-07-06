package errors

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var currentModule ModuleGetter

type ModuleGetter interface {
	PID() int
	Host() string
	Name() string
	String() string
}

func NewModuleGetter(module, host string, pid int) ModuleGetter {
	return simpleModule{
		name: module,
		host: host,
		pid:  pid,
	}
}

type simpleModule struct {
	name string
	host string
	pid  int
}

func (s simpleModule) PID() int {
	return s.pid
}

func (s simpleModule) Host() string {
	return s.host
}

func (s simpleModule) Name() string {
	return s.name
}

func (s simpleModule) String() string {
	return fmt.Sprintf("host:%s,pid:%d,module:%s", s.host, s.pid, s.name)
}

func GetModuleInfo() ServiceInfo {
	return ServiceInfo{
		Host: currentModule.Host(),
		Pid:  currentModule.PID(),
		Name: currentModule.Name(),
	}
}

func UpdateModuleInfo(getter ModuleGetter) {
	currentModule = getter
}

func ModuleString() string {
	return currentModule.String()
}

//nolint:gochecknoinits
func init() {
	moduleIP := "127.0.0.1"

	ipList, _ := getIPAddrs(false)
	for _, ip := range ipList {
		if ip != "127.0.0.1" {
			moduleIP = ip
			break
		}
	}

	currentModule = &simpleModule{
		name: filepath.Base(os.Args[0]),
		host: moduleIP,
		pid:  os.Getpid(),
	}
}

// don't use netutil for avoid import cycle
func getIPAddrs(wantIpv6 bool) ([]string, error) {
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

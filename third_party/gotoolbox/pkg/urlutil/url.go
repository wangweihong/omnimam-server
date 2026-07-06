package urlutil

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// hasScheme 检查URL是否包含协议部分
func hasScheme(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

// BuildURL 根据协议、主机和端口,路径构建URL
func BuildURL(protocol, host string, port int, paths ...string) string {
	host = TrimScheme(host)
	protocol = strings.TrimSuffix(protocol, "://")

	path := strings.Join(paths, "/")
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s://%s:%d%s", protocol, host, port, path)
}

// SplitURL 解析URL并返回协议、主机、端口
func SplitURL(rawURL string) (string, string, int, error) {
	// 如果URL字符串(如纯IP:127.0.1)没有协议部分，它会将整个字符串视为路径而不是主机
	if !hasScheme(rawURL) {
		rawURL = "http://" + rawURL
	}

	// 如果rawURL带有http协议头, 则rawURL协议头保存在u.scheme, rawURL中第一个"/"之前的部分会被当作端点保存在u.host()/u.port(),剩余部分保存在u.path
	// 如果不带协议头, 则会报保存在u.Path字段(不管是127.0.0.1还是/rest/2)
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", 0, err
	}

	// 如果端口为空，则使用默认端口
	portStr := u.Port()
	if portStr == "" {
		portStr = getDefaultPort(u.Scheme)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", "", 0, err
	}

	return u.Scheme, u.Hostname(), port, nil
}

func SplitURLV2(rawURL string) (string, string, int, string, error) {
	// 如果URL字符串(如纯IP:127.0.1)没有协议部分，它会将整个字符串视为路径而不是主机
	if !hasScheme(rawURL) {
		rawURL = "http://" + rawURL
	}

	// 如果rawURL带有http协议头, 则rawURL协议头保存在u.scheme, rawURL中第一个"/"之前的部分会被当作端点保存在u.host()/u.port(),剩余部分保存在u.path
	// 如果不带协议头, 则会报保存在u.Path字段(不管是127.0.0.1还是/rest/2)
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", 0, "", err
	}

	// 如果端口为空，则使用默认端口
	portStr := u.Port()
	if portStr == "" {
		portStr = getDefaultPort(u.Scheme)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", "", 0, "", err
	}

	return u.Scheme, u.Hostname(), port, u.Path, nil
}

func Scheme(rawURl string) string {
	scheme, _, _, _ := SplitURL(rawURl)
	return scheme
}

func Domain(rawURl string) string {
	_, domain, _, _ := SplitURL(rawURl)
	return domain
}

func Port(rawURL string) int {
	_, _, port, _ := SplitURL(rawURL)
	return port
}

func Path(rawURL string) string {
	_, _, _, path, _ := SplitURLV2(rawURL)
	return path
}

// getDefaultPort 根据协议获取默认端口
func getDefaultPort(scheme string) string {
	switch strings.ToLower(scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func ReplaceURL(originURL string, scheme *string, host *string, port *int, path *string) (string, error) {
	oscheme, ohost, oport, opath, err := SplitURLV2(originURL)
	if err != nil {
		return "", err
	}

	if scheme != nil {
		oscheme = *scheme
	}

	if host != nil {
		ohost = *host
	}

	if port != nil {
		oport = *port
	}

	if path != nil {
		opath = *path
	}

	return BuildURL(oscheme, ohost, oport, opath), nil

}

func TrimScheme(raw string) string {
	return strings.TrimPrefix(strings.TrimPrefix(raw, "http://"), "https://")
}

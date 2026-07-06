package httpcli_test

import (
	"net/http"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
)

func TestWithProxy(t *testing.T) {
	// 从环境变量中读取代理
	httpcli.WithProxy(http.ProxyFromEnvironment)
}

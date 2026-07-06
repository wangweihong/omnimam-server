package profiling_test

import (
	"net/http"
	"testing"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/profiling"
)

func TestNewProfilingServer(t *testing.T) {
	if err := profiling.StartProfilingServer(":6060"); err != nil {
		t.Log(err)
		t.Fail()
	}
	client := &http.Client{}
	resp, err := client.Get("http://127.0.0.1:6060/debug/pprof/symbol")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	if resp.StatusCode != http.StatusOK {
		t.Log("statusCode is not 200 when profiling server start")
		t.Fail()
	}

	if err := profiling.StopProfilingServer(); err != nil {
		t.Log(err)
		t.Fail()
	}

	if _, err = client.Get("http://127.0.0.1:6060/debug/pprof/symbol"); err == nil {
		t.Log("get profile success when server stop")
		t.Fail()
	}
}

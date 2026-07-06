package httpcli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/callerutil"

	"github.com/wangweihong/gotoolbox/pkg/json"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
)

func logEnabled() bool {
	debugEnv := os.Getenv("HTTPCLI_DEBUG")
	return debugEnv != "" && debugEnv != "0"
}

// 打印请求/回应参数, 以及http response body
func logHugeEnabled() bool {
	debugEnv := os.Getenv("HTTPCLI_DEBUG_HUGE")
	return debugEnv != "" && debugEnv != "0"
}

func logInfoIf(ctx context.Context, msg string) {
	if logEnabled() {
		debugLog(ctx, nil, msg)
		//log.L(ctx).F(ctx).Info(msg)
	}
}

func callEntry(start time.Time, req *HttpRequest, rawResp *HttpResponse, arg, reply any, err error) maputil.StringAny {
	fields := make(map[string]any)
	fields["req_time_begin"] = start.Format("2006-01-02 15:04:05.000000")
	fields["req_raw_url"] = req.GetPath()
	fields["req_method"] = req.GetMethod()

	end := time.Now()
	Latency := time.Since(start)
	if Latency > time.Minute {
		// Truncate in a golang < 1.8 safe way
		Latency -= Latency % time.Second
	}
	fields["req_latency_ms"] = Latency
	fields["req_time_end"] = end.Format("2006-01-02 15:04:05.000000")

	fields["req_addr"] = req.GetEndpoint()
	fields["req_url"] = req.GetFullRequestAddress()

	if rawResp != nil {
		fields["resp_status"] = rawResp.GetStatusCode()
		fields["resp_body_length"] = len(rawResp.GetBody())
		fields["req_media_type"] = rawResp.GetHeader("Content-Type")

		if logHugeEnabled() {
			fields["resp_body"] = rawResp.GetBody()
			fields["resp_headers"] = json.ToString(rawResp.GetHeaders())
			fields["caller"] = callerutil.CallersDepth(32, 4).List()
		}
	}

	if logHugeEnabled() {
		fields["req_headers"] = json.ToString(req.headerParams)
		fields["req_body"] = json.ToString(req.bodyData)
		fields["func_arg"] = json.ToString(arg)
		fields["func_reply"] = json.ToString(reply)
	}
	fields["error"] = err
	return fields
}

func debugCore(ctx context.Context, start time.Time, req *HttpRequest, rawResp *HttpResponse, arg, reply any, err error) {
	if logEnabled() {
		fields := callEntry(start, req, rawResp, arg, reply, err)
		simpleCallInfo := fmt.Sprintf(
			"%v - [%s] %v %s  %s",
			fields.Get("resp_status"),
			fields.Get("req_addr"),
			fields.Get("req_latency_ms"),
			fields.Get("req_method"),
			fields.Get("req_url"),
		)
		debugLog(ctx, fields, simpleCallInfo)
	}
}

func debugLog(ctx context.Context, fields map[string]any, msg string) {
	debugLogger.Info(ctx, fields, msg)
}

var debugLogger Logger = fmtLogger{}

type Logger interface {
	Info(context.Context, map[string]any, string)
}

func SetLogger(logger2 Logger) {
	debugLogger = logger2
}

type fmtLogger struct{}

func (fl fmtLogger) Info(ctx context.Context, fields map[string]any, msg string) {
	fmt.Println(msg)
	if fields != nil {
		json.PrintStructObject(fields)
	}
}

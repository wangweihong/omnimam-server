package httphandler

import (
	"net/http"
	"time"
)

type MonitorMetric struct {
	Host          string
	Path          string
	Method        string
	Raw           string
	UserAgent     string
	RequestId     string
	StatusCode    int
	ContentLength int64
	Latency       time.Duration
}

// FIXME: 这个功能是否和Interceptor重合?
type HttpHandler struct {
	RequestHandlers  func(*http.Request) error
	ResponseHandlers func(*http.Response) error
	MonitorHandlers  func(*MonitorMetric)
}

func NewHttpHandler() *HttpHandler {
	handler := HttpHandler{}
	return &handler
}

func (handler *HttpHandler) AddRequestHandler(requestHandler func(*http.Request) error) *HttpHandler {
	handler.RequestHandlers = requestHandler
	return handler
}

func (handler *HttpHandler) AddResponseHandler(responseHandler func(response *http.Response) error) *HttpHandler {
	handler.ResponseHandlers = responseHandler
	return handler
}

func (handler *HttpHandler) AddMonitorHandler(monitorHandler func(*MonitorMetric)) *HttpHandler {
	handler.MonitorHandlers = monitorHandler
	return handler
}

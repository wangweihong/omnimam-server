package genericmiddleware

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/mattn/go-isatty"

	"github.com/wangweihong/gotoolbox/pkg/netutil"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

const (
	MaxRequestLoggerLength  = 40960
	MaxResponseLoggerLength = 40960
)

// Request logger
// One Log Record Request Life Time
// nolint: gocognit
// TODO: 应该记录操作者身份.
func LoggerMiddleware(skippers ...skipper.SkipperFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skipper.Skip(c.Request.URL.Path, skippers...) {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		method := c.Request.Method

		start := time.Now()
		fields := make(map[string]any)
		// log会根据key的排序来依次打印，调整key的命名以达到控制输出顺序
		fields["req_time_begin"] = start.Format("2006-01-02 15:04:05.000000")
		fields["host_pid"] = os.Getpid()
		fields["host_ip"] = netutil.GetIPAddrNotError(true)
		fields["req_client_ip"] = c.ClientIP()
		fields["req_method"] = method
		fields["req_url"] = c.Request.URL.String()
		fields["req_proto"] = c.Request.Proto
		fields["header"] = c.Request.Header
		// fields["user_agent"] = c.GetHeader("User-Agent")
		fields["req_content_length"] = c.Request.ContentLength
		fields["req_media_type"] = c.GetHeader("Content-Type")

		if !DisableCopy { //nolint: nestif
			if method == http.MethodPost || method == http.MethodPut {
				mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
				if mediaType != "multipart/form-data" {
					if v, ok := c.Get(RequestBodyKey); ok {
						if b, ok := v.([]byte); ok && len(b) <= MaxRequestLoggerLength {
							fields["z_request_body"] = string(b)
						}
					}
				} else {
					if v, ok := c.Get(RequestBodyKey); ok {
						if b, ok := v.([]byte); ok && len(b) <= MaxRequestLoggerLength {
							fields["z_form_body"] = string(b)
						}
					}
				}
			}
		}
		c.Next()
		// 这里应当从回应中取出具体的状态码，错误信息。错误栈
		end := time.Now()
		Latency := time.Since(start)
		if Latency > time.Minute {
			// Truncate in a golang < 1.8 safe way
			Latency -= Latency % time.Second
		}
		fields["resp_status"] = c.Writer.Status()
		fields["resp_length"] = c.Writer.Size()
		fields["req_latency"] = Latency
		fields["req_time_end"] = end.Format("2006-01-02 15:04:05.000000")

		if v, ok := c.Get(ResponseBodyKey); ok {
			// 数据量允许时才打印
			if b, ok := v.([]byte); ok && len(b) <= MaxResponseLoggerLength {
				fields["z_resp_body"] = string(b)
			}
		}
		simpleCallInfo := fmt.Sprintf(
			"%3d - [%s] %v %s  %s",
			c.Writer.Status(),
			c.ClientIP(),
			Latency,
			c.Request.Method,
			p,
		)
		log.F(c).Info(simpleCallInfo, log.Every("call-detail", fields))
	}
}

// defaultLogFormatter is the default log format function Logger middleware uses.
var defaultLogFormatter = func(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		// Truncate in a golang < 1.8 safe way
		param.Latency = param.Latency - param.Latency%time.Second
	}

	return fmt.Sprintf("%s%3d%s - [%s] \"%v %s%s%s %s\" %s",
		// param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.ClientIP,
		param.Latency,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)
}

// Logger instances a Logger middleware that will write the logs to gin.DefaultWriter.
// By default gin.DefaultWriter = os.Stdout.
func Logger() gin.HandlerFunc {
	return LoggerWithConfig(GetLoggerConfig(nil, nil, nil))
}

// LoggerWithFormatter instance a Logger middleware with the specified log format function.
func LoggerWithFormatter(f gin.LogFormatter) gin.HandlerFunc {
	return LoggerWithConfig(gin.LoggerConfig{
		Formatter: f,
	})
}

// LoggerWithWriter instance a Logger middleware with the specified writer buffer.
// Example: os.Stdout, a file opened in write mode, a socket...
func LoggerWithWriter(out io.Writer, notlogged ...string) gin.HandlerFunc {
	return LoggerWithConfig(gin.LoggerConfig{
		Output:    out,
		SkipPaths: notlogged,
	})
}

// LoggerWithConfig instance a Logger middleware with config.
func LoggerWithConfig(conf gin.LoggerConfig) gin.HandlerFunc {
	formatter := conf.Formatter
	if formatter == nil {
		formatter = defaultLogFormatter
	}

	out := conf.Output
	if out == nil {
		out = gin.DefaultWriter
	}

	notlogged := conf.SkipPaths

	isTerm := true

	if w, ok := out.(*os.File); !ok || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd())) {
		isTerm = false
	}

	if isTerm {
		gin.ForceConsoleColor()
	}

	var skip map[string]struct{}

	if length := len(notlogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range notlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log only when path is not being skipped
		if _, ok := skip[path]; !ok {
			param := gin.LogFormatterParams{
				Request: c.Request,
				Keys:    c.Keys,
			}

			// Stop timer
			param.TimeStamp = time.Now()
			param.Latency = param.TimeStamp.Sub(start)

			param.ClientIP = c.ClientIP()
			param.Method = c.Request.Method
			param.StatusCode = c.Writer.Status()
			param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

			param.BodySize = c.Writer.Size()

			if raw != "" {
				path = path + "?" + raw
			}

			param.Path = path

			log.L(c).Info(formatter(param))
		}
	}
}

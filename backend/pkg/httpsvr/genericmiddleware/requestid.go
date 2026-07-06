package genericmiddleware

import (
	"fmt"
	"io"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/tracectx"
)

const (
	// XRequestIDKey defines X-Request-ID key string.
	XRequestIDKey = "X-Request-ID"
)

// RequestID is a middleware that injects a 'X-Request-ID' into the context and request/response header of each request.
func RequestID(skippers ...skipper.SkipperFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skipper.Skip(c.Request.URL.Path, skippers...) {
			c.Next()
			return
		}

		// Check for incoming header, use it if exists
		requestID := c.GetHeader(XRequestIDKey)

		if requestID == "" {
			requestID = tracectx.NewTraceID()
		}
		// record traceID in gin context
		c.Set(XRequestIDKey, requestID)

		// Set XRequestIDKey header
		c.Writer.Header().Set(XRequestIDKey, requestID)
		c.Next()
	}
}

// GetLoggerConfig return gin.LoggerConfig which will write the logs to specified io.Writer with given gin.LogFormatter.
// By default gin.DefaultWriter = os.Stdout
// reference: https://github.com/gin-gonic/gin#custom-log-format
func GetLoggerConfig(formatter gin.LogFormatter, output io.Writer, skipPaths []string) gin.LoggerConfig {
	if formatter == nil {
		formatter = GetDefaultLogFormatterWithRequestID()
	}

	return gin.LoggerConfig{
		Formatter: formatter,
		Output:    output,
		SkipPaths: skipPaths,
	}
}

// GetDefaultLogFormatterWithRequestID returns gin.LogFormatter with 'RequestID'.
func GetDefaultLogFormatterWithRequestID() gin.LogFormatter {
	return func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			// Truncate in a golang < 1.8 safe way
			param.Latency -= param.Latency % time.Second
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
}

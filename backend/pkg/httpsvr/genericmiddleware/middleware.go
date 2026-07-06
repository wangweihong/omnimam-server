package genericmiddleware

import (
	"net/http"
	"time"

	gindump "github.com/tpkeeper/gin-dump"

	"github.com/gin-gonic/gin"
)

const (
	MWNameContext   = "context"
	MWNameRequestID = "requestid"
	MWNameRecovery  = "recovery"
	MWNameSecure    = "secure"
	MWNameOptions   = "options"
	MWNameNoCache   = "nocache"
	MWNameCORS      = "cors"
	MWNameLogger    = "logger"
	MWNameDump      = "dump"
)

// Middlewares store registered middlewares.
var (
	MiddlewareList  = defaultMiddlewareList()
	MiddlewareNames = defaultMiddlewareListNames()
)

// NoCache is a middleware function that appends headers
// to prevent the client from caching the HTTP response.
func NoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate, value")
	c.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	c.Next()
}

// Options is a middleware function that appends headers
// for options requests and aborts then exits the middleware
// chain and ends the request.
func Options(c *gin.Context) {
	if c.Request.Method != "OPTIONS" {
		c.Next()
	} else {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "authorization, origin, content-type, accept")
		c.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Content-Type", "application/json")
		c.AbortWithStatus(http.StatusOK)
	}
}

// Secure is a middleware function that appends security
// and resource access headers.
func Secure(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-XSS-Protection", "1; mode=block")

	if c.Request.TLS != nil {
		c.Header("Strict-Transport-Security", "max-age=31536000")
	}
}

func defaultMiddlewareList() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		MWNameContext:   Context(),
		MWNameRequestID: RequestID(),
		MWNameRecovery:  gin.Recovery(),
		MWNameSecure:    Secure,
		MWNameOptions:   Options,
		MWNameNoCache:   NoCache,
		MWNameCORS:      Cors(),
		MWNameLogger:    Logger(),
		MWNameDump:      gindump.Dump(),
	}
}

func defaultMiddlewareListNames() []string {
	names := make([]string, 0, len(defaultMiddlewareList()))
	for name := range defaultMiddlewareList() {
		names = append(names, name)
	}
	return names
}

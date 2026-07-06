package genericmiddleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

// Context is a middleware that injects common prefix fields to gin.Context.
func Context() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(log.KeyRequestID), c.GetString(XRequestIDKey))
		c.Next()
	}
}

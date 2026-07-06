package middleware

// import (
// 	"strings"

// 	"github.com/gin-gonic/gin"
// )

// func ClientTypeMiddleware() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		clientType := detectClientType(c)
// 		c.Set("clientType", clientType)
// 		c.Next()
// 	}
// }

// func detectClientType(c *gin.Context) string {
// 	// 1. 检查标准OAuth2参数
// 	if c.Query("response_type") != "" || c.Query("client_id") != "" {
// 		return "oauth_client"
// 	}

// 	// 2. 检查User-Agent
// 	ua := strings.ToLower(c.Request.UserAgent())
// 	switch {
// 	case strings.Contains(ua, "mozilla"):
// 		return "web_browser"
// 	case strings.Contains(ua, "dart") || strings.Contains(ua, "flutter"):
// 		return "mobile_app"
// 	case strings.Contains(ua, "java") || strings.Contains(ua, "kotlin"):
// 		return "android_app"
// 	case strings.Contains(ua, "swift") || strings.Contains(ua, "webkit"):
// 		return "ios_app"
// 	}

// 	// 3. 检查Accept头
// 	accept := c.GetHeader("Accept")
// 	if strings.Contains(accept, "application/json") {
// 		return "api_client"
// 	}

// 	// 4. 默认处理
// 	return "web_browser"
// }

package middleware

// var jwtCodec=tokenutil.N		key, err := rsa.GenerateKey(rand.Reader, 2048)
// 		So(err, ShouldBeNil)

// 		testURL, _ := url.Parse("https://test.example.com")
// 		opts := tokenutil.Options{URL: *testURL, Key: key, MaxIssueDelay: 3 * time.Second}
// 		codec := tokenutil.DefaultJWTTrackedRequestCodec(opts)

// 中间件中的双渠道验证
// func DualAuthMiddleware() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		// 1. 优先检查Cookie（浏览器场景）
// 		if tokenStr, err := c.Cookie(iapiserver.CookieKeyToken); err == nil {
// 			if session, ok := validateSession(sessionID); ok {
// 				c.Set("user", session.User)
// 				c.Next()
// 				return
// 			}
// 		}

// 		// 2. 检查Header（API场景）
// 		// 移动端或者服务端通信采用token场景
// 		authHeader := c.GetHeader("Authorization")
// 		if authHeader == "" {
// 			authHeader = c.GetString("token")
// 		}

// 		if authHeader != "" {

// 		}

// 		if token, err := parseBearerToken(authHeader); err == nil {
// 			if claims, ok := validateJWT(token); ok {
// 				c.Set("user", claims.Subject)
// 				c.Next()
// 				return
// 			}
// 		}
// 		//}

// 		c.AbortWithStatusJSON(401, gin.H{"error": "需要认证"})
// 	}
// }

// func parseBearerToken(authHeader string) (string, error) {
// 	if !strings.HasPrefix(authHeader, "Bearer ") {
// 		return "", errors.Errorf("invalid bearer token")
// 	}
// 	return strings.TrimPrefix("Bearer "), nil
// }

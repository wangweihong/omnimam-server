package genericmiddleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type AutoRenewCookie struct {
	UserID     string    `json:"uid"`      // 用户ID
	LastActive time.Time `json:"last_act"` // 最后活跃时间
	Signature  string    `json:"sig"`      // 防篡改签名
}

// CookieRenew 实现一套基于无状态的客户端Cookie的续期机制
func CookieRenew(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// // 1. 尝试解析 Cookie
		// cookieValue, err := c.Cookie("session")
		// var session AutoRenewCookie
		// validSession := false
		// if err == nil {
		// 	// 2. 验证签名防篡改
		// 	if valid := verifySignature(cookieValue, secretKey); valid {
		// 		json.Unmarshal([]byte(cookieValue), &session)

		// 		// 3. 检查是否在有效期内
		// 		if time.Since(session.LastActive) < ttl {
		// 			validSession = true

		// 			// 4. 自动续期条件：在最后20%时间窗口内访问
		// 			if time.Since(session.LastActive) > ttl*4/5 {
		// 				session.LastActive = time.Now()
		// 				updateCookie(c, session, secretKey, ttl)
		// 			}
		// 		}
		// 	}
		// }
		// // 5. 无效或新会话处理
		// if !validSession {
		// 	session = AutoRenewCookie{
		// 		UserID:     generateUserID(),
		// 		LastActive: time.Now(),
		// 	}
		// 	updateCookie(c, session, secretKey, ttl)
		// }
		// c.Set("session", session)
		// c.Next()
	}
}

// HMAC 签名生成
func createSignature(session AutoRenewCookie, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(fmt.Sprintf("%s|%d", session.UserID, session.LastActive.UnixNano())))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

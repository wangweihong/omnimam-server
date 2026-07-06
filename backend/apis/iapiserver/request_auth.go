package iapiserver

import jwt "github.com/dgrijalva/jwt-go"

const (
	CookieKeyToken = "token"
)

type TokenInfo struct {
	UserUUID string `json:"user_uuid"`
	UserName string `json:"user_name"`
	ClientIP string `json:"client_ip"` //用来绑定远端访问ip，防止在其他地方使用该token验证
	TTL      int64  `json:"ttl"`
	jwt.StandardClaims
}

package codec

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

var (
	tokenEncodeKey = "1nudpN/99kzCgDbPYCjHBir4lJXVJmfRco2l3M4XPldh0ZOK7AcqJwk1j/hEotF4PwpDEop9bzzFSbXK8nJix+aswtCdcj2H/n9aQubUuE06/F3ggmZKcQlyC6m8N2pO1gNZsoldxgHCc55i5uAvxOiu8X2HRkXpjXkPz2hldPI="
)

func GenerateUserTokenStr(meta *iapiserver.TokenInfo) (string, error) {
	tokenClaim := jwt.NewWithClaims(jwt.SigningMethodHS256, meta)
	token, err := tokenClaim.SignedString([]byte(tokenEncodeKey))
	if err != nil {
		return "", errors.WithStack(err)
	}

	return token, nil
}

func ParseUserTokenStr(tokenStr string) (*iapiserver.TokenInfo, error) {
	parser := new(jwt.Parser)
	parser.SkipClaimsValidation = true
	token, err := parser.ParseWithClaims(tokenStr,
		&iapiserver.TokenInfo{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenEncodeKey), nil
		},
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !token.Valid {
		return nil, errors.Errorf("user token is invalid.")
	}
	claims, ok := token.Claims.(*iapiserver.TokenInfo)
	if !ok {
		return nil, errors.Errorf("claim store is not TokenInfo struct.")
	}

	return claims, nil
}

package authentication

import (
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type AuthController struct {
	srv srvv1.Service
}

func NewController(store store.Factory) *AuthController {
	return &AuthController{
		srv: srvv1.NewService(store),
	}
}

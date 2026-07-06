package identity

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type identityService struct {
	store store.Factory
}

type IdentitySrv interface {
	UserList(ctx context.Context, req *iapiserver.UserListRequest) (*iapiserver.UserListResponse, error)
	UserGet(ctx context.Context, req *iapiserver.UserGetRequest) (*iapiserver.UserGetResponse, error)
	UserAdd(ctx context.Context, req *iapiserver.UserAddRequest) (*iapiserver.UserAddResponse, error)
	UserDelete(ctx context.Context, req *iapiserver.UserDeleteRequest) error
	UserUpdate(ctx context.Context, req *iapiserver.UserUpdateRequest) error

	UserOTPGetOrAdd(ctx context.Context, req *iapiserver.OTPGenerateRequest) (string, error)
	UserOTPGet(ctx context.Context, userid string) (*iapiserver.UserOTP, error)
}

func NewService(str store.Factory) *identityService {
	return &identityService{store: str}
}

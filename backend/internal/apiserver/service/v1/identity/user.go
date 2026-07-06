package identity

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func (s *identityService) UserList(
	ctx context.Context,
	req *iapiserver.UserListRequest,
) (*iapiserver.UserListResponse, error) {
	metas, total, err := s.store.Users().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	resp := &iapiserver.UserListResponse{}
	resp.Total = total
	resp.List = metas
	return resp, nil
}

func (s *identityService) UserGet(
	ctx context.Context,
	req *iapiserver.UserGetRequest,
) (*iapiserver.UserGetResponse, error) {
	meta, err := s.store.Users().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.UserGetResponse{User: *meta}, nil
}

func (s *identityService) UserAdd(
	ctx context.Context,
	req *iapiserver.UserAddRequest,
) (*iapiserver.UserAddResponse, error) {
	meta, err := s.store.Users().Add(ctx, &req.User)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.UserAddResponse{User: *meta}, nil
}

func (s *identityService) UserUpdate(ctx context.Context, req *iapiserver.UserUpdateRequest) error {
	if _, err := s.store.Users().Update(ctx, &req.User); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *identityService) UserDelete(ctx context.Context, req *iapiserver.UserDeleteRequest) error {
	err := s.store.Users().Delete(ctx, req.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

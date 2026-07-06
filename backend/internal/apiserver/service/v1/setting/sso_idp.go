package setting

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func (s *settingService) IdentityProviderList(
	ctx context.Context,
	req *iapiserver.IdentityProviderListRequest,
) (*iapiserver.IdentityProviderListResponse, error) {
	metas, total, err := s.store.IdentityProviders().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	resp := &iapiserver.IdentityProviderListResponse{}
	resp.Total = total
	resp.List = metas
	return resp, nil
}

func (s *settingService) IdentityProviderGet(
	ctx context.Context,
	req *iapiserver.IdentityProviderGetRequest,
) (*iapiserver.IdentityProviderGetResponse, error) {
	meta, err := s.store.IdentityProviders().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.IdentityProviderGetResponse{IdentityProvider: *meta}, nil
}

func (s *settingService) IdentityProviderAdd(
	ctx context.Context,
	req *iapiserver.IdentityProviderAddRequest,
) (*iapiserver.IdentityProviderAddResponse, error) {
	meta, err := s.store.IdentityProviders().Add(ctx, &req.IdentityProvider)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.IdentityProviderAddResponse{IdentityProvider: *meta}, nil
}

func (s *settingService) IdentityProviderUpdate(
	ctx context.Context,
	req *iapiserver.IdentityProviderUpdateRequest,
) error {
	if _, err := s.store.IdentityProviders().Update(ctx, &req.IdentityProvider); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *settingService) IdentityProviderDelete(
	ctx context.Context,
	req *iapiserver.IdentityProviderDeleteRequest,
) error {
	err := s.store.IdentityProviders().Delete(ctx, req.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

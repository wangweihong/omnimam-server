package setting

import (
	"bytes"
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type settingService struct {
	store store.Factory
}

type SettingSrv interface {
	ServiceProviderSsoSamlInitiator(ctx context.Context, req *iapiserver.SpSSOInitiatorRequest) (string, string, error)
	ServiceProviderSsoSamlAcs(ctx context.Context, req *iapiserver.SpSSOInitiatorRequest) (string, error)
	ServiceProviderSsoSamlSlo(ctx context.Context) (string, error)

	ServiceProviderSsoOauth2Initiator(ctx context.Context, req *iapiserver.SpSSOInitiatorRequest) (string, error)

	SAMLProtocolSSOAuth(ctx context.Context, req *iapiserver.IdpServeSSOAnswerRequest) (*bytes.Buffer, error)

	IdentityProviderList(
		ctx context.Context,
		req *iapiserver.IdentityProviderListRequest,
	) (*iapiserver.IdentityProviderListResponse, error)
	IdentityProviderGet(
		ctx context.Context,
		req *iapiserver.IdentityProviderGetRequest,
	) (*iapiserver.IdentityProviderGetResponse, error)
	IdentityProviderAdd(
		ctx context.Context,
		req *iapiserver.IdentityProviderAddRequest,
	) (*iapiserver.IdentityProviderAddResponse, error)
	IdentityProviderDelete(ctx context.Context, req *iapiserver.IdentityProviderDeleteRequest) error
	IdentityProviderUpdate(ctx context.Context, req *iapiserver.IdentityProviderUpdateRequest) error

	ServiceProviderList(
		ctx context.Context,
		req *iapiserver.ServiceProviderListRequest,
	) (*iapiserver.ServiceProviderListResponse, error)
	ServiceProviderGet(
		ctx context.Context,
		req *iapiserver.ServiceProviderGetRequest,
	) (*iapiserver.ServiceProviderGetResponse, error)
	ServiceProviderAdd(
		ctx context.Context,
		req *iapiserver.ServiceProviderAddRequest,
	) (*iapiserver.ServiceProviderAddResponse, error)
	ServiceProviderDelete(ctx context.Context, req *iapiserver.ServiceProviderDeleteRequest) error
	ServiceProviderUpdate(ctx context.Context, req *iapiserver.ServiceProviderUpdateRequest) error
	ServiceProviderGetRedirectURL(
		ctx context.Context,
		req *iapiserver.ServiceProviderGetRequest,
	) (*iapiserver.ServiceProviderGetRedirectURLResponse, error)

	IdentityProviderSAMLMetadataUpsert(
		ctx context.Context,
		req *iapiserver.IdentityProviderMetadataUpsetRequest,
	) (*iapiserver.Setting, error)
	IdentityProviderSAMLMetadataGet(ctx context.Context) (*iapiserver.IdentityProviderMetadataGetResponse, error)
	ServiceProviderSAMLMetadataUpsert(
		ctx context.Context,
		req *iapiserver.ServiceProviderMetadataUpsetRequest,
	) (*iapiserver.Setting, error)
	ServiceProviderSAMLMetadataGet(ctx context.Context) (*iapiserver.ServiceProviderMetadataGetResponse, error)
}

func NewService(str store.Factory) *settingService {
	return &settingService{store: str}
}

package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type (
	IdentityProviderListRequest struct {
		imachinery.BasicQueryParam
		Protocol string `json:"protocol" form:"protocol"`
	}

	IdentityProviderListResponse struct {
		imachinery.ListRet
		List []*IdentityProvider `json:"list"`
	}
)

type (
	IdentityProviderAddRequest struct {
		IdentityProvider
	}

	IdentityProviderAddResponse struct {
		IdentityProvider
	}
)

type (
	IdentityProviderGetRequest struct {
		IdentityProvider
	}

	IdentityProviderGetResponse struct {
		IdentityProvider
	}
)

type (
	IdentityProviderUpdateRequest struct {
		IdentityProvider
	}

	IdentityProviderUpdateResponse struct {
		IdentityProvider
	}
)

type (
	IdentityProviderDeleteRequest struct {
		IdentityProvider
	}

	IdentityProviderDeleteResponse struct {
		IdentityProvider
	}
)

type (
	ServiceProviderListRequest struct {
		imachinery.BasicQueryParam
		Protocol string `json:"protocol" form:"protocol"`
	}

	ServiceProviderListResponse struct {
		imachinery.ListRet
		List []*ServiceProvider `json:"list"`
	}
)

type (
	ServiceProviderAddRequest struct {
		ServiceProvider
	}

	ServiceProviderAddResponse struct {
		ServiceProvider
	}
)

type (
	ServiceProviderGetRequest struct {
		ServiceProvider
	}

	ServiceProviderGetResponse struct {
		ServiceProvider
	}
)

type (
	//ServiceProviderGetRedirectURLRequest ServiceProviderGetRequest

	ServiceProviderGetRedirectURLResponse struct {
		Method string            `json:"method"`
		URL    string            `json:"url"`
		Host   string            `json:"host"`
		URI    string            `json:"uri"`
		Header map[string]string `json:"header"`
		Param  map[string]any    `json:"param"`
	}
)

type (
	ServiceProviderUpdateRequest struct {
		ServiceProvider
	}

	ServiceProviderUpdateResponse struct {
		ServiceProvider
	}
)

type (
	ServiceProviderDeleteRequest struct {
		ServiceProvider
	}

	ServiceProviderDeleteResponse struct {
		ServiceProvider
	}
)

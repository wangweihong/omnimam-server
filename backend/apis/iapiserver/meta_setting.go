package iapiserver

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const DefaultCertificateExpireDuration = time.Hour * 24 * 365 * 10

const (
	SettingKindSSOSamlIdpMetadata = "saml_idp"
	SettingKindSSOSamlSpMetadata  = "saml_sp"
)

const (
	FrontEndSsoIdentityProviderListJSPath = "/etc/nginx/dist/sso_idp.js"
)

type Setting struct {
	imachinery.ObjectMeta
}

func (Setting) TableName() string {
	return "settings"
}

// 存储idp SAML元数据
type (
	IdentityProviderMetadataExtender Setting
)

func (m IdentityProviderMetadataExtender) GetKey() string {
	return maputil.TypedGet[string, string](m.Extend, "key")
}

func (m IdentityProviderMetadataExtender) GetCert() string {
	return maputil.TypedGet[string, string](m.Extend, "cert")
}

func (m IdentityProviderMetadataExtender) GetEndpoint() string {
	return maputil.TypedGet[string, string](m.Extend, "endpoint")
}

func (m IdentityProviderMetadataExtender) GetRedirectSSOFrontendURL() string {
	return maputil.TypedGet[string, string](m.Extend, "redirect_sso_frontend_url")
}

// 存储sp SAML元数据
type (
	ServiceProviderMetadataExtender Setting
)

func (m ServiceProviderMetadataExtender) GetKey() string {
	return maputil.TypedGet[string, string](m.Extend, "key")
}

func (m ServiceProviderMetadataExtender) GetCert() string {
	return maputil.TypedGet[string, string](m.Extend, "cert")
}

func (m ServiceProviderMetadataExtender) GetEndpoint() string {
	return maputil.TypedGet[string, string](m.Extend, "endpoint")
}

func (m ServiceProviderMetadataExtender) GetAuthNameIdFormat() string {
	return maputil.TypedGet[string, string](m.Extend, "authn_name_id_format")
}

// 存储oauth2 idp config元数据
type (
	IdentityProviderOauth2ConfigExtender Setting
)

func (m IdentityProviderOauth2ConfigExtender) GetKey() string {
	return maputil.TypedGet[string, string](m.Extend, "key")
}

func (m IdentityProviderOauth2ConfigExtender) GetCert() string {
	return maputil.TypedGet[string, string](m.Extend, "cert")
}

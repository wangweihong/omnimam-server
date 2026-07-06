package iapiserver

import (
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"

	"github.com/wangweihong/omnimam/backend/pkg/httpform"
)

type (
	IdentityProviderMetadataUpsetRequest struct {
		// idp的访问端点
		Endpoint          string `json:"endpoint"                  form:"endpoint"                  binding:"required"`
		AuthnNameIDFormat string `json:"authn_name_id_format"      form:"authn_name_id_format"`
		// idp的前端登录路径(前后端分离). 如果发起idp sso操作时，idp没有登录成功，应该重定向前端哪个url
		RedirectSSOFrontendURL string `json:"redirect_sso_frontend_url" form:"redirect_sso_frontend_url" binding:"required"`
		KeyEncode              []byte `json:"-"`
		CertEncode             []byte `json:"-"`
	}

	IdentityProviderMetadataUpsetResponse struct {
	}
)

func (r *IdentityProviderMetadataUpsetRequest) Decode(c *gin.Context) error {
	if err := c.ShouldBind(r); err != nil {
		return errors.WithStack(err)
	}

	_, keybuf, err := httpform.FormUploadFileKey(c, httpform.FileLimitSize, httpform.KeyFileFormKey)
	if err != nil {
		return errors.WithStack(err)
	}

	_, certbuf, err := httpform.FormUploadFileKey(c, httpform.FileLimitSize, httpform.CertFileFormKey)
	if err != nil {
		return errors.WithStack(err)
	}

	r.KeyEncode = keybuf.Bytes()
	r.CertEncode = certbuf.Bytes()
	return nil
}

func (r *IdentityProviderMetadataUpsetRequest) Validate() error {
	if !stringutil.HasAnyPrefix(r.Endpoint, "http://", "https://") {
		return errors.Errorf("invalid endpoint, must contain scheme")
	}
	return nil
}

type (
	IdentityProviderMetadataGetResponse struct {
		Setting    *Setting `json:"setting"`
		DecodeKey  string   `json:"decode_key"`
		DecodeCert string   `json:"decode_cert"`
		XML        string   `json:"xml"`
	}
)

type (
	ServiceProviderMetadataUpsetRequest struct {
		Endpoint          string `json:"endpoint"             form:"endpoint"             binding:"required"`
		AuthnNameIDFormat string `json:"authn_name_id_format" form:"authn_name_id_format"`
		KeyEncode         []byte `json:"-"`
		CertEncode        []byte `json:"-"`
	}
)

func (r *ServiceProviderMetadataUpsetRequest) Decode(c *gin.Context) error {
	if err := c.ShouldBind(r); err != nil {
		return errors.WithStack(err)
	}

	_, keybuf, err := httpform.FormUploadFileKey(c, httpform.FileLimitSize, httpform.KeyFileFormKey)
	if err != nil {
		return errors.WithStack(err)
	}

	_, certbuf, err := httpform.FormUploadFileKey(c, httpform.FileLimitSize, httpform.CertFileFormKey)
	if err != nil {
		return errors.WithStack(err)
	}

	r.KeyEncode = keybuf.Bytes()
	r.CertEncode = certbuf.Bytes()
	return nil
}

func (r *ServiceProviderMetadataUpsetRequest) Validate() error {
	if !stringutil.HasAnyPrefix(r.Endpoint, "http://", "https://") {
		return errors.Errorf("invalid endpoint, must contain scheme")
	}
	return nil
}

type (
	ServiceProviderMetadataGetResponse struct {
		Setting    *Setting `json:"setting"`
		DecodeKey  string   `json:"decode_key"`
		DecodeCert string   `json:"decode_cert"`
		XML        string   `json:"xml"`
	}
)

package iapiserver

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/flate"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type IdpServeSSOAnswerRequest struct {
	SAMLRequest        string        `json:"SAMLRequest" form:"SAMLRequest" binding:"required"`
	RelayState         string        `json:"RelayState"  form:"RelayState"  binding:"required"`
	RemoteAddr         string        `json:"-"           form:"-"`
	DecodedSAMLRequest []byte        `json:"-"           form:"-"`
	Session            *saml.Session `json:"-"           form:"-"`
	Req                *http.Request `json:"-"           form:"-"`
}

/*
SAML协议规范:
  - GET请求规范 (HTTP-Redirect Binding)
    强制压缩：SAML标准要求GET请求必须使用Deflate压缩算法
    攻击面：恶意攻击者可构造高度压缩的畸形数据（如1MB数据压缩后仅1KB，解压后膨胀到1GB），导致内存耗尽
  - POST请求规范 (HTTP-POST Binding)
    禁止压缩：SAML标准明确规定POST请求直接传输Base64编码的XML文本
    无压缩层：攻击者无法通过压缩制造放大攻击
*/
func (r *IdpServeSSOAnswerRequest) Decode(c *gin.Context) error {
	switch c.Request.Method {
	case http.MethodPost:
		// 注意读取的tag为form
		var err error
		if err = c.ShouldBind(r); err != nil {
			return errors.WithStack(err)
		}
		if r.DecodedSAMLRequest, err = base64.StdEncoding.DecodeString(r.SAMLRequest); err != nil {
			return errors.Errorf("cannot decompress request: %s", err)
		}

	case http.MethodGet:
		if err := c.ShouldBindQuery(r); err != nil {
			return errors.WrapStatus(err, code.ErrValidation)
		}
		compressedRequest, err := base64.StdEncoding.DecodeString(r.SAMLRequest)
		if err != nil {
			return errors.Errorf("cannot decode SAMLRequest  %s", err)
		}
		r.DecodedSAMLRequest, err = io.ReadAll(flate.NewSaferFlateReader(bytes.NewReader(compressedRequest)))
		if err != nil {
			return errors.Errorf("cannot decompress request: %s", err)
		}
	}

	r.RemoteAddr = c.Request.RemoteAddr
	return nil
}

type (
	IdpServeOauth2SSOAuthorizeRequest struct {
		ClientID            string `json:"client_id"             form:"client_id"             binding:"required"`
		ResponseType        string `json:"response_type"         form:"response_type"         binding:"required"`
		RedirectURI         string `json:"redirect_uri"          form:"redirect_uri"          binding:"required"`
		Scope               string `json:"scope"                 form:"scope"                 binding:"required"`
		State               string `json:"state"                 form:"state"                 binding:"required"`
		CodeChallenge       string `json:"code_challenge"        form:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method" form:"code_challenge_method"`
	}
)

type (
	IdpServeOauth2SSOTokenRequest struct {
		ClientID     string `json:"client_id"     form:"client_id"     binding:"required"`
		ClientSecret string `json:"client_secret" form:"client_secret" binding:"required"`
		GrantType    string `json:"grant_type"    form:"grant_type"`
		RedirectURI  string `json:"redirect_uri"  form:"redirect_uri"  binding:"required"`
		CodeVerifier string `json:"code_verifier" form:"code_verifier"` //PKCE
	}
)

type (
	SpSSOInitiatorRequest struct {
		IdentityProviderName string `json:"identity_provider_name" form:"identity_provider_name" binding:"required"`
		//	Protocol             string `json:"protocol" binding:"required"`
		// idp登陆成功后的重定向前端URL
		RedirectURLEncode string        `json:"redirect_url_encode"    form:"redirect_url_encode"    binding:"required"`
		RedirectURL       string        `json:"-"`
		Request           *http.Request `json:"-"`
	}

	SpSSOInitiatorResponse struct {
		// initiator成功, 前端重定向到idp url进行验证
		IdpAnswerURL string `json:"idp_answer_url"`
	}

	SpSSOInfo struct {
		Name string
		SpSSOInitiatorRequest
	}
)

func (r *SpSSOInitiatorRequest) PostBind() error {
	bd, err := base64.StdEncoding.DecodeString(r.RedirectURLEncode)
	r.RedirectURL = string(bd)

	// FIXME: 需要保证重定向和发起为同一个IP
	return err
}

type (
	SpSLoRequest struct {
	}
)

type (
	SpSsoOauth2CallbackRequest struct {
		// idp授权码
		Code  string `json:"code"  form:"code"  binding:"required"`
		State string `json:"state" form:"state" binding:"required"`
	}
)

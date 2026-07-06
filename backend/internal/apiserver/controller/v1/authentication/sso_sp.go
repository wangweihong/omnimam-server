package authentication

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// SpSsoSamlInitiator  作为sp发起对idp的saml单点登录
func (rc *AuthController) SpSsoSamlInitiator(c *gin.Context) {
	r := &iapiserver.SpSSOInitiatorRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	binding, data, err := rc.srv.Settings().ServiceProviderSsoSamlInitiator(c, r)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	if binding == saml.HTTPRedirectBinding {
		// 返回URL, 由前端重定向到idp进行单点登录
		// 这里这么设计的原因是：如果由后端重定向到idp, 如果idp需要登录，idp会重定向到idp的登录页面。
		// 由于通过后端的请求的方式，发起都是在后端进行，前端浏览器页面并没有感知。因此无法正确重定向到idp的登录页。(通过F12可以看到后端重定向, 但前端页面没有切换)
		// 这导致用户没法在浏览器输入idp的账号密码。
		core.WriteResponse(c, nil, &iapiserver.SpSSOInitiatorResponse{IdpAnswerURL: data})
	} else {
		c.Header("Content-Security-Policy", ""+
			"default-src; "+
			"script-src 'sha256-AjPdJSbZmeWHnEc5ykvJFay8FTWeTeRbs9dutfZ0HqE='; "+
			"reflected-xss block; referrer no-referrer;")

		c.Data(http.StatusOK, "text/html", []byte(data))
	}
}

func onError(c *gin.Context, err error) {
	if parseErr, ok := err.(*saml.InvalidResponseError); ok {
		log.Errorf("WARNING: received invalid saml response: %s (now: %s) %s",
			parseErr.Response, parseErr.Now, parseErr.PrivateErr)
	} else {
		log.Errorf("ERROR: %s", err)
	}
	c.JSON(http.StatusForbidden, []byte(err.Error()))
}

// SpSsoSamlAcs  对idp的saml回应进行断言
func (rc *AuthController) SpSsoSamlAcs(c *gin.Context) {
	relayStateRaw := c.GetString("RelayState")
	d, err := base64.StdEncoding.DecodeString(relayStateRaw)
	if err != nil {
		onError(c, err)
		return
	}
	relayState := string(d)
	r := &iapiserver.SpSSOInitiatorRequest{}
	if err := json.Unmarshal([]byte(relayState), r); err != nil {
		onError(c, err)
		return
	}
	b, err := base64.StdEncoding.DecodeString(r.RedirectURLEncode)
	if err != nil {
		onError(c, err)
		return
	}
	r.RedirectURL = string(b)

	log.Infof("idp:%v,redirect_url:%v", r.IdentityProviderName, r.RedirectURL)
	authRedirectURL, err := rc.srv.Settings().ServiceProviderSsoSamlAcs(c, r)
	if err != nil {
		onError(c, err)
		return
	}
	c.Header(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
	)

	c.Redirect(http.StatusFound, authRedirectURL)
}

// SpSsoSamlSLo 发起单点登出
func (rc *AuthController) SpSsoSamlSLO(c *gin.Context) {
	sloRedirectURL, err := rc.srv.Settings().ServiceProviderSsoSamlSlo(c)
	if err != nil {
		onError(c, err)
		return
	}
	c.Redirect(http.StatusFound, sloRedirectURL)
}

// SpSsoOauth2Initiator  作为sp发起对idp的saml单点登录
func (rc *AuthController) SpSsoOauth2Initiator(c *gin.Context) {
	r := &iapiserver.SpSSOInitiatorRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	// 前端应用：https://frontend.com
	// 后端服务：https://backend.com
	// Google IdP配置：
	// Client ID: GOOGLE_CLIENT_ID
	// 授权端点：https://accounts.google.com/o/oauth2/v2/auth

	// 这一步骤获取的格式应为"https://accounts.google.com/o/oauth2/v2/auth?client_id=GOOGLE_CLIENT_ID&redirect_uri=https://frontend.com/auth/callback&response_type=code&scope=openid%20email%20profile&state=8x4A2dF9kL3zPqW7"
	data, err := rc.srv.Settings().ServiceProviderSsoOauth2Initiator(c, r)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	// 返回URL, 由前端重定向到idp进行单点登录
	// 这里这么设计的原因是：如果由后端重定向到idp, 如果idp需要登录，idp会重定向到idp的登录页面。
	// 由于通过后端的请求的方式，发起都是在后端进行，前端浏览器页面并没有感知。因此无法正确重定向到idp的登录页。(通过F12可以看到后端重定向, 但前端页面没有切换)
	// 这导致用户没法在浏览器输入idp的账号密码。
	core.WriteResponse(c, nil, &iapiserver.SpSSOInitiatorResponse{IdpAnswerURL: data})
}

// SpSsoOauth2Callback  idp授权通过后,重定向回前端,前端提取授权码和State传递给后端,并进行令牌交换
func (rc *AuthController) SpSsoOauth2Callback(c *gin.Context) {
	r := &iapiserver.SpSsoOauth2CallbackRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	// relayStateRaw := c.GetString("RelayState")
	// d, err := base64.StdEncoding.DecodeString(relayStateRaw)
	// if err != nil {
	// 	onError(c, err)
	// 	return
	// }
	// relayState := string(d)
	// r := &iapiserver.SpSSOInitiatorRequest{}
	// if err := json.Unmarshal([]byte(relayState), r); err != nil {
	// 	onError(c, err)
	// 	return
	// }
	// b, err := base64.StdEncoding.DecodeString(r.RedirectURLEncode)
	// if err != nil {
	// 	onError(c, err)
	// 	return
	// }
	// r.RedirectURL = string(b)

	// log.Infof("idp:%v,redirect_url:%v", r.IdentityProviderName, r.RedirectURL)
	// authRedirectURL, err := rc.srv.Settings().ServiceProviderSsoSamlAcs(c, r)
	// if err != nil {
	// 	onError(c, err)
	// 	return
	// }
	// c.Header("Accept",
	// "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")

	// c.Redirect(http.StatusFound, authRedirectURL)
}

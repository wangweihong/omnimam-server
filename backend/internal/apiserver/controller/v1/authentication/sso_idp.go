package authentication

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/randutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/codec"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// IdpServeSAMLProtocolSSO  处理从sp接收到的sso请求
func (rc *AuthController) IdpServeSAMLProtocolSSO(c *gin.Context) {
	r := &iapiserver.IdpServeSSOAnswerRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	idpMetaRet, err := rc.srv.Settings().IdentityProviderSAMLMetadataGet(c)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	idpMetaExtender := iapiserver.IdentityProviderMetadataExtender(*idpMetaRet.Setting)

	token, _ := c.Cookie(iapiserver.CookieKeyToken)
	if token == "" {
		// 如果idp服务在当前浏览器未登录，则直接重定向到idp元数据记录的前端登录页面URL
		redirectURL := idpMetaExtender.GetRedirectSSOFrontendURL() + "?" + c.Request.URL.RawQuery
		log.Infof("cookie is empty, current not login, redirect to %v", redirectURL)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}
	// 如果服务在当前已登录，则查询登录的用户信息, 返回给sp
	tokenInfo, err := codec.ParseUserTokenStr(token)
	if err != nil {
		redirectURL := idpMetaExtender.GetRedirectSSOFrontendURL() + "?" + c.Request.URL.RawQuery
		log.Infof("cookie token found, but not valid,redirect to %v", redirectURL)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	user, err := rc.srv.Identities().UserGet(c, &iapiserver.UserGetRequest{
		User: iapiserver.User{
			ObjectMeta: imachinery.ObjectMeta{
				ID: tokenInfo.UserUUID,
			},
		},
	},
	)
	if err != nil {
		redirectURL := idpMetaExtender.GetRedirectSSOFrontendURL() + "?" + c.Request.URL.RawQuery
		log.Infof("cannot get user %v:%v,redirect to %v", tokenInfo.UserUUID, err, redirectURL)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}
	session := &saml.Session{
		ID:                    base64.StdEncoding.EncodeToString(randutil.RandBytes(32)),
		NameID:                user.Name,
		Index:                 hex.EncodeToString(randutil.RandBytes(32)),
		UserName:              user.Name,
		UserEmail:             user.Mail,
		UserCommonName:        user.Name,
		UserSurname:           user.Name,
		UserGivenName:         user.Name,
		UserScopedAffiliation: "",
	}

	r.Session = session
	r.Req = c.Request
	output, err := rc.srv.Settings().SAMLProtocolSSOAuth(c, r)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// 返回一个html到浏览器, html的代码会自动执行触发浏览器跳转到sp页面并请求sp的sso acs路由
	c.Data(http.StatusOK, "text/html", output.Bytes())
}

package setting

import (
	"bytes"
	"context"
	"crypto/rsa"
	gx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/wangweihong/gotoolbox/pkg/certificate/x509"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/randutil"
	"github.com/wangweihong/gotoolbox/pkg/tokenutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"

	dsig "github.com/russellhaering/goxmldsig"
)

// SP发起SSO请求(SAML或者Oauth2)
func (s *settingService) ServiceProviderSsoSamlInitiator(
	ctx context.Context,
	req *iapiserver.SpSSOInitiatorRequest,
) (string, string, error) {
	idp, err := s.store.IdentityProviders().GetByName(ctx, req.IdentityProviderName)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	if !idp.Enable {
		return "", "", errors.Errorf("identity provider %v disable", idp.Name)
	}

	if idp.Protocol != iapiserver.SSOProtocolSAML {
		return "", "", errors.Errorf("identity provider not %v protocol", iapiserver.SSOProtocolSAML)
	}
	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlSpMetadata)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	spExtender := iapiserver.ServiceProviderMetadataExtender(*meta)
	sp, err := NewServiceProvider(&spExtender, idp)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	binding := saml.HTTPRedirectBinding
	bindingLocation := sp.GetSSOBindingLocation(binding)
	if bindingLocation == "" {
		binding = saml.HTTPPostBinding
		bindingLocation = sp.GetSSOBindingLocation(binding)
	}
	authReq, err := sp.MakeAuthenticationRequest(bindingLocation, binding, saml.HTTPPostBinding)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	b, err := json.Marshal(req)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	relayState := base64.StdEncoding.EncodeToString(b)
	if binding == saml.HTTPRedirectBinding {
		redirectURL, err := authReq.Redirect(relayState, sp)
		if err != nil {
			return "", "", errors.WithStack(err)
		}
		// 前后端分离架构，后端重定向到idp，前端是无感知的,浏览器无法切换切换到idp的登录页面。因此这里直接返回给前端，由前端进行重定向
		//c.Redirect(redirectURL.String(), http.StatusFound)
		log.Infof("sso redirect url:%v", redirectURL.String())
		return binding, redirectURL.String(), nil
	}
	if binding == saml.HTTPPostBinding {
		// w.Header().Add("Content-Security-Policy", ""+
		// 	"default-src; "+
		// 	"script-src 'sha256-AjPdJSbZmeWHnEc5ykvJFay8FTWeTeRbs9dutfZ0HqE='; "+
		// 	"reflected-xss block; referrer no-referrer;")
		// w.Header().Add("Content-type", "text/html")
		var buf bytes.Buffer
		buf.WriteString(`<!DOCTYPE html><html><body>`)
		buf.Write(authReq.Post(relayState))
		buf.WriteString(`</body></html>`)
		return binding, buf.String(), nil
	}

	return "", "", errors.Errorf("not support binding:%v", binding)
}

// FIXME: use config replace
var key = func() *rsa.PrivateKey {
	b, _ := pem.Decode([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9
yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ
4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPu
fOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5t
InWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2
EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABAoIBAFWZwDTeESBdrLcT
zHZe++cJLxE4AObn2LrWANEv5AeySYsyzjRBYObIN9IzrgTb8uJ900N/zVr5VkxH
xUa5PKbOcowd2NMfBTw5EEnaNbILLm+coHdanrNzVu59I9TFpAFoPavrNt/e2hNo
NMGPSdOkFi81LLl4xoadz/WR6O/7N2famM+0u7C2uBe+TrVwHyuqboYoidJDhO8M
w4WlY9QgAUhkPyzZqrl+VfF1aDTGVf4LJgaVevfFCas8Ws6DQX5q4QdIoV6/0vXi
B1M+aTnWjHuiIzjBMWhcYW2+I5zfwNWRXaxdlrYXRukGSdnyO+DH/FhHePJgmlkj
NInADDkCgYEA6MEQFOFSCc/ELXYWgStsrtIlJUcsLdLBsy1ocyQa2lkVUw58TouW
RciE6TjW9rp31pfQUnO2l6zOUC6LT9Jvlb9PSsyW+rvjtKB5PjJI6W0hjX41wEO6
fshFELMJd9W+Ezao2AsP2hZJ8McCF8no9e00+G4xTAyxHsNI2AFTCQcCgYEA5cWZ
JwNb4t7YeEajPt9xuYNUOQpjvQn1aGOV7KcwTx5ELP/Hzi723BxHs7GSdrLkkDmi
Gpb+mfL4wxCt0fK0i8GFQsRn5eusyq9hLqP/bmjpHoXe/1uajFbE1fZQR+2LX05N
3ATlKaH2hdfCJedFa4wf43+cl6Yhp6ZA0Yet1r8CgYEAwiu1j8W9G+RRA5/8/DtO
yrUTOfsbFws4fpLGDTA0mq0whf6Soy/96C90+d9qLaC3srUpnG9eB0CpSOjbXXbv
kdxseLkexwOR3bD2FHX8r4dUM2bzznZyEaxfOaQypN8SV5ME3l60Fbr8ajqLO288
wlTmGM5Mn+YCqOg/T7wjGmcCgYBpzNfdl/VafOROVbBbhgXWtzsz3K3aYNiIjbp+
MunStIwN8GUvcn6nEbqOaoiXcX4/TtpuxfJMLw4OvAJdtxUdeSmEee2heCijV6g3
ErrOOy6EqH3rNWHvlxChuP50cFQJuYOueO6QggyCyruSOnDDuc0BM0SGq6+5g5s7
H++S/wKBgQDIkqBtFr9UEf8d6JpkxS0RXDlhSMjkXmkQeKGFzdoJcYVFIwq8jTNB
nJrVIGs3GcBkqGic+i7rTO1YPkquv4dUuiIn+vKZVoO6b54f+oPBXd4S0BnuEqFE
rdKNuCZhiaE2XD9L/O9KP1fh5bfEcKwazQ23EvpJHBMm8BGC+/YZNw==
-----END RSA PRIVATE KEY-----`))
	k, _ := gx509.ParsePKCS1PrivateKey(b.Bytes)
	return k
}()

func (s *settingService) ServiceProviderSsoSamlAcs(
	ctx context.Context,
	req *iapiserver.SpSSOInitiatorRequest,
) (string, error) {
	idp, err := s.store.IdentityProviders().GetByName(ctx, req.IdentityProviderName)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if !idp.Enable {
		return "", errors.Errorf("identity provider %v disable", idp.Name)
	}

	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlSpMetadata)
	if err != nil {
		return "", errors.WithStack(err)
	}
	spExtender := iapiserver.ServiceProviderMetadataExtender(*meta)

	sp, err := NewServiceProvider(&spExtender, idp)
	if err != nil {
		return "", errors.WithStack(err)
	}
	sp.AllowIDPInitiated = true

	possibleRequestIDs := []string{}
	//  由IDP主动登录发起的SAML
	//  这种情况下并不存在cookie(请求非从SP发起)
	if sp.AllowIDPInitiated {
		possibleRequestIDs = append(possibleRequestIDs, "")
	}

	// 提取所有合法(未篡改过)的cookie中的请求ID
	assertion, err := sp.ParseResponse(req.Request, possibleRequestIDs)
	if err != nil {
		return "", errors.WithStack(err)
	}
	// 断言成功,获取到用户在idp的名称
	name := assertion.Subject.NameID.Value
	d := iapiserver.SpSSOInfo{
		Name:                  name,
		SpSSOInitiatorRequest: *req,
	}

	signedData, err := tokenutil.NewRSAJWTCodec(key, 0).Encode(tokenutil.TrackedRequest{
		Index: randutil.RandString(nil, 256),
		Value: d,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	ott := &iapiserver.OneTimeToken{}
	ott.Type = iapiserver.OneTimeTokenTypeSAML
	ott.Name = name
	ott.Payload = signedData

	if _, err := s.store.OneTimeTokens().Add(ctx, ott); err != nil {
		return "", errors.WithStack(err)
	}
	// 这里是和前端约定, 发起到特定页面路由的重定向操作
	// 当重定向到该页面后，前端会发起sp后端的'sso'类型的登录请求。sp后端从ott中获取到签名的数据比较请求签名的数据
	// 从而验证登录成功。
	redirectURL := req.RedirectURL
	redirectURL += "?sign=" + signedData
	log.Infof("redirectURL:%v", redirectURL)
	// r.Header.Set("Accept",
	// "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	// http.Redirect(w, r, redirectURL, http.StatusFound)
	return redirectURL, nil
}

func (s *settingService) ServiceProviderSsoSamlSlo(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if user.Type != iapiserver.UserTypeSSO {
		return "", errors.Errorf("current not sso user")
	}

	idp, err := s.store.IdentityProviders().GetByName(ctx, user.Source)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if !idp.Enable {
		return "", errors.Errorf("identity provider %v disable", idp.Name)
	}

	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlSpMetadata)
	if err != nil {
		return "", errors.WithStack(err)
	}
	spExtender := iapiserver.ServiceProviderMetadataExtender(*meta)
	sp, err := NewServiceProvider(&spExtender, idp)
	if err != nil {
		return "", errors.WithStack(err)
	}
	sloUrl := sp.GetSLOBindingLocation(saml.HTTPRedirectBinding)
	if sloUrl == "" {
		return "", errors.Errorf("idp %v not support slo", meta.Name)
	}
	req, err := sp.MakeLogoutRequest(sloUrl, user.Name)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return req.Redirect("").String(), nil
}

func NewServiceProvider(
	spMeta *iapiserver.ServiceProviderMetadataExtender,
	idp *iapiserver.IdentityProvider,
) (*saml.ServiceProvider, error) {
	rootURL, err := url.Parse(spMeta.GetEndpoint())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	keyPair, leaf, err := x509.ParseCert(spMeta.GetCert(), spMeta.GetKey())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	//d, err := base64.StdEncoding.DecodeString(ssoApp.SAMLIdP.Metadata)
	//if err != nil {
	//	return nil, status.UpdateStatus(err)
	//}

	idP, err := samlsp.ParseMetadata([]byte(idp.SAML.Metadata))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sp := ServiceProvider(samlsp.Options{
		EntityID:    spMeta.GetEndpoint(),
		URL:         *rootURL,
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: leaf,
		SignRequest: true,
		IDPMetadata: idP,
	})
	return &sp, nil
}

func ServiceProvider(opts samlsp.Options) saml.ServiceProvider {
	metadataURL := opts.URL.ResolveReference(&url.URL{Path: iapiserver.SsoSpURLSamlMetadata})
	acsURL := opts.URL.ResolveReference(&url.URL{Path: iapiserver.SsoSpURLSamlACS})
	sloURL := opts.URL.ResolveReference(&url.URL{Path: iapiserver.SsoSpURLSamlSLO})

	var forceAuthn *bool
	if opts.ForceAuthn {
		forceAuthn = &opts.ForceAuthn
	}
	signatureMethod := dsig.RSASHA1SignatureMethod
	if !opts.SignRequest {
		signatureMethod = ""
	}

	if opts.DefaultRedirectURI == "" {
		opts.DefaultRedirectURI = "/"
	}

	if len(opts.LogoutBindings) == 0 {
		opts.LogoutBindings = []string{saml.HTTPPostBinding}
	}

	return saml.ServiceProvider{
		EntityID:              opts.EntityID,
		Key:                   opts.Key,
		Certificate:           opts.Certificate,
		HTTPClient:            opts.HTTPClient,
		Intermediates:         opts.Intermediates,
		MetadataURL:           *metadataURL,
		AcsURL:                *acsURL,
		SloURL:                *sloURL,
		IDPMetadata:           opts.IDPMetadata,
		ForceAuthn:            forceAuthn,
		RequestedAuthnContext: opts.RequestedAuthnContext,
		SignatureMethod:       signatureMethod,
		AllowIDPInitiated:     opts.AllowIDPInitiated,
		DefaultRedirectURI:    opts.DefaultRedirectURI,
		LogoutBindings:        opts.LogoutBindings,
	}
}

// 生成sp支持的idp列表, 前端能够直接在登录页面显示可单点登录的应用
func generateIdentityProviderListJsFile(ctx context.Context, st store.Factory) error {
	path := iapiserver.FrontEndSsoIdentityProviderListJSPath
	idps, _, err := st.IdentityProviders().List(ctx, &iapiserver.IdentityProviderListRequest{})
	if err != nil {
		return errors.WithStack(err)
	}
	data := "window.ssoidps =　" + json.ToString(idps)
	if err := ioutil.WriteFile(path, []byte(data), 0755); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

package setting

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/randutil"
	"github.com/wangweihong/gotoolbox/pkg/tokenutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// SP发起SSO请求(Oauth2)
func (s *settingService) ServiceProviderSsoOauth2Initiator(
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

	if idp.Protocol != iapiserver.SSOProtocolOAUTH2 {
		return "", errors.Errorf("identity provider not %v protocol", iapiserver.SSOProtocolOAUTH2)
	}

	// authURL, err := url.Parse(idp.AuthorizationURL)
	// if err != nil {
	// 	h.sendError(w, "server_error", "Invalid authorization URL")
	// 	return
	// }

	// TODO: 支持PKCE验证
	// 生成安全的随机state值
	randStr := randutil.RandString(nil, 256)
	state, err := tokenutil.NewRSAJWTCodec(key, 0).Encode(tokenutil.TrackedRequest{
		Index: randStr,
		//Value: d,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	ott := &iapiserver.OneTimeToken{}
	ott.Type = iapiserver.OneTimeTokenTypeOauth2
	ott.Name = req.IdentityProviderName + "-" + randStr[:32]
	ott.Payload = state

	if _, err := s.store.OneTimeTokens().Add(ctx, ott); err != nil {
		return "", errors.WithStack(err)
	}
	redirectURL := idp.Oauth2.AuthCodeRedirectURL(idp.Endpoint, state, req.RedirectURL, "", "")
	return redirectURL, nil
}

// 由前端处理idp回调后, 在请求当前
// SP处理SSO回应(Oauth2)
// func (s *settingService) ServiceProviderSsoOauth2Callback(ctx context.Context, req
// *iapiserver.SpSsoOauth2CallbackRequest) (string, error) {
// 	ott, err := s.store.OneTimeTokens().GetByHash(ctx, req.State)
// 	if err != nil {
// 		return "", errors.WithStack(err)
// 	}

// 	state, err := tokenutil.NewRSAJWTCodec(key, 0).Decode(req.State)
// 	if err != nil {
// 		return "", errors.WithStack(err)
// 	}

// 	redirectURL := idp.Oauth2.AuthCodeRedirectURL(idp.Endpoint, state, req.RedirectURL)
// 	return redirectURL, nil
// }

func generateOauth2State() string {
	return ""
}

func generateOauth2CodeVerifier() string {
	return ""
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func oauth2ExchangeCodeForToken(
	idp *iapiserver.IdentityProvider,
	frontendRedirectURI string,
	code string,
	codeVerifier string,
) (*TokenResponse, error) {
	httpResp, err := httpcli.NewHttpRequestBuilder().POST().
		WithEndpoint(idp.Endpoint+"/"+idp.Oauth2.TokenURI).
		AddQueryParam("grant_type", "authorization_code").
		AddQueryParam("code", code).
		AddQueryParam("redirect_uri", frontendRedirectURI).
		AddQueryParam("client_id", idp.Oauth2.ClientID).
		AddQueryParam("client_secret", idp.Oauth2.ClientSecret).
		AddQueryParam("code_verifier", codeVerifier).Build().Invoke()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var token TokenResponse

	if err := httpResp.Decode(&token); err != nil {
		return nil, errors.WithStack(err)
	}
	return &token, nil
}

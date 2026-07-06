package setting

import (
	"bytes"
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// 对接收到的sp的Oauth2请求进行验证回应
// 前后端分离架构，应该记录前端进行oauth2登录的路由.当这个Auth接口调用后，重定向到前端页面进行登录,再当用户填写账号密码登录后. 返回重定向路由前端重定向回sp。
func (s *settingService) Oauth2ProtocolSSOAuth(
	ctx context.Context,
	req *iapiserver.IdpServeOauth2SSOAuthorizeRequest,
) (*bytes.Buffer, error) {
	sp, err := s.store.ServiceProviders().GetByKey(ctx, iapiserver.SSOProtocolOAUTH2, req.ClientID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(sp.Oauth2.RedirectURIs) != 0 && !sets.NewString(req.RedirectURI).HasAnyPrefix(sp.Oauth2.RedirectURIs...) {
		return nil, errors.Errorf("invalid redirect uri:%v", req.RedirectURI)
	}

	if sp.Oauth2.Type == iapiserver.Oauth2SpTypePublic && req.CodeChallenge == "" {
		return nil, errors.Errorf("invalid request,PKCE required")
	}

	// 生成授权码
	// 重定向URI

	// switch sp.Oauth2.Type {
	// case iapiserver.Oauth2SpTypeConfidential:

	// case iapiserver.Oauth2SpTypePublic:
	// default:
	// 	return nil, errors.Errorf("unsupport oauth2 type")
	// }
	//frontendRedirectURI:=""
	return nil, nil
}

package setting

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// 当前服务作为sp时，添加第三方idp应用作为单点登录提供商
// 在服务器使用第三方idp进行单点登陆前，需要先执行以下的步骤：
//  1. 通过idp的公共端点下载idp的元数据文件
//  2. 生成当前服务器的sp元数据文件，
//  3. 将sp元数据文件注册到idp服务
func (s *settingService) ServiceProviderList(
	ctx context.Context,
	req *iapiserver.ServiceProviderListRequest,
) (*iapiserver.ServiceProviderListResponse, error) {
	metas, total, err := s.store.ServiceProviders().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	resp := &iapiserver.ServiceProviderListResponse{}
	resp.Total = total
	resp.List = metas
	return resp, nil
}

func (s *settingService) ServiceProviderGet(
	ctx context.Context,
	req *iapiserver.ServiceProviderGetRequest,
) (*iapiserver.ServiceProviderGetResponse, error) {
	meta, err := s.store.ServiceProviders().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.ServiceProviderGetResponse{ServiceProvider: *meta}, nil
}

func (s *settingService) ServiceProviderGetByKey(
	ctx context.Context,
	key string,
) (*iapiserver.ServiceProviderGetResponse, error) {
	metas, _, err := s.store.ServiceProviders().
		List(ctx, &iapiserver.ServiceProviderListRequest{Protocol: iapiserver.SSOProtocolSAML})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, meta := range metas {
		if meta.SAML.Key == key {
			return &iapiserver.ServiceProviderGetResponse{ServiceProvider: *meta}, nil
		}
	}

	return nil, errors.Errorf("not service provider with key '%s'", key)
}

func (s *settingService) ServiceProviderAdd(
	ctx context.Context,
	req *iapiserver.ServiceProviderAddRequest,
) (*iapiserver.ServiceProviderAddResponse, error) {
	meta, err := s.store.ServiceProviders().Add(ctx, &req.ServiceProvider)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.ServiceProviderAddResponse{ServiceProvider: *meta}, nil
}

func (s *settingService) ServiceProviderUpdate(
	ctx context.Context,
	req *iapiserver.ServiceProviderUpdateRequest,
) error {
	if _, err := s.store.ServiceProviders().Update(ctx, &req.ServiceProvider); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *settingService) ServiceProviderDelete(
	ctx context.Context,
	req *iapiserver.ServiceProviderDeleteRequest,
) error {
	err := s.store.ServiceProviders().Delete(ctx, req.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// ServiceProviderGetRedirectURL 获取ServiceProvider应用页面触发SSO登录的URL
// 一般情况SSO的发起是在SP的页面上进行。如果需要集成sp应用到idp, 即登录到idp后，点击sp应用即可'免密'登录到sp, 则需要获取sp的登录页上触发sso的url.
func (s *settingService) ServiceProviderGetRedirectURL(
	ctx context.Context,
	req *iapiserver.ServiceProviderGetRequest,
) (*iapiserver.ServiceProviderGetRedirectURLResponse, error) {
	meta, err := s.store.ServiceProviders().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	resp := &iapiserver.ServiceProviderGetRedirectURLResponse{}
	switch meta.Type {
	case iapiserver.ServiceProviderTypeGitlab:
		resp.Param = make(map[string]interface{})
		resp.Header = make(map[string]string)
		resp.Method = "GET"
		resp.Host = meta.Endpoint
		redirectURL, err := getGitlabCookieAndToken(meta.Endpoint)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		resp.URL = redirectURL
	case iapiserver.ServiceProviderTypeJira:
		resp.Method = "GET"
		resp.URL = meta.Endpoint + "/plugins/servlet/samlsso?idp=1"
		resp.Host = meta.Endpoint
		resp.URI = "/plugins/servlet/samlsso?idp=1"

	case iapiserver.ServiceProviderTypeConfluence:
		resp.Method = "GET"
		resp.URL = url.PathEscape(resp.URL)
		resp.Host = meta.Endpoint
		resp.URI = "/plugins/servlet/samlsso?" + url.PathEscape("redirectTo=/")
		resp.URL = meta.Endpoint + resp.URI

	default:
		resp.Method = "GET"
		resp.URL = meta.Endpoint
		resp.Host = meta.Endpoint
		resp.URI = ""
	}
	return resp, nil
}

func getGitlabCookieAndToken(endpoint string) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", errors.WithStack(err)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 返回错误以阻止重定向
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", endpoint+"/users/sign_in", nil)
	if err != nil {
		return "", errors.WithStack(err)
	}
	req.Header.Add(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	)
	client.Jar = jar
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.WithStack(err)
	}

	re := regexp.MustCompile(`<meta name="csrf-token" content="([^"]*)"`)
	matches := re.FindStringSubmatch(string(body))
	var token string
	if len(matches) > 1 {
		token = matches[1]
	} else {
		return "", errors.Errorf("CSRF token not found")
	}

	formData := url.Values{}
	formData.Set("_method", "post")
	formData.Set("authenticity_token", token)
	data := formData.Encode()
	req, err = http.NewRequest("POST", endpoint+"/users/auth/saml", bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", errors.WithStack(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	)
	resp, err = client.Do(req)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.WithStack(err)
	}
	rb := string(responseBody)
	rb = strings.TrimPrefix(rb, "Redirecting to ")
	rb = strings.TrimSuffix(rb, "...")
	if resp.StatusCode != http.StatusFound {
		return "", errors.Errorf("status code no 302, is %v,err:%v", resp.StatusCode, rb)
	}
	return rb, nil
}

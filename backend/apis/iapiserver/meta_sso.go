package iapiserver

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/crewjam/saml/samlsp"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/sets"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	SsoSpURLSamlInitiator = "/v1/auth/sso/sp/saml/initiator"
	SsoSpURLSamlACS       = "/v1/auth/sso/sp/saml/acs"
	SsoSpURLSamlSLO       = "/v1/auth/sso/sp/saml/slo"
	SsoSpURLSamlMetadata  = "/v1/auth/sso/sp/saml/metadata"
)

const (
	SsoIdpURLAnswer   = "/v1/auth/sso/idp/saml/answer"
	SsoIdpURLMetadata = "/v1/auth/sso/idp/saml/metadata"
)

const (
	SSOProtocolSAML   = "saml"
	SSOProtocolOAUTH2 = "oauth2"
)

type SAMLIdP struct {
	Metadata string `json:"metadata"`
}

func (s *SAMLIdP) Validate() error {
	if s == nil {
		return errors.Errorf("saml is empty when protocol is saml")
	}

	if s.Metadata == "" {
		return errors.Errorf("idp metadata is empty")
	}

	if _, err := samlsp.ParseMetadata([]byte(s.Metadata)); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type Oauth2Idp struct {
	// Idp的验证URI
	AuthorizeURI string `json:"authorize_uri"`
	// Idp的令牌URI
	TokenURI string `json:"token_uri"`
	// Idp的查询用户信息URI
	UserInfoURI string `json:"user_info_uri"`
	// UserInfoURI返回信息中的用户名字段
	UserNameField string `json:"user_name_field"`
	// 在Oauth2Idp注册的应用ID
	ClientID string `json:"client_id"`
	// 在Oauth2Idp注册的应用秘钥
	ClientSecret string `json:"client_secret"`
	//
	Scopes []string `json:"scopes"`
}

func (s *Oauth2Idp) Validate() error {
	if s == nil {
		return errors.Errorf("Oauth2Idp is empty when protocol is oauth2")
	}

	if sliceutil.ZeroCount(
		[]string{s.AuthorizeURI, s.TokenURI, s.UserInfoURI, s.UserNameField, s.ClientID, s.ClientSecret},
	) != 0 {
		return errors.Errorf("Oauth2Idp param exists empty data")
	}
	return nil
}

// 生成重定向到idp进行sso验证的url
func (s *Oauth2Idp) AuthCodeRedirectURL(
	endpoint, state, frontEndRedirectURL string,
	codeChallenge, CodeChallengeMethod string,
) string {
	authURL := endpoint + "/" + s.AuthorizeURI
	var buf bytes.Buffer
	buf.WriteString(authURL)

	v := url.Values{
		"response_type": {"code"},
		"client_id":     {s.ClientID},
	}

	v.Set("redirect_uri", frontEndRedirectURL)
	if len(s.Scopes) > 0 {
		v.Set("scope", strings.Join(s.Scopes, " "))
	}
	if state != "" {
		v.Set("state", state)
	}

	if codeChallenge != "" {
		v.Set("code_challenge", codeChallenge)
	}
	if CodeChallengeMethod != "" {
		v.Set("code_challenge_method", codeChallenge)
	}

	if strings.Contains(authURL, "?") {
		buf.WriteByte('&')
	} else {
		buf.WriteByte('?')
	}
	buf.WriteString(v.Encode())
	return buf.String()
}

type SpSAMLMetadata struct {
	Endpoint          string `json:"endpoint"`
	Key               string `json:"key"`
	Cert              string `json:"cert"`
	AuthnNameIDFormat string `json:"authn_name_id_format"`
}

/* 当前服务作为sso sp时，记录信任的idP*/

// sso idp(单点登录身份提供商验证信息)
type IdentityProvider struct {
	imachinery.ObjectMeta
	Enable   bool       `json:"enable"`
	Protocol string     `json:"protocol"`
	Endpoint string     `json:"endpoint"`
	SAML     *SAMLIdP   `json:"saml"     gorm:"-"`
	Oauth2   *Oauth2Idp `json:"oauth2"   gorm:"-"`
}

func (IdentityProvider) TableName() string {
	return "identity_provider"
}

func (s IdentityProvider) Validate() error {
	switch s.Protocol {
	case SSOProtocolSAML:
		if err := s.SAML.Validate(); err != nil {
			return errors.WithStack(err)
		}

	case SSOProtocolOAUTH2:
		if err := s.Oauth2.Validate(); err != nil {
			return errors.WithStack(err)
		}
	}
	return errors.Errorf("invalid sso protocol")
}

func (s *IdentityProvider) BeforeCreate(tx *gorm.DB) error {
	if s.Extend == nil {
		s.Extend = make(map[string]any)
	}
	if s.SAML != nil {
		s.Extend["saml"] = json.ToString(s.SAML)
	}
	if s.Oauth2 != nil {
		s.Extend["oauth2"] = json.ToString(s.Oauth2)
	}

	if err := s.ObjectMeta.BeforeCreate(tx); err != nil {
		return errors.Errorf("failed to run `BeforeCreate` hook: %v", err)
	}
	return nil
}

// AfterCreate run after create database record.
func (s *IdentityProvider) AfterCreate(tx *gorm.DB) error {
	return tx.Save(s).Error
}

// BeforeUpdate run before update database record.
func (s *IdentityProvider) BeforeUpdate(tx *gorm.DB) error {
	if s.SAML != nil {
		s.SetExtendValue("saml", s.SAML)
	}

	if s.Oauth2 != nil {
		s.SetExtendValue("oauth2", s.Oauth2)
	}
	if err := s.ObjectMeta.BeforeUpdate(tx); err != nil {
		return fmt.Errorf("failed to run `BeforeUpdate` hook: %v", err)
	}
	return nil
}

func (s *IdentityProvider) AfterFind(tx *gorm.DB) error {
	if s.Protocol == SSOProtocolSAML {
		if err := json.Unmarshal([]byte(s.ExtendShadow), &s.SAML); err != nil {
			return errors.WithStack(err)
		}
	}
	if s.Protocol == SSOProtocolOAUTH2 {
		if err := json.Unmarshal([]byte(s.ExtendShadow), &s.Oauth2); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

type IdpSAMLMetadata struct {
	Endpoint          string `json:"endpoint"`
	Key               string `json:"key"`
	Cert              string `json:"cert"`
	AuthnNameIDFormat string `json:"authn_name_id_format"`
}

/* 当前服务作为sso idp时，记录信任的sp*/
const (
	ServiceProviderTypeGitlab     = "gitlab"
	ServiceProviderTypeJira       = "jira"
	ServiceProviderTypeConfluence = "confluence"
	ServiceProviderTyUnknown      = "unknown"
)

// sso sp(单点登录服务提供商)
type ServiceProvider struct {
	imachinery.ObjectMeta
	Enable   bool   `json:"enable"`
	Protocol string `json:"protocol" binding:"required,oneof=saml oauth2"`
	Endpoint string `json:"endpoint" binding:"required"`
	// sp应用类型
	Type string `json:"type"     binding:"required"`

	SAML   *SAMLSp   `json:"saml"   gorm:"-"`
	Oauth2 *Oauth2Sp `json:"oauth2" gorm:"-"`
}

func (ServiceProvider) TableName() string {
	return "service_provider"
}

func (s ServiceProvider) Validate() error {
	if !stringutil.HasAnyPrefix(s.Endpoint, "http://", "https://") {
		return errors.Errorf("invalid endpoint, must contain scheme")
	}

	switch s.Type {
	case ServiceProviderTypeGitlab:
	case ServiceProviderTypeJira:
	case ServiceProviderTypeConfluence:
	default:
		s.Type = "unknown"
	}

	switch s.Protocol {
	case SSOProtocolSAML:
		if err := s.SAML.Validate(); err != nil {
			return errors.WithStack(err)
		}
	case SSOProtocolOAUTH2:
		if err := s.SAML.Validate(); err != nil {
			return errors.WithStack(err)
		}
	}
	return errors.Errorf("invalid sso protocol:%v", s.Protocol)
}

type SAMLSp struct {
	Key      string `json:"key"`
	Metadata string `json:"metadata"`
}

func (s *SAMLSp) Validate() error {
	if s == nil {
		return errors.Errorf("saml is empty when protocol is saml")
	}

	if s.Metadata == "" {
		return errors.Errorf("sp metadata is empty")
	}
	ed, err := samlsp.ParseMetadata([]byte(s.Metadata))
	if err != nil {
		return errors.WithStack(err)
	}
	s.Key = ed.EntityID
	return nil
}

const (
	// 公共客户端(比如说浏览器,或者无数据库的桌面应用). 无法安全存储client_secret, 通常采用client_id+PKCE
	Oauth2SpTypePublic = "public"
	// 机密客户端. 验证需要提交client_id和client_secret,oauth2.1则要求也需要使用PKCE
	Oauth2SpTypeConfidential = "confidential"
)

type Oauth2Sp struct {
	RedirectURIs []string `json:"redirect_uris"`
	Type         string   `json:"string"` // 应用类型, public
	ClientID     string   `json:"client_id"`
	// 在Oauth2Idp注册的应用秘钥
	ClientSecret string `json:"client_secret"`
}

func (s *Oauth2Sp) Validate() error {
	if s == nil {
		return errors.Errorf("Oauth2Sp is empty when protocol is oauth2")
	}

	if sliceutil.ZeroCount([]string{s.ClientID, s.ClientSecret}) != 0 {
		return errors.Errorf("ouath2Sp param exists empty data")
	}

	if !sets.NewString(Oauth2SpTypeConfidential, Oauth2SpTypePublic).Has(s.Type) {
		return errors.Errorf("ouath2Sp wrong type: %v", s.Type)
	}
	return nil
}

func (s *ServiceProvider) BeforeCreate(tx *gorm.DB) error {
	if s.SAML != nil {
		s.Extend = s.Extend.Set("saml", s.SAML)
	}

	if s.Oauth2 != nil {
		s.Extend = s.Extend.Set("oauth2", s.SAML)
	}

	if err := s.ObjectMeta.BeforeCreate(tx); err != nil {
		return errors.Errorf("failed to run `BeforeCreate` hook: %v", err)
	}
	return nil
}

// AfterCreate run after create database record.
func (s *ServiceProvider) AfterCreate(tx *gorm.DB) error {
	return tx.Save(s).Error
}

// BeforeUpdate run before update database record.
func (s *ServiceProvider) BeforeUpdate(tx *gorm.DB) error {
	if s.SAML != nil {
		s.SetExtendValue("saml", s.SAML)
	}

	if s.Oauth2 != nil {
		s.SetExtendValue("oauth2", s.Oauth2)
	}

	if err := s.ObjectMeta.BeforeUpdate(tx); err != nil {
		return fmt.Errorf("failed to run `BeforeUpdate` hook: %w", err)
	}
	return nil
}

func (s *ServiceProvider) AfterFind(tx *gorm.DB) error {
	if s.Protocol == SSOProtocolSAML {
		if err := json.Unmarshal([]byte(s.ExtendShadow), &s.SAML); err != nil {
			return errors.WithStack(err)
		}
	}
	if s.Protocol == SSOProtocolOAUTH2 {
		if err := json.Unmarshal([]byte(s.ExtendShadow), &s.Oauth2); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

package iapiserver

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/hash"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	defaultOttTTL = 5 * time.Minute
)
const (
	OneTimeTokenTypeSAML   = "saml"
	OneTimeTokenTypeOauth2 = "oauth2"
)

// 一次性令牌, 用完即销毁
type OneTimeToken struct {
	imachinery.ObjectMeta
	Type        string          `json:"type"    gorm:"column:type;index"                      binding:"required,oneof=saml"`
	TTLSeconds  int64           `json:"ttl"     gorm:"-"                                      binding:"required"` // ttl seconds
	Payload     string          `json:"payload" gorm:"payload"                                binding:"required"`
	PayloadHash string          `json:"-"       gorm:"column:payload_hash;type:text;not null"`
	ExpiresAt   imachinery.Time `json:"-"       gorm:"column:expires_at;index;not null"` // 过期时间
	Used        bool            `json:"-"       gorm:"column:used;default:false;index"`
}

// gorm数据库钩子
// BeforeCreate run before create database record.
func (obj *OneTimeToken) BeforeCreate(tx *gorm.DB) error {
	ttl := defaultOttTTL
	if obj.TTLSeconds > 0 {
		ttl = time.Duration(obj.TTLSeconds) * time.Second
	}
	obj.ExpiresAt = imachinery.NewTime(time.Now().Add(ttl))

	if err := obj.ObjectMeta.BeforeCreate(tx); err != nil {
		return errors.WithStack(err)
	}
	obj.PayloadHash, _ = hash.NewSha512().Sum(obj.Payload)
	return nil
}

// 一次性令牌, 用完即销毁
type SAMLOneTimeToken struct {
}

// oauth2 授权状态记录
type Oauth2StateRecord struct {
	SessionID    string    // 绑定用户会话
	ClientIP     string    // 记录请求源IP
	UserAgent    string    // 客户端指纹
	CreatedAt    time.Time // 创建时间
	PKCEVerifier string    // PKCE验证码
}

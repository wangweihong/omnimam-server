package iapiserver

import (
	"github.com/wangweihong/gotoolbox/pkg/randutil"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	UserTypeLocal = "local"
	UserTypeLdap  = "ldap"
	UserTypeSSO   = "sso"
)

// +k8s:deepcopy-gen=true
type User struct {
	imachinery.ObjectMeta
	Password string `json:"password,omitempty"`
	Mail     string `json:"mail"`
	Phone    string `json:"phone"`
	Type     string `json:"type"` //用户来源： 本地创建，ldap同步, sso单点创建?
	Source   string `json:"source"`
	Default  bool   `json:"default"` // 默认用户
}

func (u *User) Transfer() *User {
	u.Password = ""
	return nil
}

type UserToken struct {
	imachinery.ObjectMeta

	Token    string `json:"token"`
	ClientIP string `json:"client_ip"`
	UserID   string `json:"user_id"`
}

type UserOTP struct {
	imachinery.ObjectMeta

	Secret string `json:"secret"  gorm:"not null"`
	UserID string `json:"user_id"`
}

// gorm数据库钩子
// BeforeCreate run before create database record.
func (obj *UserOTP) BeforeCreate(tx *gorm.DB) error {
	// 生成10组重置otp密码
	obj.Extend = obj.Extend.Set("reset_password", randutil.RandStringSlice(nil, 10, 10))
	return obj.BeforeCreate(tx)
}

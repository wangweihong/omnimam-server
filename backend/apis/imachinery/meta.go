package imachinery

import (
	"maps"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"gorm.io/gorm"
)

// Extend defines a new type used to store extended fields.
type Extend struct {
	maputil.StringAny
}

// String returns the string format of Extend.
func (ext Extend) String() string {
	data, _ := json.Marshal(ext)
	return string(data)
}

func (ext *Extend) DeepCopyInto(out *Extend) {
	if ext == nil || out == nil {
		return
	}
	if ext.StringAny != nil {
		out.StringAny = ext.StringAny.DeepCopy()
		return
	}
}

func (ext *Extend) DeepCopy() *Extend {
	if ext == nil {
		return nil
	}
	out := new(Extend)
	if ext.StringAny != nil {
		out.StringAny = ext.StringAny.DeepCopy()
		return out
	}
	return out
}

// Merge merge extend fields from extendShadow.q
func (ext Extend) Merge(extendShadow string) Extend {
	var extend Extend

	// always trust the extendShadow in the database
	_ = json.Unmarshal([]byte(extendShadow), &extend)
	ext.StringAny = maputil.Copy(extend.StringAny, ext.StringAny)
	return ext
}

// TypeMeta describes an individual object in an API response or request
// with strings representing the type of the object and its API schema version.
// Structures that are versioned or persisted should inline TypeMeta.
type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// required: false
	Kind string `json:"kind,omitempty"`

	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	APIVersion string `json:"apiVersion,omitempty"`
}

// ListMeta describes metadata that synthetic resources must have, including lists and
// various status objects. A resource may have only one of {ObjectMeta, ListMeta}.
type ListMeta struct {
	TotalCount int64 `json:"totalCount,omitempty"`
}

// ObjectMeta 是 SSOT S2 通用资源元数据，所有标准持久化资源优先嵌入该结构。
// 数据库列、OpenAPI 请求/响应字段均使用 lower_snake_case；GORM 也通过该结构维护通用字段。
type ObjectMeta struct {
	// ID 是资源唯一标识，由服务端在创建时生成；客户端不应在更新时修改。
	ID string `json:"id,omitempty" form:"id" gorm:"primary_key;column:id;type:text"`

	// Name 是资源名称或内部幂等名称；是否必填和格式约束由具体请求 DTO 决定。
	Name string `json:"name,omitempty" form:"name" gorm:"column:name;type:text;not null" binding:"omitempty"`

	// Extend 存放低频、实验性或第三方扩展字段；需要查询、过滤、排序或强校验的字段不得放入此处。
	Extend maputil.StringAny `json:"extend,omitempty" gorm:"-" binding:"omitempty"`

	// ExtendShadow 是 Extend 的数据库影子字段，固定保存 JSON 字符串；业务代码不要直接修改。
	ExtendShadow string `json:"-" gorm:"column:extend_shadow;type:text;default:''" binding:"omitempty"`

	// CreatedAt 是服务端创建资源的时间，数据库和 API 均使用 lower_snake_case。
	CreatedAt Time `json:"created_at,omitempty" gorm:"column:created_at;type:timestamptz;not null"`

	// UpdatedAt 是服务端最后更新资源的时间，数据库和 API 均使用 lower_snake_case。
	UpdatedAt Time `json:"updated_at,omitempty" gorm:"column:updated_at;type:timestamptz;not null"`

	// Description 是资源描述，供列表、详情和审计场景展示。
	Description string `json:"description,omitempty" gorm:"column:description;type:text;default:''" binding:"omitempty,description"`

	// ResourceVersion 是资源版本号，用于后续并发控制、审计或增量同步。
	ResourceVersion int64 `json:"resource_version" gorm:"column:resource_version;type:integer;default:0"`
}

func (obj *ObjectMeta) DeepCopyInto(target *ObjectMeta) {
	target.ID = obj.ID
	target.Name = obj.Name
	target.CreatedAt = obj.CreatedAt
	target.UpdatedAt = obj.UpdatedAt
	target.Description = obj.Description
	target.ResourceVersion = obj.ResourceVersion
	target.ExtendShadow = obj.ExtendShadow

	target.Extend = maputil.Clone(obj.Extend)
	if target.Extend == nil {
		target.Extend = make(map[string]any)
	}

	maps.Copy(target.Extend, obj.Extend)
}

// gorm数据库钩子
// BeforeCreate run before create database record.
func (obj *ObjectMeta) BeforeCreate(tx *gorm.DB) error {
	if obj.ID == "" {
		obj.ID = uuid.New().String()
	}

	if obj.Extend == nil {
		obj.Extend = maputil.StringAny{}
	}
	obj.ExtendShadow = obj.Extend.String()
	obj.CreatedAt = NewTime(time.Now())
	obj.UpdatedAt = NewTime(time.Now())
	obj.ResourceVersion = 1

	return nil
}

// AfterCreate 为嵌入 ObjectMeta 的持久化资源提供统一无副作用创建后 hook。
func (*ObjectMeta) AfterCreate(*gorm.DB) error { return nil }

// BeforeUpdate run before update database record.
func (obj *ObjectMeta) BeforeUpdate(tx *gorm.DB) error {
	if obj.Extend == nil {
		obj.Extend = maputil.StringAny{}
	}
	obj.ExtendShadow = obj.Extend.String()
	obj.UpdatedAt = NewTime(time.Now())
	obj.ResourceVersion++

	return nil
}

// AfterUpdate 为嵌入 ObjectMeta 的持久化资源提供统一无副作用更新后 hook。
func (*ObjectMeta) AfterUpdate(*gorm.DB) error { return nil }

// AfterFind run after find to unmarshal an extend shadow string into mExtend struct.
func (obj *ObjectMeta) AfterFind(tx *gorm.DB) error {
	if obj.ExtendShadow == "" {
		obj.Extend = maputil.StringAny{}
		return nil
	}
	if err := json.Unmarshal([]byte(obj.ExtendShadow), &obj.Extend); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (obj *ObjectMeta) SetExtendValue(key string, value any) {
	if key == "" {
		return
	}
	obj.Extend = obj.Extend.Set(key, value)
}

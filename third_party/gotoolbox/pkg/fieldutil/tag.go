package fieldutil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// 字段和标签信息
// 注意字段可能是embedded struct, 通过Field.Anonymous来判定
type FieldTag struct {
	Field reflect.StructField
	Tag   string
}

type FieldTags []FieldTag

func (fts FieldTags) ToMap() map[string]FieldTag {
	ftm := make(map[string]FieldTag, len(fts))
	for _, v := range fts {
		ftm[v.Field.Name] = v
	}
	return ftm
}

func (fts FieldTags) ToTagMap() map[string]FieldTag {
	ftm := make(map[string]FieldTag, len(fts))
	for _, v := range fts {
		if v.Tag != "" {
			ftm[v.Tag] = v
		}
	}
	return ftm
}

// ParseStructFields 从结构体或者结构体指针(指针非空值)中获取其字段以及标签信息
func ParseStructFieldTags(s any, tagName string, HideField ...string) FieldTags {
	fs := ParseStructFieldValues(s)
	if fs == nil {
		return nil
	}

	hideFieldSets := sets.NewString(HideField...)

	fts := make([]FieldTag, 0, len(fs))
	for _, v := range fs {
		// 忽略非导出字段
		if !v.T.IsExported() {
			continue
		}

		if hideFieldSets.Has(v.T.Name) {
			continue
		}

		tag := v.T.Tag.Get(tagName)
		tag = strings.TrimSuffix(tag, ",omitempty")
		if tag == "-" {
			continue
		}

		if hideFieldSets.Has(tag) {
			continue
		}

		fts = append(fts, FieldTag{
			Field: v.T,
			Tag:   tag,
		})
	}
	return fts
}

func (fts FieldTags) Tags() []string {
	tags := make([]string, 0, len(fts))
	for _, v := range fts {
		if v.Tag != "" {
			tags = append(tags, v.Tag)
		}
	}
	return tags
}

func GetFieldTagMapping(p any) map[string]string {
	fmt.Println(p)
	fts := ParseStructFieldTags(p, JsonTag)

	fieldTags := make(map[string]string, 0)

	for _, v := range fts {
		key := v.Tag
		if key == "" {
			key = v.Field.Name
		}
		fieldTags[key] = v.Field.Name
	}
	return fieldTags
}

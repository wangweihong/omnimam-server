package imachinery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BasicQueryParam struct {
	// 公共参数
	PagingParams
	Keyword        string   `json:"keyword"         form:"keyword"`        // 模糊搜索关键字
	SearchFields   []string `json:"search_fields"   form:"search_fields"`  // 模糊搜索字段（如 ["name", "description"]）
	SortField      string   `json:"sort_field"      form:"sort_field"`     // 排序字段
	SortOrder      string   `json:"sort_order"      form:"sort_order"`     // 排序方向（asc/desc）
	CreatedAfter   int64    `json:"created_after"   form:"created_after"`  // 创建时间范围
	CreatedBefore  int64    `json:"created_before"  form:"created_before"` // 创建时间范围
	SpecificFilter string   `json:"specific_filter" form:"specific_filter"`

	SpecificFilterShadow map[string]string `json:"-"`
}

/*
resourceSpecificFilter example:

	func(q *gorm.DB) *gorm.DB {
	        if params.Framework != "" {
	            q = q.Where("framework = ?", params.Framework)
	        }
	        if params.Type != "" {
	            q = q.Where("type = ?", params.Type)
	        }
	        return q
	    }
*/
func (params BasicQueryParam) ToQuery(
	ctx context.Context,
	db *gorm.DB,
	resourceSpecificFilter func(*gorm.DB) *gorm.DB,
) *gorm.DB {
	query := db.WithContext(ctx)
	// 1. 模糊搜索
	if params.Keyword != "" {
		searchPattern := "%" + params.Keyword + "%"
		fields := params.SearchFields
		if len(fields) == 0 {
			fields = []string{"name", "description"}
		}

		orConditions := make([]string, 0, len(fields))
		args := make([]any, 0, len(fields))

		for _, field := range fields {
			orConditions = append(orConditions, fmt.Sprintf("%s LIKE ?", field))
			args = append(args, searchPattern)
		}
		if len(orConditions) > 0 {
			query = query.Where(strings.Join(orConditions, " OR "), args...)
		}
	}

	// 2. 应用资源特定过滤
	if resourceSpecificFilter != nil {
		query = resourceSpecificFilter(query)
	}

	// 3. 时间范围过滤
	if params.CreatedAfter != 0 {
		t := time.Unix(params.CreatedAfter, 0)
		query = query.Where("created_at >= ?", t)
	}

	if params.CreatedBefore != 0 {
		t := time.Unix(params.CreatedBefore, 0)
		query = query.Where("created_at <= ?", t)
	}

	// 4. 排序处理
	if params.SortField != "" && isValidFieldName(params.SortField) {
		order := params.SortField
		if strings.ToLower(params.SortOrder) == "desc" {
			order += " DESC"
		} else {
			order += " ASC"
		}
		query = query.Order(order)
	} else {
		query = query.Order("created_at DESC")
	}
	// 5. 分页处理
	if params.PageNum > 0 && params.PageSize > 0 {
		// 限制最大页大小
		if params.PageSize > 1000 {
			params.PageSize = 1000
		}
		offset := (params.PageNum - 1) * params.PageSize
		query = query.Offset(offset).Limit(params.PageSize)
	}
	return query
}

func isValidFieldName(field string) bool {
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(field)
}

type ListRet struct {
	Total int64 `json:"total"`
}

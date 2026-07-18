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

// Validate 在 controller 边界统一校验分页参数和 offset 溢出。
func (params BasicQueryParam) Validate() error {
	_, err := params.PagingParams.Normalize()
	return err
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
	query := params.ToUnpaginatedQuery(ctx, db, resourceSpecificFilter)
	window, err := params.PagingParams.Normalize()
	if err != nil {
		query.AddError(err)
		return query
	}
	return ApplyPagination(query, window)
}

// ToUnpaginatedQuery 构建仅包含过滤和排序的查询，供计数后再分页的列表接口使用。
func (params BasicQueryParam) ToUnpaginatedQuery(
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
	return query
}

// ApplyPagination 将已归一化的分页窗口应用到 GORM 查询。
func ApplyPagination(query *gorm.DB, window PageWindow) *gorm.DB {
	return query.Offset(window.Offset).Limit(window.Limit)
}

func isValidFieldName(field string) bool {
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(field)
}

type ListRet struct {
	Total int64 `json:"total"`
}

package imachinery

import (
	"fmt"
	"math"
)

const (
	// DefaultPageSize 是客户端未传或传入零值时的全局默认返回数量。
	DefaultPageSize = 500
	// MaxPageSize 是所有 HTTP 列表接口允许的全局单页数量上限。
	MaxPageSize = 500
)

// +k8s:deepcopy-gen=true
type PagingParams struct {
	// PageNum 是从 0 开始的页码，负数会触发参数校验错误。
	PageNum int `json:"page_num" form:"page_num"`
	// PageSize 是期望返回的数量，零值默认为 500，超过 500 时截断为 500。
	PageSize int `json:"page_size" form:"page_size"`
}

// PageWindow 是经全局规则归一化后的零基分页窗口。
type PageWindow struct {
	Offset int
	Limit  int
}

// Normalize 校验分页参数并返回全局统一的查询窗口。
func (params PagingParams) Normalize() (PageWindow, error) {
	if params.PageNum < 0 {
		return PageWindow{}, fmt.Errorf("page_num must be greater than or equal to zero")
	}
	if params.PageSize < 0 {
		return PageWindow{}, fmt.Errorf("page_size must be greater than or equal to zero")
	}

	pageSize := params.PageSize
	if pageSize == 0 {
		pageSize = DefaultPageSize
	} else if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if params.PageNum > math.MaxInt/pageSize {
		return PageWindow{}, fmt.Errorf("pagination offset overflows int")
	}

	return PageWindow{Offset: params.PageNum * pageSize, Limit: pageSize}, nil
}

// PaginateSlice 对内存列表应用与数据库查询相同的分页语义。
func PaginateSlice[T any](items []T, window PageWindow) []T {
	if window.Offset >= len(items) {
		return []T{}
	}
	end := window.Offset + window.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[window.Offset:end]
}

// GetOptions is the standard query options to the standard REST get call.
type GetOptions struct {
}

// DeleteOptions may be provided when deleting an API object.
type DeleteOptions struct {
}

// CreateOptions may be provided when creating an API object.
type CreateOptions struct {
	DryRun bool
}

type PatchOptions struct {
}

type UpdateOptions struct {
}

type ListOptions struct {
	PagingParams
	// 模糊搜索字段, 支持传递多个过滤项，通过",”隔开
	Fuzzy     string `json:"fuzzy"      query:"fuzzy"`
	SortBy    string `json:"sort_by"    query:"sort_by"`
	SortField string `json:"sort_field" query:"sort_field"`
	SortAsc   bool   `json:"sort_asc"   query:"sort_asc"`
}

package imachinery

type PagingParams struct {
	PageNum  int `json:"page_num"  form:"page_num"`
	PageSize int `json:"page_size" form:"page_size"`
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

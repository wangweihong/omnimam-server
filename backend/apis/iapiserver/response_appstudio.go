package iapiserver

// +k8s:deepcopy-gen=true
type StudioApplicationListResponse struct {
	Total int64                `json:"total"`
	Items []*StudioApplication `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioSourceFileListResponse struct {
	Total int64               `json:"total"`
	Items []*StudioSourceFile `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioFileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Revision  int64  `json:"revision"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchHit struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Snippet    string `json:"snippet"`
	Revision   int64  `json:"revision"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioSourceSearchHit `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioBuildListResponse struct {
	Total int64          `json:"total"`
	Items []*StudioBuild `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioReleaseListResponse struct {
	Total int64            `json:"total"`
	Items []*StudioRelease `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionListResponse struct {
	Total int64                       `json:"total"`
	Items []*StudioApplicationVersion `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeInstanceListResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioRuntimeInstance `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeLogListResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioRuntimeLogEntry `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioOperationResult struct {
	Success bool `json:"success"`
}

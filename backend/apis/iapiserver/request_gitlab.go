package iapiserver

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	// GitLabTaskDomain 是 Task Center 校验 GitLab 来源调用的稳定 caller domain。
	GitLabTaskDomain = "gitlab"
	// GitLabFunctionPipelineRun 是通过 Task Center 观察一个远端 GitLab Pipeline 的 functionRef。
	GitLabFunctionPipelineRun = "gitlab.pipeline.run"
)

var gitLabProjectPathPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// GitLabServerCreateRequest 创建独立 GitLab 连接；credential 只写且不会返回。
type GitLabServerCreateRequest struct {
	// Name 是管理员可辨识的全局唯一连接名称。
	Name string `json:"name" binding:"required,max=200"`
	// Description 是连接用途说明，不参与 GitLab API 调用。
	Description string `json:"description" binding:"max=2000"`
	// APIURL 是包含 GitLab API v4 前缀的服务端调用地址。
	APIURL string `json:"api_url" binding:"required,url,max=2048"`
	// ExternalURL 是供管理员跳转的 GitLab Web 地址。
	ExternalURL string `json:"external_url" binding:"required,url,max=2048"`
	// NamespacePath 固定新建 Project 所属 Group，客户端不能覆盖 namespace ID。
	NamespacePath string `json:"namespace_path" binding:"required,max=255"`
	// Credential 是只写 PRIVATE-TOKEN，响应和日志不得返回。
	Credential string `json:"credential" binding:"required,max=4096"`
	// IsAppStudioDefault 仅允许 UNKNOWN 初始连接保持 false；READY 后通过更新接口设置默认。
	IsAppStudioDefault bool `json:"is_appstudio_default"`
}

func (r GitLabServerCreateRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" || strings.Trim(r.NamespacePath, "/ ") == "" || strings.TrimSpace(r.Credential) == "" {
		return fmt.Errorf("name, namespace_path, and credential are required")
	}
	if r.IsAppStudioDefault {
		return fmt.Errorf("a new unknown server cannot be the appstudio default")
	}
	return nil
}

// GitLabServerUpdateRequest 局部更新连接；nil 字段表示保留当前值。
type GitLabServerUpdateRequest struct {
	// Name 修改管理员可辨识名称；省略时保留原值。
	Name *string `json:"name" binding:"omitempty,min=1,max=200"`
	// Description 修改用途说明；空字符串表示清空。
	Description *string `json:"description" binding:"omitempty,max=2000"`
	// APIURL 修改服务端调用地址并将连接状态重置为 UNKNOWN。
	APIURL *string `json:"api_url" binding:"omitempty,url,max=2048"`
	// ExternalURL 修改管理员跳转地址，不改变连接检测状态。
	ExternalURL *string `json:"external_url" binding:"omitempty,url,max=2048"`
	// NamespacePath 修改固定 Group path 并将连接状态重置为 UNKNOWN。
	NamespacePath *string `json:"namespace_path" binding:"omitempty,min=1,max=255"`
	// Credential 替换只写 PRIVATE-TOKEN；省略时保留真实原值。
	Credential *string `json:"credential" binding:"omitempty,min=1,max=4096"`
	// IsAppStudioDefault 设置或清除 AppStudio 默认连接；设置为 true 要求 Server 已 READY。
	IsAppStudioDefault *bool `json:"is_appstudio_default"`
	// ResourceVersion 可选地执行乐观并发控制。
	ResourceVersion *int64 `json:"resource_version" binding:"omitempty,min=1"`
}

func (r GitLabServerUpdateRequest) Validate() error {
	if r.Name == nil && r.Description == nil && r.APIURL == nil && r.ExternalURL == nil && r.NamespacePath == nil && r.Credential == nil && r.IsAppStudioDefault == nil && r.ResourceVersion == nil {
		return fmt.Errorf("at least one field is required")
	}
	if (r.Name != nil && strings.TrimSpace(*r.Name) == "") || (r.NamespacePath != nil && strings.Trim(*r.NamespacePath, "/ ") == "") || (r.Credential != nil && strings.TrimSpace(*r.Credential) == "") {
		return fmt.Errorf("name, namespace_path, and credential cannot be blank")
	}
	return nil
}

// GitLabServerListRequest 查询 Server；列表始终排除 credential。
type GitLabServerListRequest struct {
	imachinery.BasicQueryParam
	// Status 过滤最近连接检测状态；空值表示不过滤。
	Status string `form:"status" binding:"omitempty,oneof=UNKNOWN READY ERROR"`
}

func (r *GitLabServerListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 20
	}
	if len(r.SearchFields) == 0 {
		r.SearchFields = []string{"name", "description"}
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

func (r GitLabServerListRequest) Validate() error {
	if err := r.BasicQueryParam.Validate(); err != nil {
		return err
	}
	if len(r.Keyword) > 200 || !gitLabAllowed(r.SortField, "", "name", "created_at", "updated_at", "status") || !gitLabAllowed(strings.ToLower(r.SortOrder), "asc", "desc") {
		return fmt.Errorf("gitlab server list query is invalid")
	}
	return nil
}

// GitLabProjectCreateRequest 在 READY Server 的固定 Namespace 创建 private Project。
type GitLabProjectCreateRequest struct {
	// GitLabServerID 是 READY GitLabServer 的 OmniMAM ID。
	GitLabServerID string `json:"gitlab_server_id" binding:"required,max=128"`
	// Name 是远端 Project 展示名称。
	Name string `json:"name" binding:"required,max=255"`
	// Path 是固定 Namespace 下的 URL path，只允许 GitLab 合法的基础字符。
	Path string `json:"path" binding:"required,max=255"`
	// Description 同时用于远端 Project 和本地投影说明。
	Description string `json:"description" binding:"max=2000"`
}

func (r GitLabProjectCreateRequest) Validate() error {
	if strings.TrimSpace(r.GitLabServerID) == "" || strings.TrimSpace(r.Name) == "" || !gitLabProjectPathPattern.MatchString(r.Path) {
		return fmt.Errorf("gitlab_server_id, name, or path is invalid")
	}
	return nil
}

// GitLabProjectListRequest 查询本地 Project 投影，可按 Server 过滤。
type GitLabProjectListRequest struct {
	imachinery.BasicQueryParam
	// GitLabServerID 按所属连接过滤本地 Project 投影。
	GitLabServerID string `form:"gitlab_server_id" binding:"omitempty,max=128"`
}

func (r *GitLabProjectListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 20
	}
	if len(r.SearchFields) == 0 {
		r.SearchFields = []string{"name", "description", "path_with_namespace"}
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

func (r GitLabProjectListRequest) Validate() error {
	if err := r.BasicQueryParam.Validate(); err != nil {
		return err
	}
	if len(r.Keyword) > 200 || !gitLabAllowed(r.SortField, "", "name", "created_at", "updated_at") || !gitLabAllowed(strings.ToLower(r.SortOrder), "asc", "desc") {
		return fmt.Errorf("gitlab project list query is invalid")
	}
	return nil
}

// GitLabPipelineRunTaskArguments 是 Task Center 的不可变业务输入，不包含远端 ID、URL 或 PAT。
type GitLabPipelineRunTaskArguments struct {
	// GitLabProjectID 是本地稳定 Project ID，Worker 据此解析远端连接。
	GitLabProjectID string `json:"gitlab_project_id" binding:"required,max=128"`
	// Ref 是 GitLab Pipeline 使用的 branch 或 tag ref。
	Ref string `json:"ref" binding:"required,max=255"`
	// Variables 是最多 100 项的 Pipeline 变量，不在任务输出中回显。
	Variables map[string]string `json:"variables,omitempty" binding:"max=100"`
}

type GitLabServerListResponse struct {
	Total int64           `json:"total"`
	Items []*GitLabServer `json:"items"`
}

type GitLabProjectListResponse struct {
	Total int64            `json:"total"`
	Items []*GitLabProject `json:"items"`
}

func gitLabAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

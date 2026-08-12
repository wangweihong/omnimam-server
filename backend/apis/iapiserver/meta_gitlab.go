package iapiserver

import (
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	// GitLabServerStatusUnknown 表示连接尚未检测或连接信息已改变。
	GitLabServerStatusUnknown = "UNKNOWN"
	// GitLabServerStatusReady 表示 API、credential 和 Namespace 最近一次检测成功。
	GitLabServerStatusReady = "READY"
	// GitLabServerStatusError 表示最近一次连接检测失败。
	GitLabServerStatusError     = "ERROR"
	GitLabProjectStatusCreating = "CREATING"
	GitLabProjectStatusReady    = "READY"
	GitLabProjectStatusError    = "ERROR"
)

// +k8s:deepcopy-gen=true
// GitLabServer 是独立 GitLab domain 的连接配置；Credential 不参与任何 JSON 响应。
type GitLabServer struct {
	imachinery.ObjectMeta
	// APIURL 是 OmniMAM Server 实际调用的 GitLab API v4 地址。
	APIURL string `json:"api_url" gorm:"column:api_url;type:text;not null"`
	// ExternalURL 是供管理界面跳转和展示 clone 地址的 GitLab Web 地址。
	ExternalURL string `json:"external_url" gorm:"column:external_url;type:text;not null"`
	// NamespacePath 是本 Server 新建 Project 固定使用的 Group path。
	NamespacePath string `json:"namespace_path" gorm:"column:namespace_path;type:text;not null"`
	// Credential 是 PRIVATE-TOKEN；禁止序列化、日志记录或进入 Task 输入输出。
	Credential string `json:"-" gorm:"column:credential;type:text;not null"`
	// Status 是最近连接检测投影，只允许 UNKNOWN、READY 或 ERROR。
	Status string `json:"status" gorm:"column:status;type:text;not null;default:'UNKNOWN';index"`
	// IsAppStudioDefault 表示该 READY Server 是否为 AppStudio 唯一默认连接。
	IsAppStudioDefault bool `json:"is_appstudio_default" gorm:"column:is_appstudio_default;not null;default:false"`
	// LastCheckedAt 是最近完成连接检测的服务端时间。
	LastCheckedAt *imachinery.Time `json:"last_checked_at,omitempty" gorm:"column:last_checked_at;type:timestamptz"`
	// LastError 是最近检测失败的脱敏摘要，不保存 GitLab 原始响应或 credential。
	LastError string `json:"last_error" gorm:"column:last_error;type:text;not null;default:''"`
}

func (GitLabServer) TableName() string { return "gitlab_servers" }
func (m *GitLabServer) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if m.Status == "" {
		m.Status = GitLabServerStatusUnknown
	}
	return nil
}
func (m *GitLabServer) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *GitLabServer) AfterCreate(*gorm.DB) error     { return nil }
func (m *GitLabServer) AfterUpdate(*gorm.DB) error     { return nil }
func (m *GitLabServer) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }

// +k8s:deepcopy-gen=true
// GitLabProject 是远端 GitLab Project 的本地投影；其他领域只能引用其 OmniMAM ID。
type GitLabProject struct {
	imachinery.ObjectMeta
	Status string `json:"status" gorm:"column:status;type:text;not null;default:'CREATING'"`
	// GitLabServerID 指向拥有远端连接和 credential 的 GitLabServer。
	GitLabServerID string `json:"gitlab_server_id" gorm:"column:gitlab_server_id;type:text;not null;index;uniqueIndex:idx_gitlab_project_external,priority:1;uniqueIndex:idx_gitlab_project_path,priority:1"`
	// ExternalProjectID 是 GitLab numeric project ID，仅允许在 GitLab domain 内使用。
	ExternalProjectID int64 `json:"external_project_id" gorm:"column:external_project_id;default:null;uniqueIndex:idx_gitlab_project_external,priority:2"`
	// Path 是 GitLab Namespace 内的 Project path。
	Path string `json:"path" gorm:"column:path;type:text;not null"`
	// PathWithNamespace 是 GitLab 返回的完整 Namespace/Project path。
	PathWithNamespace string `json:"path_with_namespace" gorm:"column:path_with_namespace;type:text;not null;uniqueIndex:idx_gitlab_project_path,priority:2"`
	// WebURL 是供管理员跳转的 GitLab Project 页面地址。
	WebURL string `json:"web_url" gorm:"column:web_url;type:text;not null;default:''"`
	// HTTPURLToRepo 是 GitLab 返回的 HTTP clone URL，不包含 credential。
	HTTPURLToRepo string `json:"http_url_to_repo" gorm:"column:http_url_to_repo;type:text;not null;default:''"`
	// SSHURLToRepo 是 GitLab 返回的 SSH clone URL。
	SSHURLToRepo string `json:"ssh_url_to_repo" gorm:"column:ssh_url_to_repo;type:text;not null;default:''"`
	// DefaultBranch 是 GitLab 返回的默认分支；空仓库时按合同回退为 main。
	DefaultBranch string `json:"default_branch" gorm:"column:default_branch;type:text;not null;default:'main'"`
}

func (GitLabProject) TableName() string { return "gitlab_projects" }
func (m *GitLabProject) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if m.DefaultBranch == "" {
		m.DefaultBranch = "main"
	}
	if m.Status == "" {
		m.Status = GitLabProjectStatusCreating
	}
	return nil
}
func (m *GitLabProject) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *GitLabProject) AfterCreate(*gorm.DB) error     { return nil }
func (m *GitLabProject) AfterUpdate(*gorm.DB) error     { return nil }
func (m *GitLabProject) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }

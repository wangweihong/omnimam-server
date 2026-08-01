package iapiserver

import (
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const MCPTasksExtensionID = "io.modelcontextprotocol/tasks"

// MCPTaskBinding 保存 MCP Task 到既有 ApplicationRun/AtomicTask 的短期协议映射。
// 它不复制运行状态、输入输出或制品内容；过期删除也不得修改源领域事实。
// +k8s:deepcopy-gen=true
type MCPTaskBinding struct {
	imachinery.ObjectMeta
	// MCPTaskID 是返回给已协商 Tasks 扩展客户端的稳定任务标识，不是授权凭证。
	MCPTaskID string `json:"mcp_task_id" gorm:"column:mcp_task_id;type:text;not null;uniqueIndex"`
	// PrincipalID 固定创建映射的 Identity 主体，每次读取仍需重新认证和对象授权。
	PrincipalID string `json:"-" gorm:"column:principal_id;type:text;not null;uniqueIndex:idx_mcp_binding_principal_run,priority:1;index:idx_mcp_binding_principal_access,priority:1"`
	// ClientName 记录非安全决策用途的 MCP clientInfo.name，禁止用它代替 Principal。
	ClientName string `json:"client_name,omitempty" gorm:"column:client_name;type:text;not null;default:''"`
	// ApplicationRunID 指向已经持久化并完成 AtomicTask 绑定的 ApplicationRun。
	ApplicationRunID string `json:"application_run_id" gorm:"column:application_run_id;type:text;not null;uniqueIndex:idx_mcp_binding_principal_run,priority:2"`
	// AtomicTaskID 固定创建时的 Task Center 事实标识，状态始终从 Task Center 现查。
	AtomicTaskID string `json:"atomic_task_id" gorm:"column:atomic_task_id;type:text;not null;index"`
	// ExtensionID v1 固定为 MCP Tasks 扩展，不接受自定义扩展或交互式状态机。
	ExtensionID string `json:"extension_id" gorm:"column:extension_id;type:text;not null;default:'io.modelcontextprotocol/tasks'"`
	// ExpiresAt 仅控制协议映射保留期，过期不删除 ApplicationRun、AtomicTask 或 Artifact。
	ExpiresAt imachinery.Time `json:"expires_at" gorm:"column:expires_at;type:timestamptz;not null;index"`
	// LastAccessedAt 用于受控 TTL 清理和访问观测，不延长源领域对象生命周期。
	LastAccessedAt imachinery.Time `json:"last_accessed_at" gorm:"column:last_accessed_at;type:timestamptz;not null;index:idx_mcp_binding_principal_access,priority:2,sort:desc"`
}

func (MCPTaskBinding) TableName() string { return "mcp_task_bindings" }
func (b *MCPTaskBinding) BeforeCreate(tx *gorm.DB) error {
	if b.ExtensionID == "" {
		b.ExtensionID = MCPTasksExtensionID
	}
	return b.ObjectMeta.BeforeCreate(tx)
}
func (*MCPTaskBinding) AfterCreate(*gorm.DB) error { return nil }
func (b *MCPTaskBinding) BeforeUpdate(tx *gorm.DB) error {
	return b.ObjectMeta.BeforeUpdate(tx)
}
func (*MCPTaskBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *MCPTaskBinding) AfterFind(tx *gorm.DB) error {
	return b.ObjectMeta.AfterFind(tx)
}

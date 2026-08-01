package store

import (
	"context"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// MCPStoreFactory 是 MCP composition root 消费的可选存储能力，不扩大通用 Factory 的测试替身表面。
type MCPStoreFactory interface {
	MCPTaskBindings() MCPTaskBindingStore
}

// MCPTaskBindingStore 只维护短期协议映射；调用方负责每次重新授权源 ApplicationRun。
type MCPTaskBindingStore interface {
	CreateOrGet(context.Context, *iapiserver.MCPTaskBinding) (*iapiserver.MCPTaskBinding, bool, error)
	GetByTaskID(context.Context, string) (*iapiserver.MCPTaskBinding, error)
	GetByPrincipalRun(context.Context, string, string) (*iapiserver.MCPTaskBinding, error)
	Touch(context.Context, string, time.Time) error
	DeleteExpired(context.Context, time.Time) (int64, error)
}

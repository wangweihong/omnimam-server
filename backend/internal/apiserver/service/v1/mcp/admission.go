package mcp

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/rate"

	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

var errAdmissionRateLimited = stderrors.New("MCP admission rate limit exceeded")

// AdmissionController 是 MCP 消费的配额边界；生产 adapter 可替换为共享配额服务，不能在 Tool 中硬编码策略来源。
type AdmissionController interface {
	AdmitRequest(context.Context, AdmissionRequest) error
	AdmitTool(context.Context, ToolAdmissionRequest) error
}

// AdmissionRequest 只携带低基数授权上下文，不包含 Token、完整参数或受控 URL。
type AdmissionRequest struct {
	PrincipalID string
	ClientName  string
	Method      string
	Name        string
}

// ToolAdmissionRequest 为已完成 Schema 校验的 Tool 提供费用/大小策略所需的最小事实。
type ToolAdmissionRequest struct {
	AdmissionRequest
	Arguments any
}

type LocalAdmissionConfig struct {
	RequestRatePerSecond int
	RequestBurst         int
	ToolRatePerSecond    int
	ToolBurst            int
	MaxUploadBytes       int64
	MaxTrackedScopes     int
}

type admissionBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// LocalAdmissionController 提供单实例有界令牌桶和单次上传上限；共享费用/存储额度可通过 AdmissionController 替换。
type LocalAdmissionController struct {
	mu             sync.Mutex
	requestBuckets map[string]*admissionBucket
	toolBuckets    map[string]*admissionBucket
	config         LocalAdmissionConfig
}

func NewLocalAdmissionController(config LocalAdmissionConfig) (*LocalAdmissionController, error) {
	if config.RequestRatePerSecond < 1 || config.RequestBurst < 1 || config.ToolRatePerSecond < 1 ||
		config.ToolBurst < 1 || config.MaxUploadBytes < 1 || config.MaxTrackedScopes < 1 {
		return nil, fmt.Errorf("MCP local admission limits must be positive")
	}
	return &LocalAdmissionController{
		requestBuckets: make(map[string]*admissionBucket),
		toolBuckets:    make(map[string]*admissionBucket),
		config:         config,
	}, nil
}

func (a *LocalAdmissionController) AdmitRequest(_ context.Context, request AdmissionRequest) error {
	if a == nil || request.PrincipalID == "" {
		return errAdmissionRateLimited
	}
	if !a.allow(a.requestBuckets, request.PrincipalID, a.config.RequestRatePerSecond, a.config.RequestBurst) {
		return errAdmissionRateLimited
	}
	if request.Method == protocol.MethodToolsCall &&
		!a.allow(a.toolBuckets, request.PrincipalID+"\x00"+request.Name, a.config.ToolRatePerSecond, a.config.ToolBurst) {
		return errAdmissionRateLimited
	}
	return nil
}

func (a *LocalAdmissionController) AdmitTool(_ context.Context, request ToolAdmissionRequest) error {
	if a == nil {
		return errAdmissionRateLimited
	}
	arguments, ok := request.Arguments.(*protocol.AssetsPrepareUploadArguments)
	if ok && arguments.SizeBytes > a.config.MaxUploadBytes {
		return errAdmissionRateLimited
	}
	return nil
}

func (a *LocalAdmissionController) allow(
	buckets map[string]*admissionBucket,
	key string,
	requestsPerSecond, burst int,
) bool {
	now := time.Now()
	a.mu.Lock()
	bucket := buckets[key]
	if bucket == nil {
		if len(a.requestBuckets)+len(a.toolBuckets) >= a.config.MaxTrackedScopes {
			a.evictOldestLocked()
		}
		bucket = &admissionBucket{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst)}
		buckets[key] = bucket
	}
	bucket.lastSeen = now
	a.mu.Unlock()
	return bucket.limiter.AllowN(now, 1)
}

func (a *LocalAdmissionController) evictOldestLocked() {
	var oldestMap map[string]*admissionBucket
	oldestKey := ""
	oldestAt := time.Time{}
	for _, buckets := range []map[string]*admissionBucket{a.requestBuckets, a.toolBuckets} {
		for key, bucket := range buckets {
			if oldestKey == "" || bucket.lastSeen.Before(oldestAt) {
				oldestMap, oldestKey, oldestAt = buckets, key, bucket.lastSeen
			}
		}
	}
	if oldestMap != nil {
		delete(oldestMap, oldestKey)
	}
}

// AllowAllAdmissionController 是显式 noop 测试/迁移替身；生产 composition root 默认不注入它。
type AllowAllAdmissionController struct{}

func (AllowAllAdmissionController) AdmitRequest(context.Context, AdmissionRequest) error { return nil }
func (AllowAllAdmissionController) AdmitTool(context.Context, ToolAdmissionRequest) error { return nil }

var _ AdmissionController = (*LocalAdmissionController)(nil)
var _ AdmissionController = AllowAllAdmissionController{}

package iapiserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	// InfraRuntimeModeJob 表示执行完成后结束的 Runtime 模式。
	InfraRuntimeModeJob = "JOB"
	// InfraRuntimeModeService 表示持续提供服务的 Runtime 模式。
	InfraRuntimeModeService = "SERVICE"
)

const (
	// InfraNodeStatusOnline 表示 Infrastructure 节点当前可用。
	InfraNodeStatusOnline = "ONLINE"
	// InfraNodeStatusOffline 表示 Infrastructure 节点当前不可用。
	InfraNodeStatusOffline = "OFFLINE"
	// InfraRuntimeProfileStatusActive 表示 Runtime Profile 可被新请求选用。
	InfraRuntimeProfileStatusActive = "ACTIVE"
	// InfraRuntimeStatusAccepted 表示 Runtime 请求已接受但尚未准备完成。
	InfraRuntimeStatusAccepted = "ACCEPTED"
	// InfraRuntimeStatusPreparing 表示 Runtime 正在准备 Provider 资源。
	InfraRuntimeStatusPreparing = "PREPARING"
	// InfraRuntimeStatusRunning 表示 Runtime 正在运行。
	InfraRuntimeStatusRunning = "RUNNING"
	// InfraRuntimeStatusSucceeded 表示 Runtime 已成功完成。
	InfraRuntimeStatusSucceeded = "SUCCEEDED"
	// InfraRuntimeStatusFailed 表示 Runtime 执行失败。
	InfraRuntimeStatusFailed = "FAILED"
	// InfraRuntimeStatusDeleted 表示 Runtime 资源已删除。
	InfraRuntimeStatusDeleted = "DELETED"
	// InfraRuntimeStatusCanceled 表示 JOB Runtime 已取消。
	InfraRuntimeStatusCanceled = "CANCELED"
	// InfraRuntimeStatusStopped 表示 SERVICE Runtime 已停止。
	InfraRuntimeStatusStopped = "STOPPED"
	// InfraRuntimeEndpointStatusReady 表示 Endpoint 可以被解析使用。
	InfraRuntimeEndpointStatusReady = "READY"
	// InfraRuntimeConfigBindingStatusPending 表示配置绑定尚未完成注入。
	InfraRuntimeConfigBindingStatusPending = "PENDING"
	// InfraRuntimeOutputStatusPending 表示输出已声明但内容尚未收集。
	InfraRuntimeOutputStatusPending = "PENDING"
	// InfraRuntimeOutputStatusCollected 表示输出内容已经收集并可读取。
	InfraRuntimeOutputStatusCollected = "COLLECTED"
)

const (
	// InfraOperationCreate/Start/Stop/Cancel/Reconcile 是 Infrastructure 内部命令操作。
	InfraOperationCreate    = "create"
	InfraOperationStart     = "start"
	InfraOperationStop      = "stop"
	InfraOperationCancel    = "cancel"
	InfraOperationReconcile = "reconcile"

	// InfraEndpointVisibilityInternal 表示仅允许受控内部调用访问 Endpoint。
	InfraEndpointVisibilityInternal = "INTERNAL"
	// InfraEndpointVisibilityUserAccessible 表示 Endpoint 可返回给用户侧流程。
	InfraEndpointVisibilityUserAccessible = "USER_ACCESSIBLE"

	// Infrastructure 支持的受控挂载类型。
	InfraMountKindAgentWorkspace          = "AGENT_WORKSPACE"
	InfraMountKindStudioWorkspaceRevision = "STUDIO_WORKSPACE_REVISION"
	InfraMountKindStudioSnapshot          = "STUDIO_SNAPSHOT"
	InfraMountKindArtifact                = "ARTIFACT"
	InfraMountKindTemporary               = "TEMPORARY"

	// Infrastructure 支持的配置绑定类型。
	InfraConfigBindingTypePlainConfig    = "PLAIN_CONFIG"
	InfraConfigBindingTypeModelAccess    = "MODEL_ACCESS"
	InfraConfigBindingTypeSecretRef      = "SECRET_REF"
	InfraConfigBindingTypeIntegrationRef = "INTEGRATION_REF"

	// InfraResolveEndpointPurposeAgentRuntimeAdapter 是 Agent Runtime 解析 Endpoint 的用途。
	InfraResolveEndpointPurposeAgentRuntimeAdapter = "AGENT_RUNTIME_ADAPTER"

	// Infrastructure 受控引用前缀。
	InfraRefPrefixAgentWorkspace          = "agent-workspace://"
	InfraRefPrefixStudioWorkspaceRevision = "studio-workspace-revision://"
	InfraRefPrefixStudioSnapshot          = "studio-snapshot://"
	InfraRefPrefixArtifact                = "artifact://"
	InfraRefPrefixTemporary               = "temporary://"
	InfraRefPrefixEndpoint                = "infra-endpoint://"
	InfraRefPrefixOutput                  = "infra-output://"
	InfraEndpointDisplayRefPrefix         = "infra-runtime://"

	// InfraContentDigestSHA256Prefix 是内容指纹的固定编码前缀。
	InfraContentDigestSHA256Prefix = "sha256:"

	// Infrastructure Provider 和默认 Profile 使用的固定标识。
	InfraProviderTypeDocker                     = "docker"
	InfraNodeIDDockerLocal                      = "docker-local"
	InfraNodeNameDockerLocal                    = "Docker Local"
	InfraRuntimeProfileRevisionInitial          = "1.0"
	InfraRuntimeProfileIDAgentHermes            = "agent.hermes"
	InfraRuntimeProfileIDAgentCoding            = "agent.coding"
	InfraRuntimeProfileIDAppStudioPreviewWeb    = "appstudio.preview.static-web"
	InfraRuntimeProfileIDAppStudioPreviewAPI    = "appstudio.preview.web-backend"
	InfraRuntimeProfileIDAppStudioBuildWeb      = "appstudio.build.static-web"
	InfraRuntimeProfileIDAppStudioBuildAPI      = "appstudio.build.web-backend"
	InfraRuntimeProfileIDAppStudioProductionWeb = "appstudio.production.static-web"
	InfraRuntimeProfileIDAppStudioProductionAPI = "appstudio.production.web-backend"
	InfraRuntimeCapabilityCPU                   = "cpu"
	InfraRuntimeCapabilityNetwork               = "network"
	InfraRuntimeCapabilityPersistentWorkspace   = "persistent_workspace"
	InfraRuntimeCapabilityWorkspaceTool         = "workspace_tool"
	InfraRuntimeCapabilityEndpoint              = "endpoint"
	InfraRuntimeCapabilityArtifactOutput        = "artifact_output"

	// Infrastructure endpoint 解析和内容响应使用的协议标识。
	InfraProtocolHTTP                 = "http"
	InfraProtocolHTTPS                = "https"
	InfraHeaderContentDigest          = "X-Content-Digest"
	InfraRuntimeLogLevelInfo          = "INFO"
	InfraRuntimeLogSourceDocker       = "docker"
	InfraRuntimeEventReasonReconciled = "infra_runtime_reconciled"
)

// +k8s:deepcopy-gen=true
type InfraRuntimeMountInput struct {
	SourceRef        string `json:"source_ref" binding:"required,max=2048"`
	TargetPath       string `json:"target_path" binding:"required,max=512"`
	ReadOnly         bool   `json:"read_only"`
	MountKind        string `json:"mount_kind" binding:"required,oneof=AGENT_WORKSPACE STUDIO_WORKSPACE_REVISION STUDIO_SNAPSHOT ARTIFACT TEMPORARY"`
	AuthorizationRef string `json:"-"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeConfigBindingInput struct {
	Name        string `json:"name" binding:"required,max=200"`
	BindingType string `json:"binding_type" binding:"required,oneof=PLAIN_CONFIG MODEL_ACCESS SECRET_REF INTEGRATION_REF"`
	Reference   string `json:"reference" binding:"required,max=2048"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeEndpointRequest struct {
	// EndpointName 是固定 RuntimeProfile Revision 声明的受控 Endpoint 名称。
	EndpointName string `json:"endpoint_name" binding:"required,max=128"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeOutputDeclaration struct {
	// OutputKey 是函数合同内稳定的输出名称，只允许小写字母、数字和下划线。
	OutputKey string `json:"output_key" binding:"required,max=128"`
	// RelativePath 是相对 RuntimeProfile 输出根的普通文件路径，不得逃逸输出目录。
	RelativePath string `json:"relative_path" binding:"required,max=512"`
	// MediaType 是收集后内容响应和完整性校验使用的 MIME 类型。
	MediaType string `json:"media_type" binding:"required,max=255"`
}

// +k8s:deepcopy-gen=true
type InfraResourceRequirement struct {
	CPUCores    float64 `json:"cpu_cores,omitempty" binding:"min=0"`
	MemoryMB    int64   `json:"memory_mb,omitempty" binding:"min=0"`
	DiskMB      int64   `json:"disk_mb,omitempty" binding:"min=0"`
	GPUCount    int     `json:"gpu_count,omitempty" binding:"min=0"`
	GPUMemoryMB int64   `json:"gpu_memory_mb,omitempty" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type InfraTimeoutPolicy struct {
	StartupTimeoutSeconds   int              `json:"startup_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	ExecutionTimeoutSeconds int              `json:"execution_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	IdleTimeoutSeconds      int              `json:"idle_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	MaximumLifetimeSeconds  int              `json:"maximum_lifetime_seconds,omitempty" binding:"omitempty,min=1"`
	ExpiresAt               *imachinery.Time `json:"expires_at,omitempty"`
}

// +k8s:deepcopy-gen=true
type InfraCreateRuntimeRequest struct {
	// RequestID 在 requesting_service 范围内提供幂等身份。
	RequestID string `json:"request_id" binding:"required,max=200"`
	// RequestingService 固定为受信 Task Center 服务身份。
	RequestingService string `json:"requesting_service" binding:"required,oneof=task-center"`
	// OwnerDomain 标识 Runtime 事实归属的业务域。
	OwnerDomain string `json:"owner_domain" binding:"required,oneof=agent appstudio task-center asset-library"`
	// OwnerReference 标识来源域拥有的稳定业务对象。
	OwnerReference string `json:"owner_reference" binding:"required,max=512"`
	// RequestUserID 记录发起执行的用户，不用于服务鉴权。
	RequestUserID string `json:"request_user_id,omitempty" binding:"omitempty,max=128"`
	// RuntimeMode 限定为 JOB 或 SERVICE，并必须匹配固定 Profile。
	RuntimeMode string `json:"runtime_mode" binding:"required,oneof=JOB SERVICE"`
	// RuntimeProfileID 选择受控运行配置，不接受任意 Provider 参数。
	RuntimeProfileID string `json:"runtime_profile_id" binding:"required,max=200"`
	// RuntimeProfileRevision 固定本次执行使用的 Profile Revision。
	RuntimeProfileRevision string `json:"runtime_profile_revision" binding:"required,max=64"`
	// SourceRef 是来源域生成的受控引用，不得是宿主机路径。
	SourceRef string `json:"source_ref,omitempty" binding:"omitempty,max=2048"`
	// Mounts 声明只读或临时受控挂载，不允许传入任意宿主机目录。
	Mounts []InfraRuntimeMountInput `json:"mounts,omitempty" binding:"omitempty,max=20,dive"`
	// ConfigurationBindings 仅传递受控配置引用，Secret 正文不得进入请求。
	ConfigurationBindings []InfraRuntimeConfigBindingInput `json:"configuration_bindings,omitempty" binding:"omitempty,max=50,dive"`
	// EndpointRequest 仅用于 SERVICE，名称必须来自固定 Profile Revision。
	EndpointRequest *InfraRuntimeEndpointRequest `json:"endpoint_request,omitempty"`
	// OutputDeclarations 仅用于 JOB，并必须与函数合同声明完全一致。
	OutputDeclarations []InfraRuntimeOutputDeclaration `json:"output_declarations,omitempty" binding:"omitempty,max=20,dive"`
	// ResourceRequirement 是受控资源需求，不暴露 Provider 原生配置。
	ResourceRequirement InfraResourceRequirement `json:"resource_requirement,omitempty"`
	// TimeoutPolicy 固定启动、执行、空闲和最大生存期边界。
	TimeoutPolicy      InfraTimeoutPolicy `json:"timeout_policy,omitempty"`
	AuthorizationRef   string             `json:"-"`
	EndpointVisibility string             `json:"-"`
	FunctionRef        string             `json:"-"`
	FunctionArguments  json.RawMessage    `json:"-"`
}

func (r *InfraCreateRuntimeRequest) Validate() error {
	if r.EndpointRequest != nil && r.RuntimeMode != InfraRuntimeModeService {
		return fmt.Errorf("endpoint request is only supported for service runtimes")
	}
	if len(r.OutputDeclarations) > 0 && r.RuntimeMode != InfraRuntimeModeJob {
		return fmt.Errorf("output declarations are only supported for job runtimes")
	}
	outputKeys := make(map[string]struct{}, len(r.OutputDeclarations))
	for _, mount := range r.Mounts {
		if !strings.HasPrefix(mount.SourceRef, InfraRefPrefixAgentWorkspace) && !strings.HasPrefix(mount.SourceRef, InfraRefPrefixStudioWorkspaceRevision) && !strings.HasPrefix(mount.SourceRef, InfraRefPrefixStudioSnapshot) && !strings.HasPrefix(mount.SourceRef, InfraRefPrefixArtifact) && !strings.HasPrefix(mount.SourceRef, InfraRefPrefixTemporary) {
			return fmt.Errorf("unsupported mount source reference")
		}
		if !strings.HasPrefix(mount.TargetPath, "/") || strings.Contains(mount.TargetPath, "..") {
			return fmt.Errorf("invalid mount target")
		}
	}
	for _, output := range r.OutputDeclarations {
		if _, exists := outputKeys[output.OutputKey]; exists {
			return fmt.Errorf("duplicate output key")
		}
		outputKeys[output.OutputKey] = struct{}{}
		for index, char := range output.OutputKey {
			if index == 0 && (char < 'a' || char > 'z') {
				return fmt.Errorf("invalid output key")
			}
			if index > 0 && (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
				return fmt.Errorf("invalid output key")
			}
		}
		if strings.HasPrefix(output.RelativePath, "/") || strings.Contains(output.RelativePath, "..") {
			return fmt.Errorf("invalid output relative path")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type InfraResolveEndpointRequest struct {
	// OwnerReference 必须与 Endpoint 所属 Runtime 的来源对象一致。
	OwnerReference string `json:"owner_reference" binding:"required,max=512"`
	// Purpose 固定为 AgentRuntimeAdapter 的同步内部调用。
	Purpose string `json:"purpose" binding:"required,oneof=AGENT_RUNTIME_ADAPTER"`
	// AuditCorrelationID 是 Invocation、trace 或等价的稳定审计关联 ID。
	AuditCorrelationID string `json:"audit_correlation_id" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type InfraAttachArtifactRequest struct {
	// ArtifactID 是 Asset Library 已完成内容的稳定 Artifact ID。
	ArtifactID string `json:"artifact_id" binding:"required,max=128"`
	// SizeBytes 必须与 RuntimeOutput descriptor 和 Artifact 内容一致。
	SizeBytes int64 `json:"size_bytes" binding:"min=0"`
	// ContentDigest 必须是实际内容的 lowercase SHA-256 digest。
	ContentDigest string `json:"content_digest" binding:"required,len=71"`
}

// Validate 校验 Artifact 回链使用的精确 lowercase SHA-256 格式。
func (r *InfraAttachArtifactRequest) Validate() error {
	if !validInfraContentDigest(r.ContentDigest) {
		return fmt.Errorf("invalid content digest")
	}
	return nil
}

// +k8s:deepcopy-gen=true
type InfraRuntimeListRequest struct {
	imachinery.BasicQueryParam
	Status         string `form:"status" binding:"omitempty,max=256"`
	OwnerDomain    string `form:"owner_domain" binding:"omitempty,oneof=agent appstudio task-center asset-library"`
	OwnerReference string `form:"owner_reference" binding:"omitempty,max=512"`
}

// Validate 校验 Infrastructure Runtime 列表的独立分页上限。
func (r *InfraRuntimeListRequest) Validate() error {
	return normalizeInfraPaging(&r.PagingParams)
}

// +k8s:deepcopy-gen=true
type InfraActionRequest struct {
	Reason          string `json:"reason,omitempty" binding:"omitempty,max=1000"`
	RequestID       string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	ResourceVersion int64  `json:"resource_version" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type InfraBasicListRequest struct {
	imachinery.BasicQueryParam
	Status string `form:"status" binding:"omitempty,max=256"`
}

// Validate 校验 Infrastructure 基础列表的独立分页上限。
func (r *InfraBasicListRequest) Validate() error {
	return normalizeInfraPaging(&r.PagingParams)
}

func normalizeInfraPaging(params *imachinery.PagingParams) error {
	if params.PageNum < 0 {
		return fmt.Errorf("page_num must be greater than or equal to zero")
	}
	if params.PageSize < 0 {
		return fmt.Errorf("page_size must be greater than or equal to zero")
	}
	if params.PageSize == 0 {
		params.PageSize = 50
	}
	if params.PageSize > 200 {
		return fmt.Errorf("page_size must be less than or equal to 200")
	}
	_, err := params.Normalize()
	return err
}

func validInfraContentDigest(value string) bool {
	if len(value) != len(InfraContentDigestSHA256Prefix)+64 || !strings.HasPrefix(value, InfraContentDigestSHA256Prefix) {
		return false
	}
	for _, char := range value[len(InfraContentDigestSHA256Prefix):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

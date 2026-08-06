package contracts

import (
	"context"
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
)

// InfrastructureCommandExecutor 定义 Task Worker 调用 Infrastructure runtime 命令所需的最小接口。
type InfrastructureCommandExecutor interface {
	Execute(context.Context, *infrastructure.CommandRequest) (*infrastructure.CommandResponse, error)
}

// InfrastructureBuildExecutor 定义 AppStudio 构建执行所需的 Infrastructure 命令和输出交付接口。
type InfrastructureBuildExecutor interface {
	InfrastructureCommandExecutor
	ReadOutputContent(context.Context, string) (*infrastructure.OutputContent, error)
	AttachOutputArtifact(context.Context, string, *iapiserver.InfraAttachArtifactRequest) (*iapiserver.InfraRuntimeOutput, error)
}

// RequireInfrastructureRuntime validates the runtime portion of an Infrastructure command response.
func RequireInfrastructureRuntime(response *infrastructure.CommandResponse) (*iapiserver.InfraRuntime, error) {
	if response == nil {
		return nil, fmt.Errorf("infrastructure command response is missing")
	}
	if response.Result == nil {
		return nil, fmt.Errorf("infrastructure command result is missing")
	}
	if response.Result.Runtime == nil || response.Result.Runtime.ID == "" {
		return nil, fmt.Errorf("infrastructure runtime result is missing")
	}
	return response.Result.Runtime, nil
}

// HasReadyInfrastructureEndpoint verifies the endpoint returned with a ready runtime.
func HasReadyInfrastructureEndpoint(runtime *iapiserver.InfraRuntime, endpoint *iapiserver.InfraRuntimeEndpoint) bool {
	return runtime != nil && endpoint != nil && endpoint.ID != "" && endpoint.Status == iapiserver.TaskWorkerRuntimeEndpointStatusReady &&
		runtime.EndpointRef == "infra-endpoint://"+endpoint.ID
}

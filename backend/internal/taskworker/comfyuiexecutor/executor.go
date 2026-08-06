package comfyuiexecutor

import (
	"context"
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

const (
	submitFunctionRef         = "comfyui.submit"
	pollFunctionRef           = "comfyui.poll"
	collectPreviewFunctionRef = "comfyui.collect_preview"
)

// TestExecutor 定义 ComfyUI 测试运行生命周期所需的提交、轮询和收集接口。
type TestExecutor interface {
	Submit(context.Context, string) (map[string]any, error)
	Poll(context.Context, string) (map[string]any, error)
	Collect(context.Context, string) (map[string]any, error)
}

// HandlerRegistrar 定义 ComfyUI 执行器注册 Worker handler 所需的最小运行时接口。
type HandlerRegistrar interface {
	RegisterHandler(string, int, workflowruntime.Handler) error
}

// RegisterHandlers registers the ComfyUI submit, poll, and preview collection handlers.
func RegisterHandlers(runtime HandlerRegistrar, executor TestExecutor) error {
	if runtime == nil {
		return fmt.Errorf("workflow runtime is required")
	}
	if executor == nil {
		return fmt.Errorf("comfyui test executor is required")
	}
	if err := runtime.RegisterHandler(submitFunctionRef, 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := executor.Submit(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("comfyui.submit.ready", workflowruntime.TaskLogLevelInfo, "External job is ready for polling."))
		}
		return output, err
	}); err != nil {
		return errors.Wrap(err, "register comfyui submit handler")
	}
	if err := runtime.RegisterHandler(pollFunctionRef, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := executor.Poll(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			if waiting, _ := output["in_progress"].(bool); waiting {
				key, message := "comfyui.poll.running", "External job is still running."
				if output["queue_position"] != nil {
					key, message = "comfyui.poll.queued", "External job is queued."
				}
				task.Log(ctx, workflowruntime.WorkerLog(key, workflowruntime.TaskLogLevelInfo, message))
			} else {
				task.Log(ctx, workflowruntime.WorkerLog("comfyui.poll.completed", workflowruntime.TaskLogLevelInfo, "External job completed."))
			}
		}
		return output, err
	}); err != nil {
		return errors.Wrap(err, "register comfyui poll handler")
	}
	if err := runtime.RegisterHandler(collectPreviewFunctionRef, 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := executor.Collect(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("comfyui.preview.collected", workflowruntime.TaskLogLevelInfo, fmt.Sprintf("Collected %v preview outputs.", output["output_count"])))
		}
		return output, err
	}); err != nil {
		return errors.Wrap(err, "register comfyui collect preview handler")
	}
	return nil
}

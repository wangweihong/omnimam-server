package gitlabexecutor

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	gitlabsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/gitlab"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const callbackAfterSeconds = 5

// Executor 执行一个可恢复 GitLab Pipeline Attempt，不拥有 AtomicTask 或 GitLab 投影状态。
type Executor struct {
	store   store.GitLabStore
	clients gitlabsvc.ClientFactory
}

func New(store store.GitLabStore, clients gitlabsvc.ClientFactory) (*Executor, error) {
	if store == nil || clients == nil {
		return nil, fmt.Errorf("gitlab store and client factory are required")
	}
	return &Executor{store: store, clients: clients}, nil
}

func (e *Executor) Execute(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	if atomicTask == nil || atomicTask.FunctionRef != iapiserver.GitLabFunctionPipelineRun {
		return nil, errors.NewStatus(code.ErrGitLabPipelineInvalid, "gitlab pipeline atomic task is invalid")
	}
	arguments, err := decodeArguments(task.Arguments)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabPipelineInvalid, err.Error())
	}
	project, err := e.store.GetGitLabProject(ctx, arguments.GitLabProjectID)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NewStatus(code.ErrGitLabProjectNotFound, "gitlab project is not visible")
	}
	if err != nil {
		return nil, err
	}
	server, err := e.store.GetGitLabServer(ctx, project.GitLabServerID)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NewStatus(code.ErrGitLabServerNotFound, "gitlab server is not visible")
	}
	if err != nil {
		return nil, err
	}
	client, err := e.clients.NewClient(server)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabPipelineCreateFailed, "gitlab client is unavailable")
	}
	checkpoint, err := task.LoadCheckpoint(ctx)
	if err != nil {
		return nil, err
	}
	pipelineID, err := checkpointPipelineID(checkpoint)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabPipelineInvalid, err.Error())
	}
	if pipelineID == 0 {
		pipelineID, err = checkpointPipelineID(atomicTask.Output)
		if err != nil {
			return nil, errors.NewStatus(code.ErrGitLabPipelineInvalid, err.Error())
		}
	}
	if pipelineID == 0 {
		pipeline, createErr := client.CreatePipeline(ctx, project.ExternalProjectID, gitlabsvc.CreatePipelineRequest{Ref: arguments.Ref, Variables: arguments.Variables})
		if createErr != nil {
			return nil, errors.NewStatus(code.ErrGitLabPipelineCreateFailed, "gitlab pipeline creation failed")
		}
		return pipelineResult(project.ID, pipeline)
	}
	if err := ctx.Err(); err != nil {
		e.cancelPipeline(ctx, project.ExternalProjectID, pipelineID, client)
		return nil, err
	}
	pipeline, err := client.GetPipeline(ctx, project.ExternalProjectID, pipelineID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabPipelineCreateFailed, "gitlab pipeline status is unavailable")
	}
	return pipelineResult(project.ID, pipeline)
}

func pipelineResult(projectID string, pipeline *gitlabsvc.Pipeline) (map[string]any, error) {
	switch strings.ToLower(pipeline.Status) {
	case "success":
		return pipelineOutput(projectID, pipeline, false), nil
	case "failed", "skipped", "manual":
		return nil, errors.NewStatus(code.ErrGitLabPipelineFailed, "gitlab pipeline failed")
	case "canceled":
		return pipelineOutput(projectID, pipeline, false), fmt.Errorf("%w: gitlab pipeline was canceled", workflowruntime.ErrWorkerTaskCanceled)
	default:
		return pipelineOutput(projectID, pipeline, true), nil
	}
}

func (e *Executor) cancelPipeline(ctx context.Context, projectID, pipelineID int64, client gitlabsvc.Client) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _ = client.CancelPipeline(cleanupCtx, projectID, pipelineID)
}

func decodeArguments(input map[string]any) (*iapiserver.GitLabPipelineRunTaskArguments, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("gitlab pipeline arguments cannot be encoded")
	}
	var arguments iapiserver.GitLabPipelineRunTaskArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, fmt.Errorf("gitlab pipeline arguments cannot be decoded")
	}
	arguments.GitLabProjectID = strings.TrimSpace(arguments.GitLabProjectID)
	arguments.Ref = strings.TrimSpace(arguments.Ref)
	if arguments.GitLabProjectID == "" || arguments.Ref == "" || len(arguments.Ref) > 255 || len(arguments.Variables) > 100 {
		return nil, fmt.Errorf("gitlab_project_id, ref, or variables are invalid")
	}
	return &arguments, nil
}

func checkpointPipelineID(checkpoint map[string]any) (int64, error) {
	value := checkpoint[iapiserver.TaskWorkerKeyExternalJobID]
	if value == nil || fmt.Sprint(value) == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("external_job_id is invalid")
	}
	return id, nil
}

func pipelineOutput(projectID string, pipeline *gitlabsvc.Pipeline, inProgress bool) map[string]any {
	output := map[string]any{
		"gitlab_project_id": projectID,
		"pipeline_id":       pipeline.ID,
		"pipeline_status":   pipeline.Status,
		"web_url":           pipeline.WebURL,
		"external_job_id":   strconv.FormatInt(pipeline.ID, 10),
	}
	if inProgress {
		output[iapiserver.TaskWorkerKeyInProgress] = true
		output[iapiserver.TaskWorkerKeyCallbackAfterSeconds] = callbackAfterSeconds
	}
	return output
}

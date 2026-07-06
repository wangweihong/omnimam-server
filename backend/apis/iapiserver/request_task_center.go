package iapiserver

import (
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type (
	TaskDefinitionListRequest struct {
		imachinery.BasicQueryParam
		DefinitionType string `json:"definition_type" form:"definition_type" binding:"omitempty,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		ProjectID      string `json:"project_id"      form:"project_id"`
		Namespace      string `json:"namespace"       form:"namespace"`
	}

	TaskDefinitionListResponse struct {
		Total int64             `json:"total"`
		Items []*TaskDefinition `json:"items"`
	}

	AtomicTaskCreateRequest struct {
		Name                 string         `json:"name"                  binding:"required"`
		Description          string         `json:"description"`
		FunctionRef          string         `json:"function_ref"          binding:"required"`
		AppID                string         `json:"app_id"`
		EngineRef            string         `json:"engine_ref"`
		DefaultArguments     map[string]any `json:"default_arguments"`
		TimeoutPolicy        TimeoutPolicy  `json:"timeout_policy"`
		RetryPolicy          RetryPolicy    `json:"retry_policy"`
		CancelPolicy         map[string]any `json:"cancel_policy"`
		RequiredCapabilities string         `json:"required_capabilities"`
		Tags                 string         `json:"tags"`
		ProjectID            string         `json:"project_id"`
		Namespace            string         `json:"namespace"`
		CreatedBy            string         `json:"created_by"`
	}

	TaskGroupCreateRequest struct {
		Name           string                `json:"name"       binding:"required"`
		Description    string                `json:"description"`
		GroupType      string                `json:"group_type" binding:"required,oneof=SERIAL PARALLEL"`
		Children       []TaskDefinitionChild `json:"children"   binding:"required"`
		StrategyConfig StrategyConfig        `json:"strategy_config"`
		TimeoutPolicy  TimeoutPolicy         `json:"timeout_policy"`
		RetryPolicy    RetryPolicy           `json:"retry_policy"`
		Tags           string                `json:"tags"`
		ProjectID      string                `json:"project_id"`
		Namespace      string                `json:"namespace"`
		CreatedBy      string                `json:"created_by"`
	}

	DAGFlowTaskCreateRequest struct {
		Name           string         `json:"name"  binding:"required"`
		Description    string         `json:"description"`
		Nodes          []DAGNode      `json:"nodes" binding:"required"`
		Edges          []DAGEdge      `json:"edges"`
		InputMapping   map[string]any `json:"input_mapping"`
		OutputMapping  map[string]any `json:"output_mapping"`
		StrategyConfig StrategyConfig `json:"strategy_config"`
		TimeoutPolicy  TimeoutPolicy  `json:"timeout_policy"`
		RetryPolicy    RetryPolicy    `json:"retry_policy"`
		Tags           string         `json:"tags"`
		ProjectID      string         `json:"project_id"`
		Namespace      string         `json:"namespace"`
		CreatedBy      string         `json:"created_by"`
	}

	TaskRunListRequest struct {
		imachinery.BasicQueryParam
		Status         string `json:"status"          form:"status"          binding:"omitempty,oneof=PENDING READY CLAIMED RUNNING RETRYING CANCEL_REQUESTED PAUSED SUCCESS FAILED CANCELED TIMEOUT LOST"`
		DefinitionType string `json:"definition_type" form:"definition_type" binding:"omitempty,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		RootRunID      string `json:"root_run_id"     form:"root_run_id"`
		ProjectID      string `json:"project_id"      form:"project_id"`
	}

	TaskRunCreateRequest struct {
		DefinitionType string          `json:"definition_type" binding:"required,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		DefinitionID   string          `json:"definition_id"   binding:"required"`
		ParentRunID    string          `json:"parent_run_id"`
		RootRunID      string          `json:"root_run_id"`
		ScheduleAt     imachinery.Time `json:"schedule_at"`
		Input          map[string]any  `json:"input"`
		TimeoutPolicy  TimeoutPolicy   `json:"timeout_policy"`
		RetryPolicy    RetryPolicy     `json:"retry_policy"`
		ProjectID      string          `json:"project_id"`
		Namespace      string          `json:"namespace"`
		Tags           string          `json:"tags"`
		CreatedBy      string          `json:"created_by"`
	}

	TaskRunListResponse struct {
		Total int64      `json:"total"`
		Items []*TaskRun `json:"items"`
	}

	TaskAttemptListRequest struct {
		imachinery.BasicQueryParam
		RunID string `json:"run_id" form:"run_id"`
	}

	TaskAttemptListResponse struct {
		Total int64          `json:"total"`
		Items []*TaskAttempt `json:"items"`
	}

	CancelTaskRunRequest struct {
		RunID  string `json:"-"      form:"-"`
		Reason string `json:"reason"`
	}

	RetryTaskRunRequest struct {
		RunID     string `json:"-"          form:"-"`
		Reason    string `json:"reason"`
		RetryMode string `json:"retry_mode"`
	}

	WorkerRegisterRequest struct {
		WorkerType     string `json:"worker_type"     binding:"required"`
		Capabilities   string `json:"capabilities"    binding:"required"`
		Labels         string `json:"labels"`
		MaxConcurrency int    `json:"max_concurrency" binding:"required,min=1"`
	}

	WorkerHeartbeatRequest struct {
		WorkerID     string          `json:"-"             form:"-"`
		Status       string          `json:"status"        binding:"omitempty,oneof=ONLINE BUSY DRAINING OFFLINE LOST DISABLED"`
		RunningCount int             `json:"running_count"`
		ObservedAt   imachinery.Time `json:"observed_at"`
	}

	ClaimTaskRunRequest struct {
		WorkerID     string `json:"-"            form:"-"`
		Capabilities string `json:"capabilities" binding:"required"`
		Labels       string `json:"labels"`
		MaxCount     int    `json:"max_count"    binding:"required,min=1"`
	}

	ClaimTaskRunResponse struct {
		TaskRun *TaskRun        `json:"task_run,omitempty"`
		Attempt *TaskAttempt    `json:"attempt,omitempty"`
		Lease   *ExecutionLease `json:"lease,omitempty"`
	}

	ProgressUpdateRequest struct {
		RunID          string         `json:"-"           form:"-"`
		WorkerID       string         `json:"worker_id"   binding:"required"`
		AttemptID      string         `json:"attempt_id"  binding:"required"`
		LeaseID        string         `json:"lease_id"    binding:"required"`
		Progress       float64        `json:"progress"    binding:"min=0,max=1"`
		OutputSnapshot map[string]any `json:"output_snapshot"`
		ExternalJobID  string         `json:"external_job_id"`
	}

	TaskRunCompleteRequest struct {
		RunID         string         `json:"-"          form:"-"`
		WorkerID      string         `json:"worker_id"  binding:"required"`
		AttemptID     string         `json:"attempt_id" binding:"required"`
		LeaseID       string         `json:"lease_id"   binding:"required"`
		Output        map[string]any `json:"output"     binding:"required"`
		ExternalJobID string         `json:"external_job_id"`
	}

	TaskRunFailRequest struct {
		RunID         string    `json:"-"          form:"-"`
		WorkerID      string    `json:"worker_id"  binding:"required"`
		AttemptID     string    `json:"attempt_id" binding:"required"`
		LeaseID       string    `json:"lease_id"   binding:"required"`
		Error         TaskError `json:"error"      binding:"required"`
		LogsRef       string    `json:"logs_ref"`
		ExternalJobID string    `json:"external_job_id"`
	}

	LeaseRenewRequest struct {
		LeaseID   string `json:"-"          form:"-"`
		WorkerID  string `json:"worker_id"  binding:"required"`
		AttemptID string `json:"attempt_id" binding:"required"`
		RunID     string `json:"run_id"     binding:"required"`
	}

	TaskCenterHealth struct {
		SchedulerStatus   string `json:"scheduler_status"`
		WorkerOnlineTotal int64  `json:"worker_online_total"`
		WorkerLostTotal   int64  `json:"worker_lost_total"`
		QueueReadyTotal   int64  `json:"queue_ready_total"`
		TaskRunningTotal  int64  `json:"task_running_total"`
		TaskRetryingTotal int64  `json:"task_retrying_total"`
		LeaseExpiredTotal int64  `json:"lease_expired_total"`
		WatchdogStatus    string `json:"watchdog_status"`
	}

	SuccessResponse struct {
		Success bool `json:"success"`
	}
)

func (r *TaskDefinitionListRequest) PostBind() error {
	r.DefinitionType = strings.ToUpper(strings.TrimSpace(r.DefinitionType))
	return nil
}

func (r *TaskRunListRequest) PostBind() error {
	r.Status = strings.ToUpper(strings.TrimSpace(r.Status))
	r.DefinitionType = strings.ToUpper(strings.TrimSpace(r.DefinitionType))
	return nil
}

package postgresql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type taskCenterStore struct{ ds *datastore }

func newTaskCenterStore(ds *datastore) *taskCenterStore { return &taskCenterStore{ds: ds} }

func (s *taskCenterStore) ListAtomicTasks(ctx context.Context, req *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error) {
	var items []*iapiserver.AtomicTask
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("deleted_at IS NULL AND project_id = ? AND namespace = ? AND created_by = ?", req.ProjectID, req.Namespace, req.CreatedBy)
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		if req.RootTaskID != "" {
			query = query.Where("root_task_id = ?", req.RootTaskID)
		}
		if req.OwnerID != "" {
			query = query.Where("owner_id = ?", req.OwnerID)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AtomicTask{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetAtomicTask(ctx context.Context, id string) (*iapiserver.AtomicTask, error) {
	var item iapiserver.AtomicTask
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAtomicTaskNotFound, "atomic task not found")
	}
	return &item, nil
}

func (s *taskCenterStore) AddAtomicTaskIdempotent(ctx context.Context, data *iapiserver.AtomicTask) (*iapiserver.AtomicTask, bool, error) {
	if data.IdempotencyScope == "" || data.IdempotencyKey == "" {
		if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
			return nil, false, errors.WithStack(err)
		}
		return data, true, nil
	}
	var result *iapiserver.AtomicTask
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.AtomicTask
		err := tx.Where("project_id = ? AND namespace = ? AND idempotency_scope = ? AND idempotency_key = ?", data.ProjectID, data.Namespace, data.IdempotencyScope, data.IdempotencyKey).First(&existing).Error
		if err == nil {
			if atomicTaskFingerprint(&existing) != atomicTaskFingerprint(data) {
				return errors.NewStatusF(code.ErrAtomicTaskIdempotencyConflict, "atomic task idempotency request differs")
			}
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return errors.WithStack(err)
		}
		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		result, created = data, true
		return nil
	})
	return result, created, err
}

func (s *taskCenterStore) UpdateAtomicTask(ctx context.Context, data *iapiserver.AtomicTask) (*iapiserver.AtomicTask, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) ([]*iapiserver.TaskAttempt, int64, error) {
	var items []*iapiserver.TaskAttempt
	filter := func(query *gorm.DB) *gorm.DB { return query.Where("atomic_task_id = ?", req.AtomicTaskID) }
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskAttempt{}), filter).Order("attempt_no ASC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) ListTaskGroups(ctx context.Context, req *iapiserver.TaskGroupListRequest) ([]*iapiserver.TaskGroup, int64, error) {
	var items []*iapiserver.TaskGroup
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("deleted_at IS NULL AND project_id = ? AND namespace = ? AND created_by = ?", req.ProjectID, req.Namespace, req.CreatedBy)
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskGroup{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetTaskGroup(ctx context.Context, id string) (*iapiserver.TaskGroup, error) {
	var item iapiserver.TaskGroup
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrTaskGroupNotFound, "task group not found")
	}
	return &item, nil
}

func (s *taskCenterStore) AddTaskGroupWithTasks(ctx context.Context, group *iapiserver.TaskGroup, tasks []*iapiserver.AtomicTask) (*iapiserver.TaskGroup, bool, error) {
	if group.ID == "" {
		group.ID = uuid.NewString()
	}
	returnGroup := group
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if group.IdempotencyScope != "" && group.IdempotencyKey != "" {
			var existing iapiserver.TaskGroup
			err := tx.Where("project_id = ? AND namespace = ? AND idempotency_scope = ? AND idempotency_key = ?", group.ProjectID, group.Namespace, group.IdempotencyScope, group.IdempotencyKey).First(&existing).Error
			if err == nil {
				if taskGroupFingerprint(&existing) != taskGroupFingerprint(group) {
					return errors.NewStatusF(code.ErrAtomicTaskIdempotencyConflict, "task group idempotency request differs")
				}
				returnGroup = &existing
				return nil
			}
			if !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(err)
			}
		}
		if err := tx.Create(group).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, task := range tasks {
			task.OwnerType = iapiserver.TaskOwnerTypeGroup
			task.OwnerID = group.ID
			if err := tx.Create(task).Error; err != nil {
				return errors.WithStack(err)
			}
		}
		created = true
		return nil
	})
	return returnGroup, created, err
}

func (s *taskCenterStore) UpdateTaskGroup(ctx context.Context, data *iapiserver.TaskGroup) (*iapiserver.TaskGroup, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListDAGTaskGroups(ctx context.Context, req *iapiserver.DAGTaskGroupListRequest) ([]*iapiserver.DAGTaskGroup, int64, error) {
	var items []*iapiserver.DAGTaskGroup
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("deleted_at IS NULL AND project_id = ? AND namespace = ? AND created_by = ?", req.ProjectID, req.Namespace, req.CreatedBy)
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.DAGTaskGroup{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetDAGTaskGroup(ctx context.Context, id string) (*iapiserver.DAGTaskGroup, error) {
	var item iapiserver.DAGTaskGroup
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrDAGTaskGroupNotFound, "dag task group not found")
	}
	return &item, nil
}

func (s *taskCenterStore) AddDAGTaskGroupWithTasks(ctx context.Context, group *iapiserver.DAGTaskGroup, tasks []*iapiserver.AtomicTask) (*iapiserver.DAGTaskGroup, bool, error) {
	if group.ID == "" {
		group.ID = uuid.NewString()
	}
	returnGroup := group
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if group.IdempotencyScope != "" && group.IdempotencyKey != "" {
			var existing iapiserver.DAGTaskGroup
			err := tx.Where("project_id = ? AND namespace = ? AND idempotency_scope = ? AND idempotency_key = ?", group.ProjectID, group.Namespace, group.IdempotencyScope, group.IdempotencyKey).First(&existing).Error
			if err == nil {
				if dagTaskGroupFingerprint(&existing) != dagTaskGroupFingerprint(group) {
					return errors.NewStatusF(code.ErrAtomicTaskIdempotencyConflict, "dag task group idempotency request differs")
				}
				returnGroup = &existing
				return nil
			}
			if !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(err)
			}
		}
		if err := tx.Create(group).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, task := range tasks {
			task.OwnerType = iapiserver.TaskOwnerTypeDAGGroup
			task.OwnerID = group.ID
			if err := tx.Create(task).Error; err != nil {
				return errors.WithStack(err)
			}
		}
		created = true
		return nil
	})
	return returnGroup, created, err
}

func (s *taskCenterStore) UpdateDAGTaskGroup(ctx context.Context, data *iapiserver.DAGTaskGroup) (*iapiserver.DAGTaskGroup, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListOwnedTasks(ctx context.Context, ownerType, ownerID string, req *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error) {
	req.OwnerID = ownerID
	var items []*iapiserver.AtomicTask
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("owner_type = ? AND owner_id = ? AND deleted_at IS NULL", ownerType, ownerID)
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AtomicTask{}), filter).Order("child_order ASC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) AddOwnedAtomicTasks(ctx context.Context, ownerType, ownerID string, tasks []*iapiserver.AtomicTask) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			var count int64
			if err := tx.Model(&iapiserver.AtomicTask{}).Where("owner_type = ? AND owner_id = ? AND child_key = ?", ownerType, ownerID, task.ChildKey).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			task.OwnerType = ownerType
			task.OwnerID = ownerID
			if err := tx.Create(task).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (s *taskCenterStore) ListTaskSchedules(ctx context.Context, req *iapiserver.TaskScheduleListRequest) ([]*iapiserver.TaskSchedule, int64, error) {
	var items []*iapiserver.TaskSchedule
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("deleted_at IS NULL AND project_id = ? AND namespace = ?", req.ProjectID, req.Namespace)
		if req.IncludeSystem {
			query = query.Where("created_by IN ?", []string{req.CreatedBy, iapiserver.DefaultTaskCenterCreatedBy})
		} else {
			query = query.Where("created_by = ?", req.CreatedBy)
		}
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskSchedule{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *taskCenterStore) GetTaskSchedule(ctx context.Context, id string) (*iapiserver.TaskSchedule, error) {
	var item iapiserver.TaskSchedule
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrTaskScheduleNotFound, "task schedule not found")
	}
	return &item, nil
}
func (s *taskCenterStore) AddTaskSchedule(ctx context.Context, data *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}
func (s *taskCenterStore) UpdateTaskSchedule(ctx context.Context, data *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListScheduleExecutions(ctx context.Context, req *iapiserver.ScheduleExecutionListRequest) ([]*iapiserver.TaskScheduleExecution, int64, error) {
	var items []*iapiserver.TaskScheduleExecution
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("schedule_id = ?", req.ScheduleID)
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskScheduleExecution{}), filter).Order("scheduled_at DESC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) AddProjectionEventIdempotent(ctx context.Context, data *iapiserver.RuntimeProjectionEvent) (*iapiserver.RuntimeProjectionEvent, bool, error) {
	var existing iapiserver.RuntimeProjectionEvent
	err := s.ds.db.WithContext(ctx).Where("runtime_event_id = ?", data.RuntimeEventID).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, errors.WithStack(err)
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, false, errors.WithStack(err)
	}
	return data, true, nil
}

func (s *taskCenterStore) ListNonTerminalAtomicTasks(ctx context.Context, limit int) ([]*iapiserver.AtomicTask, error) {
	if limit <= 0 {
		limit = 100
	}
	var items []*iapiserver.AtomicTask
	err := s.ds.db.WithContext(ctx).Where("runtime_execution_id <> '' AND (owner_type IS NULL OR owner_type = '') AND status IN ?", []string{
		iapiserver.AtomicTaskStatusPending, iapiserver.AtomicTaskStatusBlocked, iapiserver.AtomicTaskStatusReady,
		iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying, iapiserver.AtomicTaskStatusCancelRequested,
	}).Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *taskCenterStore) ListNonTerminalTaskGroups(ctx context.Context, limit int) ([]*iapiserver.TaskGroup, error) {
	if limit <= 0 {
		limit = 100
	}
	var items []*iapiserver.TaskGroup
	err := s.ds.db.WithContext(ctx).Where("runtime_execution_id <> '' AND status IN ?", []string{
		iapiserver.TaskGroupStatusPending, iapiserver.TaskGroupStatusRunning, iapiserver.TaskGroupStatusCancelRequested,
	}).Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *taskCenterStore) ListNonTerminalDAGTaskGroups(ctx context.Context, limit int) ([]*iapiserver.DAGTaskGroup, error) {
	if limit <= 0 {
		limit = 100
	}
	var items []*iapiserver.DAGTaskGroup
	err := s.ds.db.WithContext(ctx).Where("runtime_execution_id <> '' AND status IN ?", []string{
		iapiserver.TaskGroupStatusPending, iapiserver.TaskGroupStatusRunning, iapiserver.TaskGroupStatusCancelRequested,
	}).Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *taskCenterStore) ListActiveScheduleExecutions(ctx context.Context, limit int) ([]*iapiserver.TaskScheduleExecution, error) {
	if limit <= 0 {
		limit = 100
	}
	var items []*iapiserver.TaskScheduleExecution
	err := s.ds.db.WithContext(ctx).Where("status IN ?", []string{
		iapiserver.ScheduleExecutionStatusTriggered, iapiserver.ScheduleExecutionStatusRunning,
	}).Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *taskCenterStore) ApplyRuntimeProjection(ctx context.Context, task *iapiserver.AtomicTask, attempts []*iapiserver.TaskAttempt, event *iapiserver.RuntimeProjectionEvent) (bool, error) {
	applied := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.RuntimeProjectionEvent{}).Where("runtime_event_id = ?", event.RuntimeEventID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		for _, attempt := range attempts {
			var existing iapiserver.TaskAttempt
			err := tx.Where("atomic_task_id = ? AND attempt_no = ?", attempt.AtomicTaskID, attempt.AttemptNo).First(&existing).Error
			switch {
			case err == nil:
				attempt.ID = existing.ID
				attempt.CreatedAt = existing.CreatedAt
				if err := tx.Save(attempt).Error; err != nil {
					return err
				}
			case stderrors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(attempt).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}
		if err := tx.Save(task).Error; err != nil {
			return err
		}
		if task.CanvasNodeRunID != "" {
			if err := projectCanvasNode(tx, task); err != nil {
				return err
			}
		}
		if task.OwnerID != "" {
			if err := recalculateOwner(tx, task.OwnerType, task.OwnerID); err != nil {
				return err
			}
		}
		event.ProjectionStatus = iapiserver.RuntimeProjectionStatusApplied
		event.ProjectedAt = imachinery.Now()
		if err := tx.Save(event).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	return applied, errors.WithStack(err)
}

func projectCanvasNode(tx *gorm.DB, task *iapiserver.AtomicTask) error {
	status := task.Status
	if status == iapiserver.AtomicTaskStatusCancelRequested {
		status = iapiserver.AtomicTaskStatusRunning
	}
	values := map[string]any{"atomic_task_id": task.ID, "status": status, "progress": task.Progress, "output_json": mustJSON(task.Output), "last_error_json": mustJSON(task.LastError), "task_resource_version": task.ResourceVersion, "updated_at": time.Now()}
	if iapiserver.IsAtomicTaskTerminal(task.Status) {
		values["finished_at"] = time.Now()
	}
	result := tx.Model(&iapiserver.CanvasNodeRun{}).Where("id = ? AND task_resource_version < ?", task.CanvasNodeRunID, task.ResourceVersion).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	var nodes []*iapiserver.CanvasNodeRun
	if err := tx.Where("canvas_run_id = ?", task.CanvasRunID).Find(&nodes).Error; err != nil {
		return err
	}
	summary := iapiserver.TaskSummary{Total: len(nodes)}
	progress := 0.0
	terminal := true
	failed := false
	canceled := false
	timedOut := false
	output := map[string]any{}
	for _, node := range nodes {
		progress += node.Progress
		switch node.Status {
		case iapiserver.AtomicTaskStatusPending:
			summary.Pending++
			terminal = false
		case iapiserver.AtomicTaskStatusBlocked:
			summary.Blocked++
			terminal = false
		case iapiserver.AtomicTaskStatusReady, iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying:
			summary.Running++
			terminal = false
		case iapiserver.AtomicTaskStatusSuccess:
			summary.Success++
			output[node.NodeKey] = node.Output
		case iapiserver.AtomicTaskStatusFailed:
			summary.Failed++
			failed = true
		case iapiserver.AtomicTaskStatusCanceled:
			summary.Canceled++
			canceled = true
		case iapiserver.AtomicTaskStatusTimeout:
			summary.Failed++
			timedOut = true
		case iapiserver.AtomicTaskStatusSkipped:
			summary.Skipped++
		}
	}
	if len(nodes) > 0 {
		progress /= float64(len(nodes))
	}
	runStatus := iapiserver.CanvasRunStatusRunning
	if terminal {
		switch {
		case failed:
			runStatus = iapiserver.CanvasRunStatusFailed
		case timedOut:
			runStatus = iapiserver.CanvasRunStatusTimeout
		case canceled:
			runStatus = iapiserver.CanvasRunStatusCanceled
		default:
			runStatus = iapiserver.CanvasRunStatusSuccess
		}
	}
	runValues := map[string]any{"status": runStatus, "progress": progress, "summary_json": mustJSON(summary), "output_json": mustJSON(output), "task_resource_version": task.ResourceVersion, "updated_at": time.Now()}
	if terminal {
		runValues["finished_at"] = time.Now()
	}
	return tx.Model(&iapiserver.WorkflowCanvasRun{}).Where("id = ? AND task_resource_version < ?", task.CanvasRunID, task.ResourceVersion).Updates(runValues).Error
}

func recalculateOwner(tx *gorm.DB, ownerType, ownerID string) error {
	var tasks []*iapiserver.AtomicTask
	if err := tx.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID).Find(&tasks).Error; err != nil {
		return err
	}
	summary := iapiserver.TaskSummary{Total: len(tasks)}
	progress := 0.0
	result := map[string]any{}
	terminal := true
	failed := false
	canceled := false
	timedOut := false
	for _, task := range tasks {
		progress += task.Progress
		switch task.Status {
		case iapiserver.AtomicTaskStatusPending:
			summary.Pending++
			terminal = false
		case iapiserver.AtomicTaskStatusBlocked:
			summary.Blocked++
			terminal = false
		case iapiserver.AtomicTaskStatusReady, iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying, iapiserver.AtomicTaskStatusCancelRequested:
			summary.Running++
			terminal = false
		case iapiserver.AtomicTaskStatusSuccess:
			summary.Success++
			result[task.ChildKey] = task.Output
		case iapiserver.AtomicTaskStatusFailed:
			summary.Failed++
			failed = true
		case iapiserver.AtomicTaskStatusCanceled:
			summary.Canceled++
			canceled = true
		case iapiserver.AtomicTaskStatusTimeout:
			summary.Failed++
			timedOut = true
		case iapiserver.AtomicTaskStatusSkipped:
			summary.Skipped++
		}
	}
	if len(tasks) > 0 {
		progress /= float64(len(tasks))
	}
	status := iapiserver.TaskGroupStatusRunning
	if terminal {
		switch {
		case failed:
			status = iapiserver.TaskGroupStatusFailed
		case timedOut:
			status = iapiserver.TaskGroupStatusTimeout
		case canceled:
			status = iapiserver.TaskGroupStatusCanceled
		default:
			status = iapiserver.TaskGroupStatusSuccess
		}
	}
	values := map[string]any{"summary_json": mustJSON(summary), "result_json": mustJSON(result), "progress": progress, "status": status, "updated_at": time.Now()}
	switch ownerType {
	case iapiserver.TaskOwnerTypeGroup:
		return tx.Model(&iapiserver.TaskGroup{}).Where("id = ?", ownerID).Updates(values).Error
	case iapiserver.TaskOwnerTypeDAGGroup:
		return tx.Model(&iapiserver.DAGTaskGroup{}).Where("id = ?", ownerID).Updates(values).Error
	}
	return nil
}

func (s *taskCenterStore) AcquireScheduleExecution(ctx context.Context, data *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error) {
	var result *iapiserver.TaskScheduleExecution
	acquired := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.TaskScheduleExecution
		err := tx.Where("schedule_id = ? AND scheduled_at = ?", data.ScheduleID, data.ScheduledAt.Time).First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var active int64
		if err := tx.Model(&iapiserver.TaskScheduleExecution{}).Where("schedule_id = ? AND status IN ?", data.ScheduleID, []string{iapiserver.ScheduleExecutionStatusTriggered, iapiserver.ScheduleExecutionStatusRunning}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			data.Status = iapiserver.ScheduleExecutionStatusSkippedOverlap
			data.Reason = "previous schedule execution is still active"
			data.CompletedAt = imachinery.Now()
		} else {
			acquired = true
		}
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		result = data
		return nil
	})
	return result, acquired, errors.WithStack(err)
}

func (s *taskCenterStore) UpdateScheduleExecution(ctx context.Context, data *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func mustJSON(value any) string { data, _ := json.Marshal(value); return string(data) }

func mapNotFound(err error, errorCode int, message string) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(errorCode, message)
	}
	return errors.WithStack(err)
}
func fingerprint(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("marshal-error:%T", value)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func atomicTaskFingerprint(task *iapiserver.AtomicTask) string {
	return fingerprint(map[string]any{
		"name": task.Name, "description": task.Description, "function_ref": task.FunctionRef,
		"arguments": task.Arguments, "required_capabilities": task.RequiredCapabilities,
		"retry_policy": task.RetryPolicy, "timeout_policy": task.TimeoutPolicy, "cancel_policy": task.CancelPolicy,
		"child_key": task.ChildKey, "application_run_id": task.ApplicationRunID,
		"canvas_run_id": task.CanvasRunID, "canvas_node_run_id": task.CanvasNodeRunID,
		"idempotency_scope": task.IdempotencyScope, "idempotency_key": task.IdempotencyKey,
		"project_id": task.ProjectID, "namespace": task.Namespace, "created_by": task.CreatedBy, "tags": task.Tags,
	})
}

func taskGroupFingerprint(group *iapiserver.TaskGroup) string {
	return fingerprint(map[string]any{
		"name": group.Name, "description": group.Description, "mode": group.Mode, "tasks": group.Tasks,
		"strategy": group.Strategy, "idempotency_scope": group.IdempotencyScope,
		"idempotency_key": group.IdempotencyKey, "project_id": group.ProjectID,
		"namespace": group.Namespace, "created_by": group.CreatedBy,
	})
}

func dagTaskGroupFingerprint(group *iapiserver.DAGTaskGroup) string {
	return fingerprint(map[string]any{
		"name": group.Name, "description": group.Description, "nodes": group.Nodes, "edges": group.Edges,
		"input": group.Input, "output_mapping": group.OutputMapping, "canvas_version_id": group.CanvasVersionID,
		"idempotency_scope": group.IdempotencyScope, "idempotency_key": group.IdempotencyKey,
		"project_id": group.ProjectID, "namespace": group.Namespace, "created_by": group.CreatedBy,
	})
}

var _ interface {
	ListAtomicTasks(context.Context, *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error)
} = (*taskCenterStore)(nil)

var _ = imachinery.BasicQueryParam{}

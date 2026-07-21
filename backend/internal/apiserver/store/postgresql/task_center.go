package postgresql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type taskCenterStore struct{ ds *datastore }

func newTaskCenterStore(ds *datastore) *taskCenterStore { return &taskCenterStore{ds: ds} }

func (s *taskCenterStore) ListAtomicTasks(ctx context.Context, req *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error) {
	var items []*iapiserver.AtomicTask
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

func (s *taskCenterStore) GetAtomicTasksByIDs(ctx context.Context, ids []string) ([]*iapiserver.AtomicTask, error) {
	if len(ids) == 0 {
		return []*iapiserver.AtomicTask{}, nil
	}
	var items []*iapiserver.AtomicTask
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
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
		if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(data).Error; err != nil {
				return err
			}
			return projectAtomicTaskCreated(tx, data)
		}); err != nil {
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
		if err := projectAtomicTaskCreated(tx, data); err != nil {
			return err
		}
		result, created = data, true
		return nil
	})
	return result, created, err
}

func (s *taskCenterStore) UpdateAtomicTask(ctx context.Context, data *iapiserver.AtomicTask) (*iapiserver.AtomicTask, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.AtomicTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "id = ?", data.ID).Error; err != nil {
			return err
		}
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		return projectAtomicTaskChanged(tx, &previous, data)
	}); err != nil {
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
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskGroup{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetTaskGroupsByIDs(ctx context.Context, ids []string) ([]*iapiserver.TaskGroup, error) {
	if len(ids) == 0 {
		return []*iapiserver.TaskGroup{}, nil
	}
	var items []*iapiserver.TaskGroup
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
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
		if err := projectTaskGroupCreated(tx, iapiserver.TaskOwnerTypeGroup, group); err != nil {
			return err
		}
		for _, task := range tasks {
			task.OwnerType = iapiserver.TaskOwnerTypeGroup
			task.OwnerID = group.ID
			if err := tx.Create(task).Error; err != nil {
				return errors.WithStack(err)
			}
			if err := projectAtomicTaskCreated(tx, task); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return returnGroup, created, err
}

func (s *taskCenterStore) UpdateTaskGroup(ctx context.Context, data *iapiserver.TaskGroup) (*iapiserver.TaskGroup, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.TaskGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "id = ?", data.ID).Error; err != nil {
			return err
		}
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		return projectTaskGroupChanged(tx, iapiserver.TaskOwnerTypeGroup, &previous, data, false)
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListDAGTaskGroups(ctx context.Context, req *iapiserver.DAGTaskGroupListRequest) ([]*iapiserver.DAGTaskGroup, int64, error) {
	var items []*iapiserver.DAGTaskGroup
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
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.DAGTaskGroup{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetDAGTaskGroupsByIDs(ctx context.Context, ids []string) ([]*iapiserver.DAGTaskGroup, error) {
	if len(ids) == 0 {
		return []*iapiserver.DAGTaskGroup{}, nil
	}
	var items []*iapiserver.DAGTaskGroup
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
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
		if err := projectTaskGroupCreated(tx, iapiserver.TaskOwnerTypeDAGGroup, group); err != nil {
			return err
		}
		for _, task := range tasks {
			task.OwnerType = iapiserver.TaskOwnerTypeDAGGroup
			task.OwnerID = group.ID
			if err := tx.Create(task).Error; err != nil {
				return errors.WithStack(err)
			}
			if err := projectAtomicTaskCreated(tx, task); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return returnGroup, created, err
}

func (s *taskCenterStore) UpdateDAGTaskGroup(ctx context.Context, data *iapiserver.DAGTaskGroup) (*iapiserver.DAGTaskGroup, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.DAGTaskGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "id = ?", data.ID).Error; err != nil {
			return err
		}
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		return projectTaskGroupChanged(tx, iapiserver.TaskOwnerTypeDAGGroup, &previous, data, false)
	}); err != nil {
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
			if err := projectAtomicTaskCreated(tx, task); err != nil {
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
		if req.ExecutionMode != "" {
			query = query.Where("execution_mode = ?", req.ExecutionMode)
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.TaskSchedule{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *taskCenterStore) GetTaskSchedulesByIDs(ctx context.Context, ids []string) ([]*iapiserver.TaskSchedule, error) {
	if len(ids) == 0 {
		return []*iapiserver.TaskSchedule{}, nil
	}
	var items []*iapiserver.TaskSchedule
	if err := s.ds.db.WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *taskCenterStore) GetTaskSchedule(ctx context.Context, id string) (*iapiserver.TaskSchedule, error) {
	var item iapiserver.TaskSchedule
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrTaskScheduleNotFound, "task schedule not found")
	}
	return &item, nil
}
func (s *taskCenterStore) GetTaskScheduleBySystemKey(ctx context.Context, key string) (*iapiserver.TaskSchedule, error) {
	var item iapiserver.TaskSchedule
	if err := s.ds.db.WithContext(ctx).Where("system_key = ? AND deleted_at IS NULL", key).First(&item).Error; err != nil {
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

// EnsureSystemTaskSchedule 依靠 system_key 唯一索引原子补齐系统计划和一对一状态，多 Worker 并发启动只会创建一份。
func (s *taskCenterStore) EnsureSystemTaskSchedule(ctx context.Context, data *iapiserver.TaskSchedule, state *iapiserver.ScheduleReconcileState) (*iapiserver.TaskSchedule, bool, error) {
	created := false
	result := data
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(data)
		if insert.Error != nil {
			return insert.Error
		}
		created = insert.RowsAffected == 1
		if !created {
			var existing iapiserver.TaskSchedule
			if err := tx.Where("system_key = ? AND deleted_at IS NULL", data.SystemKey).First(&existing).Error; err != nil {
				return err
			}
			result = &existing
			return nil
		}
		state.ScheduleID = data.ID
		return tx.Create(state).Error
	})
	return result, created, errors.WithStack(err)
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

func (s *taskCenterStore) GetScheduleExecution(ctx context.Context, id string) (*iapiserver.TaskScheduleExecution, error) {
	var execution iapiserver.TaskScheduleExecution
	if err := s.ds.db.WithContext(ctx).First(&execution, "id = ?", id).Error; err != nil {
		return nil, mapNotFound(err, code.ErrTaskScheduleNotFound, "schedule execution not found")
	}
	return &execution, nil
}

// GetScheduleExecutionAt 按唯一业务键读取已有轮次；未命中不是错误，misfire 守卫据此安全跳过首次迟到触发。
func (s *taskCenterStore) GetScheduleExecutionAt(ctx context.Context, scheduleID string, scheduledAt time.Time) (*iapiserver.TaskScheduleExecution, error) {
	var execution iapiserver.TaskScheduleExecution
	err := s.ds.db.WithContext(ctx).Where("schedule_id = ? AND scheduled_at = ?", scheduleID, scheduledAt).First(&execution).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &execution, nil
}

func (s *taskCenterStore) ListLatestScheduleExecutions(ctx context.Context, scheduleIDs []string) (map[string]*iapiserver.TaskScheduleExecution, error) {
	result := make(map[string]*iapiserver.TaskScheduleExecution)
	if len(scheduleIDs) == 0 {
		return result, nil
	}
	var items []*iapiserver.TaskScheduleExecution
	if err := s.ds.db.WithContext(ctx).Raw("SELECT DISTINCT ON (schedule_id) * FROM task_schedule_executions WHERE schedule_id IN ? ORDER BY schedule_id, scheduled_at DESC", scheduleIDs).Scan(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	for _, item := range items {
		if err := item.AfterFind(s.ds.db); err != nil {
			return nil, err
		}
		result[item.ScheduleID] = item
	}
	return result, nil
}

func (s *taskCenterStore) ListScheduleSources(ctx context.Context, targetType string, targetIDs []string) (map[string]*iapiserver.ScheduleSourceSummary, error) {
	result := make(map[string]*iapiserver.ScheduleSourceSummary)
	if len(targetIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		TargetID            string
		ScheduleID          string
		ScheduleName        string
		ScheduleExecutionID string
		ScheduledAt         imachinery.Time
	}
	err := s.ds.db.WithContext(ctx).
		Table("task_schedule_executions AS execution").
		Select("execution.target_id, schedule.id AS schedule_id, schedule.name AS schedule_name, execution.id AS schedule_execution_id, execution.scheduled_at").
		Joins("JOIN task_schedules AS schedule ON schedule.id = execution.schedule_id").
		Where("execution.target_type = ? AND execution.target_id IN ?", targetType, targetIDs).
		Order("execution.scheduled_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, row := range rows {
		if _, exists := result[row.TargetID]; exists {
			continue
		}
		result[row.TargetID] = &iapiserver.ScheduleSourceSummary{ScheduleID: row.ScheduleID, ScheduleName: row.ScheduleName, ScheduleExecutionID: row.ScheduleExecutionID, ScheduledAt: row.ScheduledAt}
	}
	return result, nil
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
		type attemptChange struct {
			previous *iapiserver.TaskAttempt
			current  *iapiserver.TaskAttempt
		}
		changes := make([]attemptChange, 0, len(attempts))
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
		var previousTask iapiserver.AtomicTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", task.ID).First(&previousTask).Error; err != nil {
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
				previous := existing
				changes = append(changes, attemptChange{previous: &previous, current: attempt})
			case stderrors.Is(err, gorm.ErrRecordNotFound):
				finalAttempt := *attempt
				scheduled := finalAttempt
				scheduled.Status = iapiserver.TaskAttemptStatusScheduled
				scheduled.StartedAt = imachinery.Time{}
				scheduled.CompletedAt = imachinery.Time{}
				scheduled.DurationMS = 0
				scheduled.OutputSnapshot = map[string]any{}
				scheduled.Error = iapiserver.TaskError{}
				scheduled.Retryable = false
				if err := tx.Create(&scheduled).Error; err != nil {
					return err
				}
				changes = append(changes, attemptChange{current: &scheduled})
				previous := scheduled
				if finalAttempt.Status != iapiserver.TaskAttemptStatusScheduled {
					if !finalAttempt.StartedAt.IsZero() && finalAttempt.Status != iapiserver.TaskAttemptStatusRunning {
						running := finalAttempt
						running.ID, running.CreatedAt, running.ResourceVersion = scheduled.ID, scheduled.CreatedAt, scheduled.ResourceVersion
						running.Status = iapiserver.TaskAttemptStatusRunning
						running.CompletedAt, running.DurationMS = imachinery.Time{}, 0
						running.OutputSnapshot, running.Error = map[string]any{}, iapiserver.TaskError{}
						if err := tx.Save(&running).Error; err != nil {
							return err
						}
						changes = append(changes, attemptChange{previous: &previous, current: &running})
						previous = running
					}
					finalAttempt.ID, finalAttempt.CreatedAt, finalAttempt.ResourceVersion = previous.ID, previous.CreatedAt, previous.ResourceVersion
					if err := tx.Save(&finalAttempt).Error; err != nil {
						return err
					}
					changes = append(changes, attemptChange{previous: &previous, current: &finalAttempt})
				}
			default:
				return err
			}
		}
		if err := tx.Save(task).Error; err != nil {
			return err
		}
		if err := projectAtomicTaskChanged(tx, &previousTask, task); err != nil {
			return err
		}
		for _, change := range changes {
			if err := projectTaskAttemptChanged(tx, task, change.previous, change.current); err != nil {
				return err
			}
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
	switch ownerType {
	case iapiserver.TaskOwnerTypeGroup:
		var group iapiserver.TaskGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "id = ?", ownerID).Error; err != nil {
			return err
		}
		previous := group
		group.Summary, group.Result, group.Progress, group.Status = summary, result, progress, status
		if err := tx.Save(&group).Error; err != nil {
			return err
		}
		return projectTaskGroupChanged(tx, ownerType, &previous, &group, false)
	case iapiserver.TaskOwnerTypeDAGGroup:
		var group iapiserver.DAGTaskGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "id = ?", ownerID).Error; err != nil {
			return err
		}
		previous := group
		group.Summary, group.Result, group.Progress, group.Status = summary, result, progress, status
		if err := tx.Save(&group).Error; err != nil {
			return err
		}
		return projectTaskGroupChanged(tx, ownerType, &previous, &group, false)
	}
	return nil
}

func (s *taskCenterStore) AcquireScheduleExecution(ctx context.Context, data *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error) {
	var result *iapiserver.TaskScheduleExecution
	acquired := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁定父计划将“同一 scheduled_at 幂等”和“活动轮次唯一”串行化，避免多 Worker 启动竞态落成唯一索引错误。
		var schedule iapiserver.TaskSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&schedule, "id = ?", data.ScheduleID).Error; err != nil {
			return err
		}
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
		if err := publishScheduleExecutionEvent(tx, data); err != nil {
			return err
		}
		applyScheduleSummaryTransition(&schedule, "", data.Status)
		if err := tx.Save(&schedule).Error; err != nil {
			return err
		}
		result = data
		return nil
	})
	return result, acquired, errors.WithStack(err)
}

// WithScheduleReconcileLock 使用事务级 advisory lock 防止同一 Conductor task 的超时重投与旧调用并发写 checkpoint。
func (s *taskCenterStore) WithScheduleReconcileLock(ctx context.Context, scheduleID string, fn func() error) (bool, error) {
	acquired := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", "task-center-reconcile:"+scheduleID).Scan(&acquired).Error; err != nil || !acquired {
			return err
		}
		return fn()
	})
	return acquired, errors.WithStack(err)
}

func (s *taskCenterStore) UpdateScheduleExecution(ctx context.Context, data *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.TaskScheduleExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "id = ?", data.ID).Error; err != nil {
			return err
		}
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		if err := updateScheduleSummary(tx, data.ScheduleID, previous.Status, data.Status); err != nil {
			return err
		}
		if isScheduleExecutionTerminal(data.Status) {
			return publishScheduleExecutionEvent(tx, data)
		}
		return nil
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) GetScheduleReconcileState(ctx context.Context, scheduleID string) (*iapiserver.ScheduleReconcileState, error) {
	var state iapiserver.ScheduleReconcileState
	if err := s.ds.db.WithContext(ctx).First(&state, "schedule_id = ?", scheduleID).Error; err != nil {
		return nil, mapNotFound(err, code.ErrTaskScheduleNotFound, "schedule reconcile state not found")
	}
	return &state, nil
}

// CompleteScheduleReconcile 原子提交轮次终态、checkpoint 和不回退的累计统计。
func (s *taskCenterStore) CompleteScheduleReconcile(ctx context.Context, execution *iapiserver.TaskScheduleExecution, state *iapiserver.ScheduleReconcileState) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.TaskScheduleExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "id = ?", execution.ID).Error; err != nil {
			return err
		}
		if err := tx.Save(execution).Error; err != nil {
			return err
		}
		if err := updateScheduleSummary(tx, execution.ScheduleID, previous.Status, execution.Status); err != nil {
			return err
		}
		if err := tx.Save(state).Error; err != nil {
			return err
		}
		if isScheduleExecutionTerminal(execution.Status) {
			return publishScheduleExecutionEvent(tx, execution)
		}
		return nil
	}))
}

func isScheduleExecutionTerminal(status string) bool {
	return status == iapiserver.ScheduleExecutionStatusSuccess || status == iapiserver.ScheduleExecutionStatusFailed || status == iapiserver.ScheduleExecutionStatusCanceled || status == iapiserver.ScheduleExecutionStatusSkippedOverlap || status == iapiserver.ScheduleExecutionStatusTriggerFailed
}

func publishScheduleExecutionEvent(tx *gorm.DB, execution *iapiserver.TaskScheduleExecution) error {
	payload := map[string]any{"task_schedule_id": execution.ScheduleID, "schedule_execution_id": execution.ID, "scheduled_at": execution.ScheduledAt, "execution_mode": execution.ExecutionMode, "status": execution.Status, "target_type": execution.TargetType, "target_id": execution.TargetID, "reason": execution.Reason, "occurred_at": imachinery.Now()}
	if execution.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
		payload["reconcile_summary"] = execution.ReconcileSummary
	} else {
		payload["reconcile_summary"] = nil
	}
	return publishOutbox(tx, OutboxTopicScheduleExecutionRecorded, execution.ID+":"+execution.Status, payload)
}

func updateScheduleSummary(tx *gorm.DB, scheduleID, from, to string) error {
	if from == to {
		return nil
	}
	var schedule iapiserver.TaskSchedule
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&schedule, "id = ?", scheduleID).Error; err != nil {
		return err
	}
	applyScheduleSummaryTransition(&schedule, from, to)
	return tx.Save(&schedule).Error
}

func applyScheduleSummaryTransition(schedule *iapiserver.TaskSchedule, from, to string) {
	if from == "" {
		schedule.Summary.TotalTriggered++
		if to == iapiserver.ScheduleExecutionStatusTriggered || to == iapiserver.ScheduleExecutionStatusRunning {
			schedule.Summary.Running++
		}
		if to == iapiserver.ScheduleExecutionStatusSkippedOverlap {
			schedule.Summary.SkippedOverlap++
		}
		return
	}
	if (from == iapiserver.ScheduleExecutionStatusTriggered || from == iapiserver.ScheduleExecutionStatusRunning) && isScheduleExecutionTerminal(to) && schedule.Summary.Running > 0 {
		schedule.Summary.Running--
	}
	switch to {
	case iapiserver.ScheduleExecutionStatusSuccess:
		schedule.Summary.Success++
	case iapiserver.ScheduleExecutionStatusFailed, iapiserver.ScheduleExecutionStatusTriggerFailed:
		schedule.Summary.Failed++
	case iapiserver.ScheduleExecutionStatusCanceled:
		schedule.Summary.Canceled++
	case iapiserver.ScheduleExecutionStatusSkippedOverlap:
		schedule.Summary.SkippedOverlap++
	}
}

func (s *taskCenterStore) PruneReconcileExecutions(ctx context.Context, scheduleID string, retention iapiserver.HistoryRetention, now time.Time) (int64, error) {
	var deleted int64
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var keepIDs []string
		collect := func(statuses []string, limit int, cutoff *time.Time) error {
			if limit <= 0 {
				return nil
			}
			q := tx.Model(&iapiserver.TaskScheduleExecution{}).Select("id").Where("schedule_id = ? AND execution_mode = ? AND status IN ?", scheduleID, iapiserver.TaskScheduleModeReconcile, statuses)
			if cutoff != nil {
				q = q.Where("completed_at >= ?", *cutoff)
			}
			var ids []string
			if err := q.Order("completed_at DESC, scheduled_at DESC").Limit(limit).Pluck("id", &ids).Error; err != nil {
				return err
			}
			keepIDs = append(keepIDs, ids...)
			return nil
		}
		if err := collect([]string{iapiserver.ScheduleExecutionStatusSuccess}, retention.SuccessCount, nil); err != nil {
			return err
		}
		if err := collect([]string{iapiserver.ScheduleExecutionStatusSkippedOverlap}, retention.SkippedCount, nil); err != nil {
			return err
		}
		cutoff := now.Add(-time.Duration(retention.FailureDurationSeconds) * time.Second)
		if err := collect([]string{iapiserver.ScheduleExecutionStatusFailed, iapiserver.ScheduleExecutionStatusTriggerFailed}, retention.FailureCount, &cutoff); err != nil {
			return err
		}
		q := tx.Where("schedule_id = ? AND execution_mode = ? AND status NOT IN ?", scheduleID, iapiserver.TaskScheduleModeReconcile, []string{iapiserver.ScheduleExecutionStatusTriggered, iapiserver.ScheduleExecutionStatusRunning})
		if len(keepIDs) > 0 {
			q = q.Where("id NOT IN ?", keepIDs)
		}
		result := q.Delete(&iapiserver.TaskScheduleExecution{})
		deleted = result.RowsAffected
		return result.Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	return deleted, errors.WithStack(err)
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

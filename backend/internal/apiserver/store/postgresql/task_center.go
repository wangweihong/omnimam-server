package postgresql

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	taskCenterCreatedAtColumn = "createdAt"
	taskCenterUpdatedAtColumn = "updatedAt"
)

type taskCenterStore struct{ ds *datastore }

func newTaskCenter(ds *datastore) *taskCenterStore { return &taskCenterStore{ds: ds} }

func (s *taskCenterStore) ListDefinitions(
	ctx context.Context,
	req *iapiserver.TaskDefinitionListRequest,
) ([]*iapiserver.TaskDefinition, int64, error) {
	var items []*iapiserver.TaskDefinition
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("deleted_at IS NULL OR deleted_at = ?", time.Time{})
		if req.DefinitionType != "" {
			q = q.Where("definition_type = ?", req.DefinitionType)
		}
		if req.ProjectID != "" {
			q = q.Where("project_id = ?", req.ProjectID)
		}
		if req.Namespace != "" {
			q = q.Where("namespace = ?", req.Namespace)
		}
		return q
	}
	query := taskCenterListQuery(ctx, s.ds.db.Model(&iapiserver.TaskDefinition{}), req.BasicQueryParam, filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *taskCenterStore) GetDefinition(
	ctx context.Context,
	definitionType, id string,
) (*iapiserver.TaskDefinition, error) {
	var item iapiserver.TaskDefinition
	query := s.ds.db.WithContext(ctx).Where("id = ?", id)
	if definitionType != "" {
		query = query.Where("definition_type = ?", definitionType)
	}
	if err := query.First(&item).Error; err != nil {
		return nil, mapTaskDefinitionError(err)
	}
	return &item, nil
}

func (s *taskCenterStore) AddDefinition(
	ctx context.Context,
	data *iapiserver.TaskDefinition,
) (*iapiserver.TaskDefinition, error) {
	fillTaskDefinitionDefaults(data)
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) ListRuns(
	ctx context.Context,
	req *iapiserver.TaskRunListRequest,
) ([]*iapiserver.TaskRun, int64, error) {
	var items []*iapiserver.TaskRun
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("deleted_at IS NULL OR deleted_at = ?", time.Time{})
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		if req.DefinitionType != "" {
			q = q.Where("definition_type = ?", req.DefinitionType)
		}
		if req.RootRunID != "" {
			q = q.Where("root_run_id = ?", req.RootRunID)
		}
		if req.ProjectID != "" {
			q = q.Where("project_id = ?", req.ProjectID)
		}
		return q
	}
	query := taskCenterListQuery(ctx, s.ds.db.Model(&iapiserver.TaskRun{}), req.BasicQueryParam, filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *taskCenterStore) GetRun(ctx context.Context, id string) (*iapiserver.TaskRun, error) {
	var item iapiserver.TaskRun
	if err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		Where("deleted_at IS NULL OR deleted_at = ?", time.Time{}).
		First(&item).Error; err != nil {
		return nil, mapTaskRunError(err)
	}
	return &item, nil
}

func (s *taskCenterStore) AddRun(ctx context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error) {
	fillTaskRunDefaults(data)
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) UpdateRun(ctx context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) SoftDeleteRun(ctx context.Context, id string) error {
	now := imachinery.NewTime(time.Now())
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.TaskRun{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error; err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *taskCenterStore) ListAttempts(
	ctx context.Context,
	req *iapiserver.TaskAttemptListRequest,
) ([]*iapiserver.TaskAttempt, int64, error) {
	var items []*iapiserver.TaskAttempt
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		if req.RunID != "" {
			q = q.Where("run_id = ?", req.RunID)
		}
		return q
	}
	query := taskCenterListQuery(ctx, s.ds.db.Model(&iapiserver.TaskAttempt{}), req.BasicQueryParam, filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *taskCenterStore) RegisterWorker(ctx context.Context, data *iapiserver.Worker) (*iapiserver.Worker, error) {
	now := imachinery.NewTime(time.Now())
	if data.Status == "" {
		data.Status = iapiserver.WorkerStatusOnline
	}
	if data.MaxConcurrency <= 0 {
		data.MaxConcurrency = 1
	}
	if data.Name == "" {
		data.Name = data.WorkerType
	}
	data.HeartbeatAt = now
	data.RegisteredAt = now
	data.LastSeenAt = now
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *taskCenterStore) HeartbeatWorker(
	ctx context.Context,
	req *iapiserver.WorkerHeartbeatRequest,
) (*iapiserver.Worker, error) {
	now := imachinery.NewTime(time.Now())
	observedAt := req.ObservedAt
	if observedAt.IsZero() {
		observedAt = now
	}
	var worker iapiserver.Worker
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.WorkerID).
			First(&worker).Error; err != nil {
			return mapWorkerError(err)
		}
		if req.Status != "" {
			worker.Status = req.Status
		}
		worker.RunningCount = req.RunningCount
		worker.HeartbeatAt = observedAt
		worker.LastSeenAt = now
		return errors.WithStack(tx.Save(&worker).Error)
	})
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

func (s *taskCenterStore) ClaimRun(
	ctx context.Context,
	req *iapiserver.ClaimTaskRunRequest,
) (*iapiserver.ClaimTaskRunResponse, error) {
	var ret iapiserver.ClaimTaskRunResponse
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var worker iapiserver.Worker
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.WorkerID).
			First(&worker).Error; err != nil {
			return mapWorkerError(err)
		}
		if worker.Status != iapiserver.WorkerStatusOnline && worker.Status != iapiserver.WorkerStatusBusy {
			return errors.NewStatusF(code.ErrTaskWorkerNotAvailable, "worker is not available")
		}
		if worker.MaxConcurrency > 0 && worker.RunningCount >= worker.MaxConcurrency {
			return errors.NewStatusF(code.ErrTaskWorkerNotAvailable, "worker concurrency is full")
		}

		var candidates []*iapiserver.TaskRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ?", []string{
				iapiserver.TaskRunStatusReady,
				iapiserver.TaskRunStatusPending,
				iapiserver.TaskRunStatusRetrying,
			}).
			Where("deleted_at IS NULL OR deleted_at = ?", time.Time{}).
			Order(clause.OrderByColumn{Column: clause.Column{Name: taskCenterCreatedAtColumn}, Desc: false}).
			Limit(maxClaimScanLimit(req.MaxCount)).
			Find(&candidates).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, run := range candidates {
			definition, err := getTaskDefinitionForUpdate(tx, run.DefinitionType, run.DefinitionID)
			if err != nil {
				return err
			}
			if !capabilityMatches(req.Capabilities, definition.RequiredCapabilities) {
				continue
			}
			now := time.Now()
			run.CurrentAttempt++
			run.Status = iapiserver.TaskRunStatusClaimed
			if run.MaxAttempts == 0 {
				run.MaxAttempts = 1
			}
			attempt := &iapiserver.TaskAttempt{
				RunID:         run.ID,
				AttemptNo:     run.CurrentAttempt,
				WorkerID:      worker.ID,
				Status:        iapiserver.TaskAttemptStatusClaimed,
				InputSnapshot: run.Input,
				StartedAt:     imachinery.NewTime(now),
			}
			attempt.Name = run.Name + "-attempt"
			if err := tx.Create(attempt).Error; err != nil {
				return errors.WithStack(err)
			}
			lease := &iapiserver.ExecutionLease{
				RunID:      run.ID,
				AttemptID:  attempt.ID,
				WorkerID:   worker.ID,
				AcquiredAt: imachinery.NewTime(now),
				ExpireAt:   imachinery.NewTime(now.Add(iapiserver.DefaultTaskCenterLeaseDuration)),
				Status:     iapiserver.LeaseStatusActive,
			}
			lease.Name = run.Name + "-lease"
			if err := tx.Create(lease).Error; err != nil {
				return errors.WithStack(err)
			}
			attempt.LeaseID = lease.ID
			if err := tx.Save(attempt).Error; err != nil {
				return errors.WithStack(err)
			}
			if err := tx.Save(run).Error; err != nil {
				return errors.WithStack(err)
			}
			worker.RunningCount++
			if worker.RunningCount >= worker.MaxConcurrency {
				worker.Status = iapiserver.WorkerStatusBusy
			}
			if err := tx.Save(&worker).Error; err != nil {
				return errors.WithStack(err)
			}
			ret.TaskRun = run
			ret.Attempt = attempt
			ret.Lease = lease
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (s *taskCenterStore) UpdateProgress(
	ctx context.Context,
	req *iapiserver.ProgressUpdateRequest,
) (*iapiserver.TaskRun, error) {
	var run iapiserver.TaskRun
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lease, attempt, loadedRun, err := loadActiveTaskLease(tx, req.RunID, req.AttemptID, req.LeaseID, req.WorkerID)
		if err != nil {
			return err
		}
		now := imachinery.NewTime(time.Now())
		loadedRun.Status = iapiserver.TaskRunStatusRunning
		loadedRun.Progress = req.Progress
		attempt.Status = iapiserver.TaskAttemptStatusRunning
		attempt.OutputSnapshot = req.OutputSnapshot
		attempt.ExternalJobID = req.ExternalJobID
		attempt.ProgressAt = now
		attempt.HeartbeatAt = now
		if err := tx.Save(attempt).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(lease).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(loadedRun).Error; err != nil {
			return errors.WithStack(err)
		}
		run = *loadedRun
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *taskCenterStore) CompleteRun(
	ctx context.Context,
	req *iapiserver.TaskRunCompleteRequest,
) (*iapiserver.TaskRun, error) {
	var run iapiserver.TaskRun
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lease, attempt, loadedRun, err := loadActiveTaskLease(tx, req.RunID, req.AttemptID, req.LeaseID, req.WorkerID)
		if err != nil {
			return err
		}
		now := imachinery.NewTime(time.Now())
		loadedRun.Status = iapiserver.TaskRunStatusSuccess
		loadedRun.Progress = 1
		loadedRun.Output = req.Output
		loadedRun.CompletedAt = now
		attempt.Status = iapiserver.TaskAttemptStatusSuccess
		attempt.OutputSnapshot = req.Output
		attempt.CompletedAt = now
		attempt.ExternalJobID = req.ExternalJobID
		attempt.DurationMS = durationMS(attempt.StartedAt, now)
		lease.Status = iapiserver.LeaseStatusReleased
		if err := decrementWorkerRunning(tx, req.WorkerID); err != nil {
			return err
		}
		if err := tx.Save(attempt).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(lease).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(loadedRun).Error; err != nil {
			return errors.WithStack(err)
		}
		run = *loadedRun
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *taskCenterStore) FailRun(
	ctx context.Context,
	req *iapiserver.TaskRunFailRequest,
) (*iapiserver.TaskRun, error) {
	var run iapiserver.TaskRun
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lease, attempt, loadedRun, err := loadActiveTaskLease(tx, req.RunID, req.AttemptID, req.LeaseID, req.WorkerID)
		if err != nil {
			return err
		}
		now := imachinery.NewTime(time.Now())
		taskErr := req.Error
		if taskErr.OccurredAt.IsZero() {
			taskErr.OccurredAt = now
		}
		attempt.Status = iapiserver.TaskAttemptStatusFailed
		attempt.Error = taskErr
		attempt.FailureType = taskErr.FailureType
		attempt.Retryable = taskErr.Retryable
		attempt.LogsRef = req.LogsRef
		attempt.ExternalJobID = req.ExternalJobID
		attempt.CompletedAt = now
		attempt.DurationMS = durationMS(attempt.StartedAt, now)
		loadedRun.LastError = taskErr
		loadedRun.Progress = 1
		loadedRun.CompletedAt = now
		if taskErr.Retryable && (loadedRun.MaxAttempts < 0 || loadedRun.CurrentAttempt < loadedRun.MaxAttempts) {
			loadedRun.Status = iapiserver.TaskRunStatusRetrying
			loadedRun.Progress = 0
			loadedRun.CompletedAt = imachinery.Time{}
		} else {
			loadedRun.Status = iapiserver.TaskRunStatusFailed
		}
		lease.Status = iapiserver.LeaseStatusReleased
		if err := decrementWorkerRunning(tx, req.WorkerID); err != nil {
			return err
		}
		if err := tx.Save(attempt).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(lease).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Save(loadedRun).Error; err != nil {
			return errors.WithStack(err)
		}
		run = *loadedRun
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *taskCenterStore) RenewLease(
	ctx context.Context,
	req *iapiserver.LeaseRenewRequest,
) (*iapiserver.ExecutionLease, error) {
	var lease iapiserver.ExecutionLease
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loadedLease, _, _, err := loadActiveTaskLease(tx, req.RunID, req.AttemptID, req.LeaseID, req.WorkerID)
		if err != nil {
			return err
		}
		now := time.Now()
		loadedLease.Status = iapiserver.LeaseStatusRenewed
		loadedLease.RenewedAt = imachinery.NewTime(now)
		loadedLease.ExpireAt = imachinery.NewTime(now.Add(iapiserver.DefaultTaskCenterLeaseDuration))
		if err := tx.Save(loadedLease).Error; err != nil {
			return errors.WithStack(err)
		}
		lease = *loadedLease
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &lease, nil
}

func (s *taskCenterStore) Health(ctx context.Context) (*iapiserver.TaskCenterHealth, error) {
	ret := &iapiserver.TaskCenterHealth{
		SchedulerStatus: "ok",
		WatchdogStatus:  "not_started",
	}
	counts := []struct {
		model any
		where string
		args  []any
		dst   *int64
	}{
		{model: &iapiserver.Worker{}, where: "status = ?", args: []any{iapiserver.WorkerStatusOnline}, dst: &ret.WorkerOnlineTotal},
		{model: &iapiserver.Worker{}, where: "status = ?", args: []any{iapiserver.WorkerStatusLost}, dst: &ret.WorkerLostTotal},
		{model: &iapiserver.TaskRun{}, where: "status IN ?", args: []any{[]string{iapiserver.TaskRunStatusReady, iapiserver.TaskRunStatusPending}}, dst: &ret.QueueReadyTotal},
		{model: &iapiserver.TaskRun{}, where: "status = ?", args: []any{iapiserver.TaskRunStatusRunning}, dst: &ret.TaskRunningTotal},
		{model: &iapiserver.TaskRun{}, where: "status = ?", args: []any{iapiserver.TaskRunStatusRetrying}, dst: &ret.TaskRetryingTotal},
		{model: &iapiserver.ExecutionLease{}, where: "status = ? OR expire_at < ?", args: []any{iapiserver.LeaseStatusExpired, time.Now()}, dst: &ret.LeaseExpiredTotal},
	}
	for _, count := range counts {
		if err := s.ds.db.WithContext(ctx).Model(count.model).Where(count.where, count.args...).Count(count.dst).Error; err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return ret, nil
}

func (s *taskCenterStore) AddEvent(
	ctx context.Context,
	data *iapiserver.TaskRunEvent,
) (*iapiserver.TaskRunEvent, error) {
	if data.Name == "" {
		data.Name = data.EventType
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func fillTaskDefinitionDefaults(data *iapiserver.TaskDefinition) {
	if data.ProjectID == "" {
		data.ProjectID = iapiserver.DefaultTaskCenterProjectID
	}
	if data.Namespace == "" {
		data.Namespace = iapiserver.DefaultTaskCenterNamespace
	}
	if data.CreatedBy == "" {
		data.CreatedBy = iapiserver.DefaultTaskCenterCreatedBy
	}
	if data.Name == "" {
		data.Name = strings.ToLower(strings.ReplaceAll(data.DefinitionType, "_", "-"))
	}
}

func fillTaskRunDefaults(data *iapiserver.TaskRun) {
	if data.ProjectID == "" {
		data.ProjectID = iapiserver.DefaultTaskCenterProjectID
	}
	if data.Namespace == "" {
		data.Namespace = iapiserver.DefaultTaskCenterNamespace
	}
	if data.CreatedBy == "" {
		data.CreatedBy = iapiserver.DefaultTaskCenterCreatedBy
	}
	if data.Status == "" {
		data.Status = iapiserver.TaskRunStatusReady
	}
	if data.MaxAttempts == 0 {
		data.MaxAttempts = 1
	}
	if data.Name == "" {
		data.Name = strings.ToLower(strings.ReplaceAll(data.DefinitionType, "_", "-")) + "-run"
	}
	if data.Input == nil {
		data.Input = map[string]any{}
	}
	if data.Output == nil {
		data.Output = map[string]any{}
	}
}

func mapTaskDefinitionError(err error) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "task definition not found")
	}
	return errors.WithStack(err)
}

func mapTaskRunError(err error) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatusF(code.ErrTaskRunNotFound, "task run not found")
	}
	return errors.WithStack(err)
}

func mapWorkerError(err error) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatusF(code.ErrTaskWorkerNotAvailable, "worker not found")
	}
	return errors.WithStack(err)
}

func maxClaimScanLimit(maxCount int) int {
	if maxCount <= 0 {
		return 1
	}
	if maxCount > 50 {
		return 50
	}
	return maxCount
}

func getTaskDefinitionForUpdate(
	tx *gorm.DB,
	definitionType, id string,
) (*iapiserver.TaskDefinition, error) {
	var definition iapiserver.TaskDefinition
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND definition_type = ?", id, definitionType).
		First(&definition).Error; err != nil {
		return nil, mapTaskDefinitionError(err)
	}
	return &definition, nil
}

func capabilityMatches(workerCapabilities, requiredCapabilities string) bool {
	requiredCapabilities = strings.TrimSpace(requiredCapabilities)
	if requiredCapabilities == "" {
		return true
	}
	workerSet := splitCapabilitySet(workerCapabilities)
	for _, required := range strings.Split(requiredCapabilities, ",") {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		if !workerSet[required] {
			return false
		}
	}
	return true
}

func splitCapabilitySet(value string) map[string]bool {
	ret := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			ret[item] = true
		}
	}
	return ret
}

func loadActiveTaskLease(
	tx *gorm.DB,
	runID, attemptID, leaseID, workerID string,
) (*iapiserver.ExecutionLease, *iapiserver.TaskAttempt, *iapiserver.TaskRun, error) {
	var lease iapiserver.ExecutionLease
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND run_id = ? AND attempt_id = ? AND worker_id = ?", leaseID, runID, attemptID, workerID).
		First(&lease).Error; err != nil {
		return nil, nil, nil, mapLeaseError(err)
	}
	if lease.Status != iapiserver.LeaseStatusActive && lease.Status != iapiserver.LeaseStatusRenewed {
		return nil, nil, nil, errors.NewStatusF(code.ErrTaskLeaseInvalid, "task lease is not active")
	}
	if !lease.ExpireAt.IsZero() && lease.ExpireAt.Time.Before(time.Now()) {
		return nil, nil, nil, errors.NewStatusF(code.ErrTaskLeaseInvalid, "task lease expired")
	}
	var attempt iapiserver.TaskAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND run_id = ? AND worker_id = ?", attemptID, runID, workerID).
		First(&attempt).Error; err != nil {
		return nil, nil, nil, mapAttemptError(err)
	}
	var run iapiserver.TaskRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", runID).
		First(&run).Error; err != nil {
		return nil, nil, nil, mapTaskRunError(err)
	}
	return &lease, &attempt, &run, nil
}

func mapLeaseError(err error) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatusF(code.ErrTaskLeaseInvalid, "task lease invalid")
	}
	return errors.WithStack(err)
}

func mapAttemptError(err error) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatusF(code.ErrTaskAttemptUpdateRejected, "task attempt update rejected")
	}
	return errors.WithStack(err)
}

func decrementWorkerRunning(tx *gorm.DB, workerID string) error {
	var worker iapiserver.Worker
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", workerID).First(&worker).Error; err != nil {
		return mapWorkerError(err)
	}
	if worker.RunningCount > 0 {
		worker.RunningCount--
	}
	if worker.Status == iapiserver.WorkerStatusBusy && worker.RunningCount < worker.MaxConcurrency {
		worker.Status = iapiserver.WorkerStatusOnline
	}
	return errors.WithStack(tx.Save(&worker).Error)
}

func durationMS(start, end imachinery.Time) int64 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	return end.Sub(start.Time).Milliseconds()
}

func taskCenterListQuery(
	ctx context.Context,
	db *gorm.DB,
	params imachinery.BasicQueryParam,
	resourceSpecificFilter func(*gorm.DB) *gorm.DB,
) *gorm.DB {
	query := db.WithContext(ctx)
	if params.Keyword != "" {
		query = applyTaskCenterKeywordFilter(query, params)
	}
	if resourceSpecificFilter != nil {
		query = resourceSpecificFilter(query)
	}
	if params.CreatedAfter != 0 {
		query = query.Where(`"createdAt" >= ?`, time.Unix(params.CreatedAfter, 0))
	}
	if params.CreatedBefore != 0 {
		query = query.Where(`"createdAt" <= ?`, time.Unix(params.CreatedBefore, 0))
	}
	query = query.Order(taskCenterOrderBy(params))
	if params.PageNum > 0 && params.PageSize > 0 {
		pageSize := params.PageSize
		if pageSize > 1000 {
			pageSize = 1000
		}
		query = query.Offset((params.PageNum - 1) * pageSize).Limit(pageSize)
	}
	return query
}

func applyTaskCenterKeywordFilter(query *gorm.DB, params imachinery.BasicQueryParam) *gorm.DB {
	allowedFields := map[string]string{
		"id":              "id",
		"name":            "name",
		"description":     "description",
		"definition_type": "definition_type",
		"status":          "status",
		"project_id":      "project_id",
		"namespace":       "namespace",
	}
	fields := params.SearchFields
	if len(fields) == 0 {
		fields = []string{"name", "description"}
	}
	conditions := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields))
	for _, field := range fields {
		column, ok := allowedFields[field]
		if !ok {
			continue
		}
		conditions = append(conditions, column+" LIKE ?")
		args = append(args, "%"+params.Keyword+"%")
	}
	if len(conditions) == 0 {
		return query
	}
	return query.Where(strings.Join(conditions, " OR "), args...)
}

func taskCenterOrderBy(params imachinery.BasicQueryParam) clause.OrderByColumn {
	allowedFields := map[string]string{
		"id":              "id",
		"name":            "name",
		"definition_type": "definition_type",
		"status":          "status",
		"project_id":      "project_id",
		"namespace":       "namespace",
		"createdAt":       taskCenterCreatedAtColumn,
		"created_at":      taskCenterCreatedAtColumn,
		"updatedAt":       taskCenterUpdatedAtColumn,
		"updated_at":      taskCenterUpdatedAtColumn,
	}
	column := taskCenterCreatedAtColumn
	if mapped, ok := allowedFields[params.SortField]; ok {
		column = mapped
	}
	return clause.OrderByColumn{
		Column: clause.Column{Name: column},
		Desc:   strings.ToLower(params.SortOrder) != "asc",
	}
}

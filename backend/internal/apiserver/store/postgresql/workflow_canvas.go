package postgresql

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type workflowCanvasStore struct{ ds *datastore }

func newWorkflowCanvasStore(ds *datastore) *workflowCanvasStore { return &workflowCanvasStore{ds: ds} }

func (s *workflowCanvasStore) ListWorkflowNodeDefinitions(
	ctx context.Context,
	req *iapiserver.WorkflowNodeDefinitionListRequest,
	projectID, namespace string,
) ([]*iapiserver.WorkflowNodeDefinition, int64, error) {
	var items []*iapiserver.WorkflowNodeDefinition
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where(
			"availability_scope = ? OR (availability_scope = ? AND project_id = ? AND namespace = ?)",
			iapiserver.CanvasAvailabilitySystem,
			iapiserver.CanvasAvailabilityProject,
			projectID,
			namespace,
		)
		// promptGroup is an internal normalization container. It remains addressable
		// for draft/version validation, but must not consume catalog pages or totals.
		q = q.Where("node_type <> ?", "promptGroup")
		if req.Category != "" {
			q = q.Where("category = ?", req.Category)
		}
		if req.NodeKind != "" {
			q = q.Where("node_kind = ?", req.NodeKind)
		}
		if req.ExecutionMode != "" {
			q = q.Where("execution_mode = ?", req.ExecutionMode)
		}
		if !req.IncludeDeprecated {
			q = q.Where("deprecated = FALSE")
		}
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.WorkflowNodeDefinition{}), filter).
		Order("category ASC, node_type ASC, definition_version DESC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) GetWorkflowNodeDefinition(
	ctx context.Context,
	nodeType, definitionVersion, projectID, namespace string,
	includeDeprecated bool,
) (*iapiserver.WorkflowNodeDefinition, error) {
	var item iapiserver.WorkflowNodeDefinition
	query := s.ds.db.WithContext(ctx).Where("node_type = ? AND definition_version = ?", nodeType, definitionVersion).
		Where("availability_scope = ? OR (availability_scope = ? AND project_id = ? AND namespace = ?)", iapiserver.CanvasAvailabilitySystem, iapiserver.CanvasAvailabilityProject, projectID, namespace)
	if !includeDeprecated {
		query = query.Where("deprecated = FALSE")
	}
	if err := query.First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrWorkflowNodeDefinitionNotFound, "workflow node definition not found")
	}
	return &item, nil
}

func (s *workflowCanvasStore) AddWorkflowNodeDefinitionIdempotent(
	ctx context.Context,
	data *iapiserver.WorkflowNodeDefinition,
) (*iapiserver.WorkflowNodeDefinition, bool, error) {
	var result *iapiserver.WorkflowNodeDefinition
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.WorkflowNodeDefinition
		err := tx.Where("node_type = ? AND definition_version = ?", data.NodeType, data.DefinitionVersion).First(&existing).Error
		if err == nil {
			if mismatch := workflowNodeDefinitionMismatch(&existing, data); mismatch != "" {
				return errors.NewStatusF(code.ErrWorkflowNodeDefinitionConflict, "workflow node definition content differs: %s", mismatch)
			}
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return errors.WithStack(err)
		}
		create := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "node_type"}, {Name: "definition_version"}},
			DoNothing: true,
		}).Create(data)
		if create.Error != nil {
			return errors.WithStack(create.Error)
		}
		if create.RowsAffected == 0 {
			if err := tx.Where("node_type = ? AND definition_version = ?", data.NodeType, data.DefinitionVersion).First(&existing).Error; err != nil {
				return errors.WithStack(err)
			}
			if mismatch := workflowNodeDefinitionMismatch(&existing, data); mismatch != "" {
				return errors.NewStatusF(code.ErrWorkflowNodeDefinitionConflict, "workflow node definition content differs: %s", mismatch)
			}
			result = &existing
			return nil
		}
		result, created = data, true
		return nil
	})
	return result, created, err
}

func workflowNodeDefinitionMismatch(existing, expected *iapiserver.WorkflowNodeDefinition) string {
	checks := []struct {
		name string
		ok   bool
	}{
		{"title", existing.Title == expected.Title},
		{"description", existing.Description == expected.Description},
		{"category", existing.Category == expected.Category},
		{"node_kind", existing.NodeKind == expected.NodeKind},
		{"ports", workflowJSONEqual(existing.Ports, expected.Ports)},
		{"config_schema", workflowJSONEqual(existing.ConfigSchema, expected.ConfigSchema)},
		{"controller_state_schema", workflowJSONEqual(existing.ControllerStateSchema, expected.ControllerStateSchema)},
		{"controller_schema_version", reflect.DeepEqual(existing.ControllerSchemaVersion, expected.ControllerSchemaVersion)},
		{"execution_binding", reflect.DeepEqual(existing.ExecutionBinding, expected.ExecutionBinding)},
		{"renderer", reflect.DeepEqual(existing.Renderer, expected.Renderer)},
		{"cache_allowed", existing.CacheAllowed == expected.CacheAllowed},
		{"reuse_ttl_seconds", reflect.DeepEqual(existing.ReuseTTLSeconds, expected.ReuseTTLSeconds)},
		{"availability_scope", existing.AvailabilityScope == expected.AvailabilityScope},
		{"project_id", reflect.DeepEqual(existing.ProjectID, expected.ProjectID)},
		{"namespace", reflect.DeepEqual(existing.Namespace, expected.Namespace)},
	}
	for _, check := range checks {
		if !check.ok {
			return check.name
		}
	}
	return ""
}

// workflowJSONEqual compares persisted JSON values by their wire representation.
// PostgreSQL JSON decoding normalizes numbers to float64, so Go's reflect.DeepEqual
// would reject semantically identical schemas after a restart.
func workflowJSONEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func (s *workflowCanvasStore) DeprecateWorkflowNodeDefinition(
	ctx context.Context,
	nodeType, definitionVersion, userID string,
) (*iapiserver.WorkflowNodeDefinition, error) {
	now := time.Now()
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.WorkflowNodeDefinition{}).
		Where("node_type = ? AND definition_version = ?", nodeType, definitionVersion).
		Updates(map[string]any{"deprecated": true, "deprecated_at": now, "updated_at": now})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.NewStatus(code.ErrWorkflowNodeDefinitionNotFound, "workflow node definition not found")
	}
	var item iapiserver.WorkflowNodeDefinition
	if err := s.ds.db.WithContext(ctx).Where("node_type = ? AND definition_version = ?", nodeType, definitionVersion).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *workflowCanvasStore) ListWorkflowCanvases(
	ctx context.Context,
	req *iapiserver.WorkflowCanvasListRequest,
	projectID, namespace, userID string,
) ([]*iapiserver.WorkflowCanvas, int64, error) {
	var items []*iapiserver.WorkflowCanvas
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where(
			"project_id = ? AND namespace = ? AND (created_by = ? OR visibility = ?) AND deleted_at IS NULL",
			projectID,
			namespace,
			userID,
			iapiserver.CanvasVisibilityProject,
		)
		if req.Visibility != "" {
			q = q.Where("visibility = ?", req.Visibility)
		}
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.WorkflowCanvas{}), filter).Order("updated_at DESC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) GetWorkflowCanvas(ctx context.Context, id string) (*iapiserver.WorkflowCanvas, error) {
	var item iapiserver.WorkflowCanvas
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrCanvasNotFound, "canvas not found")
	}
	return &item, nil
}

func (s *workflowCanvasStore) GetWorkflowCanvasesByIDs(ctx context.Context, ids []string) ([]*iapiserver.WorkflowCanvas, error) {
	if len(ids) == 0 {
		return []*iapiserver.WorkflowCanvas{}, nil
	}
	var items []*iapiserver.WorkflowCanvas
	err := s.ds.db.WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ids).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *workflowCanvasStore) AddWorkflowCanvas(ctx context.Context, data *iapiserver.WorkflowCanvas) (*iapiserver.WorkflowCanvas, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *workflowCanvasStore) UpdateWorkflowCanvas(ctx context.Context, data *iapiserver.WorkflowCanvas, expected int64) (*iapiserver.WorkflowCanvas, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.WorkflowCanvas{}).
		Where("id = ? AND draft_revision = ? AND deleted_at IS NULL", data.ID, expected).
		Select("name", "description", "visibility", "draft_graph_json", "draft_revision", "updated_at", "resource_version").
		Updates(data)
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
	}
	return s.GetWorkflowCanvas(ctx, data.ID)
}

func (s *workflowCanvasStore) DeleteWorkflowCanvas(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.WorkflowCanvas{}).Where("id = ? AND deleted_at IS NULL", id).Update("deleted_at", time.Now())
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.NewStatus(code.ErrCanvasNotFound, "canvas not found")
	}
	return nil
}

func (s *workflowCanvasStore) PublishWorkflowCanvas(
	ctx context.Context,
	canvas *iapiserver.WorkflowCanvas,
	version *iapiserver.CanvasVersion,
	expected int64,
) (*iapiserver.CanvasVersion, error) {
	var published *iapiserver.CanvasVersion
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked iapiserver.WorkflowCanvas
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND deleted_at IS NULL", canvas.ID).First(&locked).Error; err != nil {
			return mapNotFound(err, code.ErrCanvasNotFound, "canvas not found")
		}
		if locked.DraftRevision != expected {
			return errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
		}
		var existing iapiserver.CanvasVersion
		if err := tx.Where("canvas_id = ? AND content_digest = ?", canvas.ID, version.ContentDigest).First(&existing).Error; err == nil {
			published = &existing
			return nil
		} else if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return errors.WithStack(err)
		}
		version.Version = locked.LatestVersion + 1
		if err := tx.Create(version).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := publishCanvasVersion(tx, version, &locked); err != nil {
			return err
		}
		published = version
		return tx.Model(&locked).
			Updates(map[string]any{"latest_version": version.Version, "latest_published_version_id": version.ID, "updated_at": time.Now()}).
			Error
	})
	if err != nil {
		return nil, err
	}
	return published, nil
}

func (s *workflowCanvasStore) ListCanvasVersions(ctx context.Context, req *iapiserver.CanvasVersionListRequest) ([]*iapiserver.CanvasVersion, int64, error) {
	var items []*iapiserver.CanvasVersion
	filter := func(q *gorm.DB) *gorm.DB { return q.Where("canvas_id = ?", req.CanvasID) }
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.CanvasVersion{}), filter).Order("version DESC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) GetCanvasVersion(ctx context.Context, id string) (*iapiserver.CanvasVersion, error) {
	var item iapiserver.CanvasVersion
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrCanvasVersionNotFound, "canvas version not found")
	}
	return &item, nil
}

func (s *workflowCanvasStore) GetCanvasVersionsByIDs(ctx context.Context, ids []string) ([]*iapiserver.CanvasVersion, error) {
	if len(ids) == 0 {
		return []*iapiserver.CanvasVersion{}, nil
	}
	var items []*iapiserver.CanvasVersion
	err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *workflowCanvasStore) ListWorkflowCanvasRuns(
	ctx context.Context,
	req *iapiserver.WorkflowCanvasRunListRequest,
	projectID, namespace, userID string,
) ([]*iapiserver.WorkflowCanvasRun, int64, error) {
	var items []*iapiserver.WorkflowCanvasRun
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("project_id = ? AND namespace = ? AND created_by = ?", projectID, namespace, userID)
		if req.CanvasID != "" {
			q = q.Where("canvas_id = ?", req.CanvasID)
		}
		if req.CanvasVersionID != "" {
			q = q.Where("canvas_version_id = ?", req.CanvasVersionID)
		}
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		if req.RetryOfCanvasRunID != "" {
			q = q.Where("retry_of_canvas_run_id = ?", req.RetryOfCanvasRunID)
		}
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.WorkflowCanvasRun{}), filter).Order("created_at DESC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) GetWorkflowCanvasRun(ctx context.Context, id string) (*iapiserver.WorkflowCanvasRun, error) {
	var item iapiserver.WorkflowCanvasRun
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrCanvasRunNotFound, "canvas run not found")
	}
	return &item, nil
}

func (s *workflowCanvasStore) GetWorkflowCanvasRunsByIDs(ctx context.Context, ids []string) ([]*iapiserver.WorkflowCanvasRun, error) {
	if len(ids) == 0 {
		return []*iapiserver.WorkflowCanvasRun{}, nil
	}
	var items []*iapiserver.WorkflowCanvasRun
	err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *workflowCanvasStore) AddWorkflowCanvasRunIdempotent(
	ctx context.Context,
	data *iapiserver.WorkflowCanvasRun,
) (*iapiserver.WorkflowCanvasRun, bool, error) {
	var result *iapiserver.WorkflowCanvasRun
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.WorkflowCanvasRun
		err := tx.Where("project_id = ? AND namespace = ? AND created_by = ? AND idempotency_key = ?", data.ProjectID, data.Namespace, data.CreatedBy, data.IdempotencyKey).
			First(&existing).
			Error
		if err == nil {
			if existing.RequestDigest != data.RequestDigest {
				return errors.NewStatus(code.ErrCanvasRunIdempotencyConflict, "canvas run idempotency request differs")
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
		if err := publishCanvasRunCreated(tx, data); err != nil {
			return err
		}
		if err := publishCanvasRunRetryCreated(tx, data); err != nil {
			return err
		}
		result = data
		created = true
		return nil
	})
	return result, created, err
}

func (s *workflowCanvasStore) BindWorkflowCanvasRun(
	ctx context.Context,
	id, groupID string,
	flows []*iapiserver.CanvasFlowRun,
	nodes []*iapiserver.CanvasNodeRun,
	taskBindings []*iapiserver.CanvasNodeRunTaskBinding,
	outputBindings []*iapiserver.CanvasNodeRunOutputBinding,
) (*iapiserver.WorkflowCanvasRun, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&iapiserver.WorkflowCanvasRun{}).Where("id = ? AND task_creation_status IN ?", id, []string{iapiserver.CanvasTaskCreationPending, iapiserver.CanvasTaskCreationFailed}).Updates(map[string]any{"dag_task_group_id": groupID, "task_creation_status": iapiserver.CanvasTaskCreationCreated, "task_creation_attempts": gorm.Expr("task_creation_attempts + 1"), "status": iapiserver.CanvasRunStatusRunning, "aggregate_version": gorm.Expr("aggregate_version + 1"), "started_at": time.Now()}).Error; err != nil {
			return err
		}
		for _, flow := range flows {
			if err := tx.Create(flow).Error; err != nil {
				return err
			}
		}
		for _, node := range nodes {
			if err := tx.Create(node).Error; err != nil {
				return err
			}
		}
		nodeByExecutionKey := make(map[string]string, len(nodes))
		for _, node := range nodes {
			nodeByExecutionKey[node.ExecutionKey] = node.ID
		}
		for _, flow := range flows {
			for _, executionKey := range flow.ExecutionKeys {
				nodeID, ok := nodeByExecutionKey[executionKey]
				if !ok {
					continue
				}
				ref := &iapiserver.CanvasNodeRunFlowRef{CanvasNodeRunID: nodeID, CanvasFlowRunID: flow.ID}
				ref.ID = uuid.NewString()
				ref.Name = flow.FlowID + ":" + executionKey
				if err := tx.Create(ref).Error; err != nil {
					return err
				}
			}
		}
		for _, binding := range taskBindings {
			if err := tx.Create(binding).Error; err != nil {
				return err
			}
		}
		for _, binding := range outputBindings {
			if err := tx.Create(binding).Error; err != nil {
				return err
			}
		}
		// DAG 可在 Canvas 投影绑定提交前开始执行；绑定完成时用 Task Center 当前事实修复该崩溃窗口。
		boundNodeIDs := make(map[string]struct{}, len(taskBindings))
		boundTaskIDs := make([]string, 0, len(taskBindings))
		for _, binding := range taskBindings {
			boundNodeIDs[binding.CanvasNodeRunID] = struct{}{}
			boundTaskIDs = append(boundTaskIDs, binding.AtomicTaskID)
		}
		if len(boundTaskIDs) > 0 {
			var lockedTasks []*iapiserver.AtomicTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id IN ?", boundTaskIDs).
				Find(&lockedTasks).Error; err != nil {
				return err
			}
		}
		for nodeID := range boundNodeIDs {
			if err := recalculateCanvasNode(tx, nodeID); err != nil {
				return err
			}
		}
		var bound iapiserver.WorkflowCanvasRun
		if err := tx.Where("id = ?", id).First(&bound).Error; err != nil {
			return err
		}
		if err := publishCanvasRunBound(tx, &bound, len(taskBindings)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetWorkflowCanvasRun(ctx, id)
}
func (s *workflowCanvasStore) UpdateWorkflowCanvasRun(ctx context.Context, data *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.WorkflowCanvasRun
		if err := tx.Where("id = ?", data.ID).First(&previous).Error; err != nil {
			return err
		}
		data.AggregateVersion = previous.AggregateVersion + 1
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		if err := publishCanvasRunChanged(tx, previous.Status, data); err != nil {
			return err
		}
		if data.Status == iapiserver.CanvasRunStatusCanceled && previous.Status != data.Status {
			return publishCanvasRunCancelRequested(tx, data)
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}
func (s *workflowCanvasStore) ListCanvasNodeRuns(ctx context.Context, req *iapiserver.CanvasNodeRunListRequest) ([]*iapiserver.CanvasNodeRun, int64, error) {
	var items []*iapiserver.CanvasNodeRun
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("canvas_run_id = ?", req.CanvasRunID)
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		if req.NodeID != "" {
			q = q.Where("node_id = ?", req.NodeID)
		}
		if req.FlowID != "" {
			q = q.Where(
				"EXISTS (SELECT 1 FROM canvas_node_run_flow_refs refs JOIN canvas_flow_runs flows ON flows.id = refs.canvas_flow_run_id WHERE refs.canvas_node_run_id = canvas_node_runs.id AND flows.flow_id = ?)",
				req.FlowID,
			)
		}
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.CanvasNodeRun{}), filter).Order("node_id ASC, execution_key ASC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) ListCanvasFlowRuns(ctx context.Context, req *iapiserver.CanvasFlowRunListRequest) ([]*iapiserver.CanvasFlowRun, int64, error) {
	var items []*iapiserver.CanvasFlowRun
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("canvas_run_id = ?", req.CanvasRunID)
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.CanvasFlowRun{}), filter).Order("flow_id ASC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *workflowCanvasStore) GetCanvasNodeRun(ctx context.Context, id string) (*iapiserver.CanvasNodeRun, error) {
	var item iapiserver.CanvasNodeRun
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrCanvasNodeRunNotFound, "canvas node run not found")
	}
	return &item, nil
}

func (s *workflowCanvasStore) GetCanvasNodeRunDetail(
	ctx context.Context,
	id string,
) ([]*iapiserver.CanvasNodeRunTaskBinding, []*iapiserver.CanvasNodeRunOutputBinding, error) {
	var tasks []*iapiserver.CanvasNodeRunTaskBinding
	var outputs []*iapiserver.CanvasNodeRunOutputBinding
	if err := s.ds.db.WithContext(ctx).Where("canvas_node_run_id = ?", id).Order("binding_role ASC, shard_index ASC").Find(&tasks).Error; err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if err := s.ds.db.WithContext(ctx).Where("canvas_node_run_id = ?", id).Order("port_key ASC, shard_index ASC").Find(&outputs).Error; err != nil {
		return nil, nil, errors.WithStack(err)
	}
	return tasks, outputs, nil
}

// ProjectCanvasApplicationArtifact 按 AtomicTask、输出键和序号单调更新 Canvas 输出事实。
func (s *workflowCanvasStore) ProjectCanvasApplicationArtifact(
	ctx context.Context,
	projection *store.CanvasApplicationArtifactProjection,
) (bool, error) {
	if projection == nil || projection.AtomicTaskID == "" || projection.OutputKey == "" ||
		projection.Sequence < 0 || projection.ArtifactID == "" || projection.ArtifactResourceVersion < 1 {
		return false, errors.New("canvas application artifact projection is incomplete")
	}
	applied := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskBinding iapiserver.CanvasNodeRunTaskBinding
		if err := tx.Where("atomic_task_id = ?", projection.AtomicTaskID).First(&taskBinding).Error; err != nil {
			return err
		}
		shardKey := "root"
		if projection.Sequence > 0 {
			shardKey = fmt.Sprintf("sequence:%d", projection.Sequence)
		}
		var binding iapiserver.CanvasNodeRunOutputBinding
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("canvas_node_run_id = ? AND port_key = ? AND shard_key = ?", taskBinding.CanvasNodeRunID, projection.OutputKey, shardKey).
			First(&binding).Error
		if stderrors.Is(err, gorm.ErrRecordNotFound) && projection.Sequence > 0 {
			var root iapiserver.CanvasNodeRunOutputBinding
			if err := tx.Where("canvas_node_run_id = ? AND port_key = ? AND shard_key = ?", taskBinding.CanvasNodeRunID, projection.OutputKey, "root").First(&root).Error; err != nil {
				return err
			}
			index := projection.Sequence
			binding = iapiserver.CanvasNodeRunOutputBinding{
				CanvasNodeRunID:    root.CanvasNodeRunID,
				PortKey:            root.PortKey,
				Required:           false,
				ShardKey:           shardKey,
				ShardIndex:         &index,
				ProducerKey:        fmt.Sprintf("%s:sequence:%d", root.ProducerKey, projection.Sequence),
				AvailabilityStatus: "PENDING",
			}
			binding.ID = uuid.NewString()
			binding.Name = fmt.Sprintf("%s[%d]", root.Name, projection.Sequence)
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("canvas_node_run_id = ? AND port_key = ? AND shard_key = ?", taskBinding.CanvasNodeRunID, projection.OutputKey, shardKey).
				First(&binding).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if binding.ArtifactResourceVersion >= projection.ArtifactResourceVersion {
			return nil
		}
		previousStatus := binding.AvailabilityStatus
		previousArtifactID := binding.ArtifactID
		availabilityStatus := "PENDING"
		switch projection.ArtifactProcessingStatus {
		case iapiserver.ArtifactProcessingReady:
			availabilityStatus = "READY"
		case iapiserver.ArtifactProcessingFailed:
			availabilityStatus = "FAILED"
		}
		binding.AtomicTaskID = &projection.AtomicTaskID
		binding.ArtifactID = &projection.ArtifactID
		binding.AvailabilityStatus = availabilityStatus
		binding.ArtifactResourceVersion = projection.ArtifactResourceVersion
		binding.AggregateVersion++
		if err := tx.Save(&binding).Error; err != nil {
			return err
		}
		applied = true
		newReadyArtifact := availabilityStatus == "READY" &&
			(previousStatus != "READY" || previousArtifactID == nil || *previousArtifactID != projection.ArtifactID)
		if newReadyArtifact {
			if err := publishCanvasNodeOutputAvailable(tx, &binding, projection.MediaType); err != nil {
				return err
			}
		}
		return recalculateCanvasNode(tx, binding.CanvasNodeRunID)
	})
	return applied, errors.WithStack(err)
}

var _ storeWorkflowCanvasContract = (*workflowCanvasStore)(nil)

type storeWorkflowCanvasContract interface {
	GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error)
}

var _ = imachinery.BasicQueryParam{}

// ensureWorkflowCanvasScheme creates constraints not expressed by the GORM models.
func (ds *datastore) ensureWorkflowCanvasScheme() error {
	return ds.db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_canvas_node_run_execution ON canvas_node_runs(canvas_run_id,execution_key);
`).Error
}

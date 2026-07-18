package postgresql

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type workflowCanvasStore struct{ ds *datastore }

func newWorkflowCanvasStore(ds *datastore) *workflowCanvasStore { return &workflowCanvasStore{ds: ds} }

func (s *workflowCanvasStore) ListWorkflowCanvases(ctx context.Context, req *iapiserver.WorkflowCanvasListRequest, projectID, namespace, userID string) ([]*iapiserver.WorkflowCanvas, int64, error) {
	var items []*iapiserver.WorkflowCanvas
	filter := func(q *gorm.DB) *gorm.DB {
		return q.Where("project_id = ? AND namespace = ? AND created_by = ? AND deleted_at IS NULL", projectID, namespace, userID)
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

func (s *workflowCanvasStore) AddWorkflowCanvas(ctx context.Context, data *iapiserver.WorkflowCanvas) (*iapiserver.WorkflowCanvas, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *workflowCanvasStore) UpdateWorkflowCanvas(ctx context.Context, data *iapiserver.WorkflowCanvas, expected int64) (*iapiserver.WorkflowCanvas, error) {
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.WorkflowCanvas{}).Where("id = ? AND draft_revision = ? AND deleted_at IS NULL", data.ID, expected).Select("name", "description", "visibility", "draft_graph_json", "draft_revision", "updated_at", "resource_version").Updates(data)
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

func (s *workflowCanvasStore) PublishWorkflowCanvas(ctx context.Context, canvas *iapiserver.WorkflowCanvas, version *iapiserver.CanvasVersion, expected int64) (*iapiserver.CanvasVersion, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked iapiserver.WorkflowCanvas
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND deleted_at IS NULL", canvas.ID).First(&locked).Error; err != nil {
			return mapNotFound(err, code.ErrCanvasNotFound, "canvas not found")
		}
		if locked.DraftRevision != expected {
			return errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
		}
		version.Version = locked.LatestVersion + 1
		if err := tx.Create(version).Error; err != nil {
			return errors.WithStack(err)
		}
		return tx.Model(&locked).Updates(map[string]any{"latest_version": version.Version, "updated_at": time.Now()}).Error
	})
	if err != nil {
		return nil, err
	}
	return version, nil
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

func (s *workflowCanvasStore) ListWorkflowCanvasRuns(ctx context.Context, req *iapiserver.WorkflowCanvasRunListRequest, projectID, namespace, userID string) ([]*iapiserver.WorkflowCanvasRun, int64, error) {
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

func (s *workflowCanvasStore) AddWorkflowCanvasRunIdempotent(ctx context.Context, data *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, bool, error) {
	var result *iapiserver.WorkflowCanvasRun
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.WorkflowCanvasRun
		err := tx.Where("project_id = ? AND namespace = ? AND created_by = ? AND idempotency_key = ?", data.ProjectID, data.Namespace, data.CreatedBy, data.IdempotencyKey).First(&existing).Error
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
		result = data
		created = true
		return nil
	})
	return result, created, err
}

func (s *workflowCanvasStore) BindWorkflowCanvasRun(ctx context.Context, id, groupID string, nodes []*iapiserver.CanvasNodeRun) (*iapiserver.WorkflowCanvasRun, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&iapiserver.WorkflowCanvasRun{}).Where("id = ? AND task_creation_status = ?", id, iapiserver.CanvasTaskCreationPending).Updates(map[string]any{"dag_task_group_id": groupID, "task_creation_status": iapiserver.CanvasTaskCreationCreated, "status": iapiserver.CanvasRunStatusRunning}).Error; err != nil {
			return err
		}
		for _, node := range nodes {
			if err := tx.Create(node).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetWorkflowCanvasRun(ctx, id)
}
func (s *workflowCanvasStore) UpdateWorkflowCanvasRun(ctx context.Context, data *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
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
		return q
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.CanvasNodeRun{}), filter).Order("node_key ASC")
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

var _ storeWorkflowCanvasContract = (*workflowCanvasStore)(nil)

type storeWorkflowCanvasContract interface {
	GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error)
}

var _ = imachinery.BasicQueryParam{}

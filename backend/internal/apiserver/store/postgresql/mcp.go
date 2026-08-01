package postgresql

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type mcpTaskBindingStore struct{ ds *datastore }

func newMCPTaskBindingStore(ds *datastore) *mcpTaskBindingStore {
	return &mcpTaskBindingStore{ds: ds}
}

// CreateOrGet 依赖 principal+run 唯一约束，在并发幂等请求中返回同一映射。
func (s *mcpTaskBindingStore) CreateOrGet(
	ctx context.Context,
	data *iapiserver.MCPTaskBinding,
) (*iapiserver.MCPTaskBinding, bool, error) {
	var result *iapiserver.MCPTaskBinding
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.MCPTaskBinding
		err := tx.Where("principal_id = ? AND application_run_id = ?", data.PrincipalID, data.ApplicationRunID).
			First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		result, created = data, true
		return nil
	})
	if err != nil {
		var existing iapiserver.MCPTaskBinding
		lookupErr := s.ds.db.WithContext(ctx).
			Where("principal_id = ? AND application_run_id = ?", data.PrincipalID, data.ApplicationRunID).
			First(&existing).Error
		if lookupErr == nil {
			return &existing, false, nil
		}
		return nil, false, errors.WithStack(err)
	}
	return result, created, nil
}

func (s *mcpTaskBindingStore) GetByTaskID(ctx context.Context, id string) (*iapiserver.MCPTaskBinding, error) {
	var item iapiserver.MCPTaskBinding
	if err := s.ds.db.WithContext(ctx).First(&item, "mcp_task_id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *mcpTaskBindingStore) GetByPrincipalRun(
	ctx context.Context,
	principalID, applicationRunID string,
) (*iapiserver.MCPTaskBinding, error) {
	var item iapiserver.MCPTaskBinding
	if err := s.ds.db.WithContext(ctx).
		First(&item, "principal_id = ? AND application_run_id = ?", principalID, applicationRunID).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *mcpTaskBindingStore) Touch(ctx context.Context, id string, at time.Time) error {
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.MCPTaskBinding{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_accessed_at": imachinery.NewTime(at),
			"resource_version": gorm.Expr("resource_version + 1"),
			"updated_at":       imachinery.NewTime(at),
		})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.WithStack(gorm.ErrRecordNotFound)
	}
	return nil
}

func (s *mcpTaskBindingStore) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result := s.ds.db.WithContext(ctx).
		Where("expires_at <= ?", before).
		Delete(&iapiserver.MCPTaskBinding{})
	return result.RowsAffected, errors.WithStack(result.Error)
}

var _ store.MCPTaskBindingStore = (*mcpTaskBindingStore)(nil)

package postgresql

import (
	"context"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type canvas struct {
	ds *datastore
}

func newCanvas(ds *datastore) *canvas {
	return &canvas{ds}
}

func (s *canvas) List(ctx context.Context, includeDeleted bool) ([]*iapiserver.Canvas, error) {
	var list []*iapiserver.Canvas
	q := s.ds.db.WithContext(ctx)
	if includeDeleted {
		q = q.Where("deleted_at > 0")
	} else {
		q = q.Where("deleted_at = 0")
	}
	err := q.Order("pinned DESC, updated_at DESC").Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *canvas) ListByProject(ctx context.Context, projectID string) ([]*iapiserver.Canvas, error) {
	var list []*iapiserver.Canvas
	err := s.ds.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at = 0", projectID).
		Order("pinned DESC, updated_at DESC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *canvas) Get(ctx context.Context, id string) (*iapiserver.Canvas, error) {
	var meta iapiserver.Canvas
	err := s.ds.db.WithContext(ctx).
		Where("id = ? AND deleted_at = 0", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *canvas) GetAny(ctx context.Context, id string) (*iapiserver.Canvas, error) {
	var meta iapiserver.Canvas
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *canvas) Add(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error) {
	err := s.ds.db.WithContext(ctx).Create(data).Error
	return data, errors.WithStack(err)
}

func (s *canvas) Update(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Canvas{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"title":         data.Title,
			"icon":          data.Icon,
			"kind":          data.Kind,
			"owner":         data.Owner,
			"color":         data.Color,
			"pinned":        data.Pinned,
			"project_id":    data.ProjectID,
			"board_x":       data.BoardX,
			"board_y":       data.BoardY,
			"extend_shadow": data.Extend.String(),
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("canvas not found with id %v", data.ID)
	}
	return data, nil
}

func (s *canvas) SoftDelete(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Canvas{}).
		Where("id = ? AND deleted_at = 0", id).
		Update("deleted_at", time.Now().UnixMilli())
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("canvas not found with id %v", id)
	}
	return nil
}

func (s *canvas) Restore(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Canvas{}).
		Where("id = ? AND deleted_at > 0", id).
		Update("deleted_at", 0)
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("canvas not found in trash with id %v", id)
	}
	return nil
}

func (s *canvas) Purge(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&iapiserver.Canvas{})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("canvas not found with id %v", id)
	}
	return nil
}

func (s *canvas) CountByProject(ctx context.Context, projectID string) (int, error) {
	var count int64
	err := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Canvas{}).
		Where("project_id = ? AND deleted_at = 0", projectID).
		Count(&count).Error
	return int(count), errors.WithStack(err)
}

func (s *canvas) ReassignProject(ctx context.Context, oldProjectID, newProjectID string) (int, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Canvas{}).
		Where("project_id = ?", oldProjectID).
		Update("project_id", newProjectID)
	if result.Error != nil {
		return 0, errors.WithStack(result.Error)
	}
	return int(result.RowsAffected), nil
}

func (s *canvas) CleanupExpiredTrash(ctx context.Context, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()
	return s.ds.db.WithContext(ctx).
		Where("deleted_at > 0 AND deleted_at < ?", cutoff).
		Delete(&iapiserver.Canvas{}).Error
}

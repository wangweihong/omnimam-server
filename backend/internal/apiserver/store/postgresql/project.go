package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type project struct {
	ds *datastore
}

func newProject(ds *datastore) *project {
	return &project{ds}
}

func (s *project) List(ctx context.Context) ([]*iapiserver.Project, error) {
	var list []*iapiserver.Project
	err := s.ds.db.WithContext(ctx).
		Order("sort_order ASC, created_at ASC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *project) Get(ctx context.Context, id string) (*iapiserver.Project, error) {
	var meta iapiserver.Project
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *project) Add(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error) {
	err := s.ds.db.WithContext(ctx).Create(data).Error
	return data, errors.WithStack(err)
}

func (s *project) Update(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Project{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"name":       data.Name,
			"sort_order": data.SortOrder,
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("project not found with id %v", data.ID)
	}
	return data, nil
}

func (s *project) Delete(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&iapiserver.Project{})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("project not found with id %v", id)
	}
	return nil
}

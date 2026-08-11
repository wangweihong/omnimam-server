package postgresql

import (
	"context"
	stderrors "errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type gitLabStore struct{ ds *datastore }

func newGitLabStore(ds *datastore) *gitLabStore { return &gitLabStore{ds: ds} }

func (s *gitLabStore) ListGitLabServers(ctx context.Context, req *iapiserver.GitLabServerListRequest) ([]*iapiserver.GitLabServer, int64, error) {
	var items []*iapiserver.GitLabServer
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.GitLabServer{}), func(query *gorm.DB) *gorm.DB {
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *gitLabStore) GetGitLabServer(ctx context.Context, id string) (*iapiserver.GitLabServer, error) {
	var item iapiserver.GitLabServer
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *gitLabStore) CreateGitLabServer(ctx context.Context, item *iapiserver.GitLabServer) (*iapiserver.GitLabServer, error) {
	// Credential 参与 INSERT 参数，关闭本次 session 的 SQL 日志以避免错误路径插值泄露 PAT。
	if err := s.ds.db.Session(&gorm.Session{Logger: logger.Discard}).WithContext(ctx).Create(item).Error; err != nil {
		if isGitLabUniqueError(err) {
			return nil, store.ErrGitLabServerNameConflict
		}
		return nil, err
	}
	return item, nil
}

func (s *gitLabStore) UpdateGitLabServer(ctx context.Context, desired *iapiserver.GitLabServer, expectedVersion int64) (*iapiserver.GitLabServer, error) {
	var updated iapiserver.GitLabServer
	// Credential 参与 UPDATE 参数，整个事务禁止 SQL 参数日志。
	err := s.ds.db.Session(&gorm.Session{Logger: logger.Discard}).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", desired.ID).First(&updated).Error; err != nil {
			return err
		}
		if expectedVersion > 0 && updated.ResourceVersion != expectedVersion {
			return store.ErrGitLabResourceVersionConflict
		}
		updated.Name = desired.Name
		updated.Description = desired.Description
		updated.APIURL = desired.APIURL
		updated.ExternalURL = desired.ExternalURL
		updated.NamespacePath = desired.NamespacePath
		updated.Credential = desired.Credential
		updated.Status = desired.Status
		updated.LastCheckedAt = desired.LastCheckedAt
		updated.LastError = desired.LastError
		return tx.Save(&updated).Error
	})
	if err != nil {
		if isGitLabUniqueError(err) {
			return nil, store.ErrGitLabServerNameConflict
		}
		return nil, err
	}
	return &updated, nil
}

func (s *gitLabStore) DeleteGitLabServer(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.GitLabProject{}).Where("gitlab_server_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return store.ErrGitLabServerHasProjects
		}
		result := tx.Where("id = ?", id).Delete(&iapiserver.GitLabServer{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *gitLabStore) ListGitLabProjects(ctx context.Context, req *iapiserver.GitLabProjectListRequest) ([]*iapiserver.GitLabProject, int64, error) {
	var items []*iapiserver.GitLabProject
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.GitLabProject{}), func(query *gorm.DB) *gorm.DB {
		if req.GitLabServerID != "" {
			query = query.Where("gitlab_server_id = ?", req.GitLabServerID)
		}
		return query
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *gitLabStore) GetGitLabProject(ctx context.Context, id string) (*iapiserver.GitLabProject, error) {
	var item iapiserver.GitLabProject
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *gitLabStore) CreateGitLabProject(ctx context.Context, item *iapiserver.GitLabProject) (*iapiserver.GitLabProject, error) {
	if err := s.ds.db.WithContext(ctx).Create(item).Error; err != nil {
		if isGitLabUniqueError(err) {
			return nil, store.ErrGitLabProjectConflict
		}
		return nil, err
	}
	return item, nil
}

func (s *gitLabStore) DeleteGitLabProject(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).Where("id = ?", id).Delete(&iapiserver.GitLabProject{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func isGitLabUniqueError(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint")
}

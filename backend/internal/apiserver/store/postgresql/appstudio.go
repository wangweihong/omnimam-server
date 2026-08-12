package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type appStudioStore struct{ ds *datastore }

func newAppStudioStore(ds *datastore) *appStudioStore { return &appStudioStore{ds: ds} }

func (s *appStudioStore) CreateStudioApplicationInitialization(ctx context.Context, initialization *store.StudioApplicationInitialization) (bool, error) {
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "owner_user_id"}, {Name: "create_idempotency_key"}},
			DoNothing: true,
		}).Create(initialization.Application)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing iapiserver.StudioApplication
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", initialization.Application.ID).First(&existing).Error; err != nil {
				return err
			}
			if initialization.Revision == nil || existing.Status == iapiserver.AppStudioApplicationStatusReady {
				return nil
			}
			initialization.Application.ResourceVersion = existing.ResourceVersion
			if err := tx.Save(initialization.Application).Error; err != nil {
				return err
			}
			for _, value := range []any{initialization.Repository, initialization.Workspace} {
				if err := tx.Save(value).Error; err != nil {
					return err
				}
			}
		} else {
			created = true
			for _, value := range []any{initialization.Repository, initialization.Workspace} {
				if err := tx.Create(value).Error; err != nil {
					return err
				}
			}
			for _, value := range []any{
				initialization.Agent,
				initialization.Session,
				initialization.WorkspaceBinding,
				initialization.ModelBinding,
				initialization.MCPBinding,
				initialization.UserMessage,
				initialization.InitialInvocation,
			} {
				if value == nil {
					continue
				}
				if err := tx.Create(value).Error; err != nil {
					return err
				}
			}
			if initialization.MCPBinding != nil {
				if err := createMCPBindingRevision(tx, initialization.MCPBinding); err != nil {
					return err
				}
			}
			if initialization.Revision == nil {
				app := initialization.Application
				return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(app, nil))
			}
		}
		if initialization.Revision != nil {
			if err := tx.Create(initialization.Revision).Error; err != nil {
				return err
			}
		}
		if len(initialization.SourceFiles) > 0 {
			if err := tx.Create(&initialization.SourceFiles).Error; err != nil {
				return err
			}
		}
		app, revision := initialization.Application, initialization.Revision
		if err := appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(app, nil)); err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventSourceRevisionChanged, appStudioEventKey(iapiserver.AppStudioEventSourceRevisionChanged, app.ID, revision.Revision), revision.ResourceVersion, studioSourceRevisionPayload(app.ID, revision.Revision, nil))
	})
	return created, err
}

func (s *appStudioStore) GetStudioApplicationInitialization(ctx context.Context, owner, idempotencyKey string) (*store.StudioApplicationInitialization, error) {
	var app iapiserver.StudioApplication
	if err := s.ds.db.WithContext(ctx).Where("owner_user_id = ? AND create_idempotency_key = ?", owner, idempotencyKey).First(&app).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
	}
	var agent iapiserver.Agent
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", app.CodingAgentID, owner).First(&agent).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentInitializationFailed, "coding agent initialization is incomplete")
	}
	var session iapiserver.AgentSession
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND agent_id = ?", app.CodingSessionID, owner, app.CodingAgentID).First(&session).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentInitializationFailed, "coding agent session initialization is incomplete")
	}
	var invocation iapiserver.AgentInvocation
	if err := s.ds.db.WithContext(ctx).Where("agent_id = ? AND session_id = ?", app.CodingAgentID, app.CodingSessionID).Order("created_at ASC").First(&invocation).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentInitializationFailed, "initial coding invocation is incomplete")
	}
	return &store.StudioApplicationInitialization{Application: &app, Agent: &agent, Session: &session, InitialInvocation: &invocation}, nil
}

func (s *appStudioStore) GetStudioApplicationGitLabScope(ctx context.Context, gitLabProjectID string) (*store.StudioApplicationGitLabScope, error) {
	var scope store.StudioApplicationGitLabScope
	err := s.ds.db.WithContext(ctx).Table("studio_source_repositories").
		Select("studio_applications.id AS application_id, studio_applications.owner_user_id, studio_workspaces.id AS workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_source_repositories.studio_application_id").
		Joins("JOIN studio_workspaces ON studio_workspaces.repository_id = studio_source_repositories.id").
		Where("studio_source_repositories.gitlab_project_id = ?", gitLabProjectID).Take(&scope).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application is unavailable")
	}
	return &scope, nil
}

func (s *appStudioStore) GetStudioApplicationByCodingAgent(ctx context.Context, agentID, owner string) (*iapiserver.StudioApplication, error) {
	var app iapiserver.StudioApplication
	if err := s.ds.db.WithContext(ctx).Where("coding_agent_id = ? AND owner_user_id = ?", agentID, owner).First(&app).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
	}
	return &app, nil
}

func (s *appStudioStore) GetStudioApplicationWorkloadScope(ctx context.Context, applicationID, agentID string, generation int64) (*iapiserver.StudioApplication, error) {
	var app iapiserver.StudioApplication
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND coding_agent_id = ? AND coding_agent_generation = ?", applicationID, agentID, generation).First(&app).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application workload scope not visible")
	}
	return &app, nil
}

func (s *appStudioStore) ListStudioApplications(ctx context.Context, req *iapiserver.StudioApplicationListRequest) ([]*iapiserver.StudioApplication, int64, error) {
	var items []*iapiserver.StudioApplication
	params := req.BasicQueryParam
	params.Keyword = ""
	query := params.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.StudioApplication{}), func(query *gorm.DB) *gorm.DB {
		query = query.Where("owner_user_id = ?", req.OwnerUserID)
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			pattern := "%" + keyword + "%"
			query = query.Where("name ILIKE ? OR description ILIKE ?", pattern, pattern)
		}
		if req.Status != "" {
			query = query.Where("status = ?", req.Status)
		}
		return query
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *appStudioStore) GetStudioApplication(ctx context.Context, id, owner string) (*iapiserver.StudioApplication, error) {
	var item iapiserver.StudioApplication
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, owner).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
	}
	return &item, nil
}

func (s *appStudioStore) UpdateStudioApplication(ctx context.Context, app *iapiserver.StudioApplication, expectedVersion int64) (*iapiserver.StudioApplication, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.StudioApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", app.ID, app.OwnerUserID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
		}
		if previous.ResourceVersion != expectedVersion {
			return errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "studio application resource version conflicts")
		}
		app.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(app).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(app, previous.Status))
	})
	return app, err
}

// BeginStudioApplicationInitializationRetry 原子校验 ERROR 状态并切换当前 DAG 与 CREATING。
func (s *appStudioStore) BeginStudioApplicationInitializationRetry(ctx context.Context, appID, owner, dagID string) (*iapiserver.StudioApplication, error) {
	var app iapiserver.StudioApplication
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", appID, owner).First(&app).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
		}
		if app.InitializationDAGTaskGroupID == dagID {
			return nil
		}
		if app.Status != iapiserver.AppStudioApplicationStatusError {
			return errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "studio application initialization is already running or completed")
		}
		previousStatus := app.Status
		app.InitializationDAGTaskGroupID = dagID
		app.Status = iapiserver.AppStudioApplicationStatusCreating
		if err := tx.Save(&app).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(&app, previousStatus))
	})
	return &app, err
}

// RollbackStudioApplicationInitializationRetry 仅回滚仍指向候选 DAG 的 CREATING Application。
func (s *appStudioStore) RollbackStudioApplicationInitializationRetry(ctx context.Context, appID, owner, dagID string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app iapiserver.StudioApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", appID, owner).First(&app).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
		}
		if app.InitializationDAGTaskGroupID != dagID || app.Status != iapiserver.AppStudioApplicationStatusCreating {
			return nil
		}
		previousStatus := app.Status
		app.Status = iapiserver.AppStudioApplicationStatusError
		if err := tx.Save(&app).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(&app, previousStatus))
	})
}

// ReplaceStudioCodingAgent 创建新 Agent 聚合并在同一事务中切换应用当前 generation。
func (s *appStudioStore) ReplaceStudioCodingAgent(ctx context.Context, appID, owner string, replacement *store.StudioCodingAgentReplacement) (*iapiserver.StudioApplication, error) {
	var app iapiserver.StudioApplication
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", appID, owner).First(&app).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioApplicationNotVisible, "studio application not visible")
		}
		if app.CodingAgentID == replacement.Agent.ID {
			return nil
		}
		var existing int64
		if err := tx.Model(&iapiserver.Agent{}).Where("id = ?", replacement.Agent.ID).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "coding agent replacement idempotency key was already used")
		}
		if replacement.Agent.OwnerUserID != owner || replacement.Agent.WorkspaceID != app.DefaultWorkspaceID || replacement.Session.AgentID != replacement.Agent.ID || replacement.WorkspaceBinding.AgentID != replacement.Agent.ID || replacement.ModelBinding.AgentID != replacement.Agent.ID {
			return errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "coding agent replacement binding is invalid")
		}
		for _, value := range []any{replacement.Agent, replacement.Session, replacement.WorkspaceBinding, replacement.ModelBinding, replacement.MCPBinding} {
			if value == nil {
				continue
			}
			if err := tx.Create(value).Error; err != nil {
				return err
			}
		}
		if replacement.MCPBinding != nil {
			if err := createMCPBindingRevision(tx, replacement.MCPBinding); err != nil {
				return err
			}
		}
		previousStatus := app.Status
		app.CodingAgentID = replacement.Agent.ID
		app.CodingSessionID = replacement.Session.ID
		app.CodingAgentGeneration++
		if err := tx.Save(&app).Error; err != nil {
			return err
		}
		if err := appendAgentOutbox(tx, "Agent", replacement.Agent.ID, "agent_lifecycle_changed", replacement.Agent.ResourceVersion, map[string]any{"agent_id": replacement.Agent.ID, "status": replacement.Agent.Status}); err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, app.ID, iapiserver.AppStudioEventApplicationLifecycleChanged, appStudioEventKey(iapiserver.AppStudioEventApplicationLifecycleChanged, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(&app, previousStatus))
	})
	return &app, err
}

func (s *appStudioStore) GetStudioWorkspaceByApplication(ctx context.Context, appID, owner string) (*iapiserver.StudioWorkspace, error) {
	var item iapiserver.StudioWorkspace
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_workspaces.studio_application_id = ? AND studio_applications.owner_user_id = ?", appID, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio source not visible")
	}
	return &item, nil
}

func (s *appStudioStore) GetStudioWorkspace(ctx context.Context, id, owner string) (*iapiserver.StudioWorkspace, error) {
	var item iapiserver.StudioWorkspace
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_workspaces.id = ? AND studio_applications.owner_user_id = ?", id, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio source not visible")
	}
	return &item, nil
}

func (s *appStudioStore) GetStudioSourceRepository(ctx context.Context, workspaceID, owner string) (*iapiserver.StudioSourceRepository, error) {
	var item iapiserver.StudioSourceRepository
	err := s.ds.db.WithContext(ctx).
		Joins("JOIN studio_workspaces ON studio_workspaces.repository_id = studio_source_repositories.id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_workspaces.id = ? AND studio_applications.owner_user_id = ?", workspaceID, owner).First(&item).Error
	return &item, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio source repository not visible")
}

func (s *appStudioStore) ResolveStudioPreviewSource(ctx context.Context, workspaceID string, revision int64, previewRuntimeID string, resourceVersion int64) (*store.StudioSourceArchiveAccess, error) {
	var result struct {
		GitLabProjectID string
		CommitSHA       string
		ContentDigest   string
	}
	err := s.ds.db.WithContext(ctx).Table("studio_preview_runtimes").
		Select("studio_source_repositories.gitlab_project_id, studio_workspace_revisions.commit_sha, studio_workspace_revisions.content_digest").
		Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_preview_runtimes.workspace_id").
		Joins("JOIN studio_source_repositories ON studio_source_repositories.id = studio_workspaces.repository_id").
		Joins("JOIN studio_workspace_revisions ON studio_workspace_revisions.workspace_id = studio_workspaces.id AND studio_workspace_revisions.revision = studio_preview_runtimes.workspace_revision").
		Where("studio_preview_runtimes.id = ? AND studio_preview_runtimes.workspace_id = ? AND studio_preview_runtimes.workspace_revision = ? AND studio_preview_runtimes.resource_version = ?", previewRuntimeID, workspaceID, revision, resourceVersion).
		Take(&result).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceAccessInvalid, "studio preview source authorization is invalid")
	}
	if result.GitLabProjectID == "" || result.CommitSHA == "" {
		return nil, errors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "studio preview source authorization is invalid")
	}
	return &store.StudioSourceArchiveAccess{GitLabProjectID: result.GitLabProjectID, CommitSHA: result.CommitSHA, ContentDigest: result.ContentDigest}, nil
}

// ResolveStudioBuildSource resolves one current Build grant to its immutable Snapshot commit.
func (s *appStudioStore) ResolveStudioBuildSource(ctx context.Context, snapshotID, applicationID, buildID string, resourceVersion int64) (*store.StudioSourceArchiveAccess, error) {
	var result struct {
		GitLabProjectID string
		CommitSHA       string
		ContentDigest   string
	}
	err := s.ds.db.WithContext(ctx).Table("studio_builds").
		Select("studio_source_repositories.gitlab_project_id, studio_workspace_revisions.commit_sha, studio_source_snapshots.content_digest").
		Joins("JOIN studio_source_snapshots ON studio_source_snapshots.id = studio_builds.source_snapshot_id").
		Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_source_snapshots.workspace_id").
		Joins("JOIN studio_source_repositories ON studio_source_repositories.id = studio_workspaces.repository_id").
		Joins("JOIN studio_workspace_revisions ON studio_workspace_revisions.workspace_id = studio_source_snapshots.workspace_id AND studio_workspace_revisions.revision = studio_source_snapshots.workspace_revision").
		Where("studio_builds.id = ? AND studio_builds.studio_application_id = ? AND studio_builds.source_snapshot_id = ? AND studio_builds.resource_version = ?", buildID, applicationID, snapshotID, resourceVersion).
		Where("studio_source_snapshots.status = ?", iapiserver.AppStudioSnapshotStatusReady).
		Take(&result).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceAccessInvalid, "studio build source authorization is invalid")
	}
	if result.GitLabProjectID == "" || result.CommitSHA == "" || result.ContentDigest == "" {
		return nil, errors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "studio build source authorization is invalid")
	}
	return &store.StudioSourceArchiveAccess{GitLabProjectID: result.GitLabProjectID, CommitSHA: result.CommitSHA, ContentDigest: result.ContentDigest}, nil
}

func (s *appStudioStore) ListStudioSourceFiles(ctx context.Context, workspaceID string, revision int64, prefix, owner string) ([]*iapiserver.StudioSourceFile, error) {
	var items []*iapiserver.StudioSourceFile
	query := s.ds.db.WithContext(ctx).Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_source_files.workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_source_files.workspace_id = ? AND studio_source_files.revision = ? AND studio_applications.owner_user_id = ? AND studio_source_files.deleted = FALSE", workspaceID, revision, owner)
	if prefix != "" {
		query = query.Where("studio_source_files.path LIKE ?", prefix+"%")
	}
	err := query.Order("studio_source_files.path ASC").Find(&items).Error
	return items, err
}

func (s *appStudioStore) GetStudioWorkspaceRevision(ctx context.Context, workspaceID string, revision int64, owner string) (*iapiserver.StudioWorkspaceRevision, error) {
	var item iapiserver.StudioWorkspaceRevision
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_workspace_revisions.workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_workspace_revisions.workspace_id = ? AND studio_workspace_revisions.revision = ? AND studio_applications.owner_user_id = ?", workspaceID, revision, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio source revision not visible")
	}
	return &item, nil
}

// GetStudioChangeSetByIdempotencyKey 返回同一 Workspace 内已提交的 ChangeSet，供服务层在写正文前完成幂等重放。
func (s *appStudioStore) GetStudioChangeSetByIdempotencyKey(ctx context.Context, workspaceID, idempotencyKey, owner string) (*iapiserver.StudioChangeSet, error) {
	var item iapiserver.StudioChangeSet
	err := s.ds.db.WithContext(ctx).
		Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_change_sets.workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_change_sets.workspace_id = ? AND studio_change_sets.idempotency_key = ? AND studio_applications.owner_user_id = ?", workspaceID, idempotencyKey, owner).
		First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio change set not visible")
	}
	return &item, nil
}

func (s *appStudioStore) ApplyStudioChangeSet(ctx context.Context, owner string, changeSet *iapiserver.StudioChangeSet, revision *iapiserver.StudioWorkspaceRevision, files []*iapiserver.StudioSourceFile) (*iapiserver.StudioChangeSet, error) {
	var result = changeSet
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.StudioChangeSet
		err := tx.Where("workspace_id = ? AND idempotency_key = ?", changeSet.WorkspaceID, changeSet.IdempotencyKey).First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		var workspace iapiserver.StudioWorkspace
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
			Where("studio_workspaces.id = ? AND studio_applications.owner_user_id = ?", changeSet.WorkspaceID, owner).First(&workspace).Error
		if err != nil {
			return mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio source not visible")
		}
		if workspace.CurrentRevision != changeSet.BaseRevision {
			return errors.NewStatus(code.ErrAppStudioSourceRevisionConflict, "source base revision conflicts")
		}
		if workspace.Status != iapiserver.AppStudioWorkspaceStatusReady {
			return errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "studio source is not ready")
		}
		if err := tx.Create(changeSet).Error; err != nil {
			return err
		}
		if len(files) > 0 {
			if err := tx.Create(&files).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(revision).Error; err != nil {
			return err
		}
		if err := tx.Model(&workspace).Updates(map[string]any{"current_revision": revision.Revision, "current_revision_digest": revision.ContentDigest, "resource_version": gorm.Expr("resource_version + 1"), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.StudioSourceRepository{}).Where("id = ?", workspace.RepositoryID).Updates(map[string]any{"current_revision": revision.Revision, "resource_version": gorm.Expr("resource_version + 1"), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeApplication, workspace.StudioApplicationID, iapiserver.AppStudioEventSourceRevisionChanged, appStudioEventKey(iapiserver.AppStudioEventSourceRevisionChanged, workspace.StudioApplicationID, revision.Revision), revision.ResourceVersion, studioSourceRevisionPayload(workspace.StudioApplicationID, revision.Revision, changeSet))
	})
	return result, err
}

// ResolveStudioInvocationChangeSets 返回每个 Invocation 最大目标 Revision 对应的最后一个已应用 ChangeSet。
func (s *appStudioStore) ResolveStudioInvocationChangeSets(ctx context.Context, appID, owner string, invocationIDs []string) (map[string]*iapiserver.StudioChangeSet, error) {
	resolved := make(map[string]*iapiserver.StudioChangeSet, len(invocationIDs))
	if len(invocationIDs) == 0 {
		return resolved, nil
	}
	var items []*iapiserver.StudioChangeSet
	err := s.ds.db.WithContext(ctx).
		Select("DISTINCT ON (studio_change_sets.agent_invocation_id) studio_change_sets.*").
		Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_change_sets.workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_applications.id = ? AND studio_applications.owner_user_id = ?", appID, owner).
		Where("studio_change_sets.agent_invocation_id IN ? AND studio_change_sets.status = ? AND studio_change_sets.target_revision IS NOT NULL", invocationIDs, iapiserver.AppStudioChangeSetStatusApplied).
		Order("studio_change_sets.agent_invocation_id, studio_change_sets.target_revision DESC, studio_change_sets.created_at DESC, studio_change_sets.id DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		resolved[item.AgentInvocationID] = item
	}
	return resolved, nil
}

func (s *appStudioStore) CreateStudioSourceSnapshot(ctx context.Context, owner string, snapshot *iapiserver.StudioSourceSnapshot) (*iapiserver.StudioSourceSnapshot, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.StudioApplication{}).Where("id = ? AND owner_user_id = ?", snapshot.StudioApplicationID, owner).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.NewStatus(code.ErrAppStudioSnapshotNotVisible, "studio source snapshot not visible")
		}
		if snapshot.CommitSHA != "" {
			var existing iapiserver.StudioSourceSnapshot
			if err := tx.Where("studio_application_id = ? AND commit_sha = ?", snapshot.StudioApplicationID, snapshot.CommitSHA).First(&existing).Error; err == nil {
				*snapshot = existing
				return nil
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
		}
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeSourceSnapshot, snapshot.ID, iapiserver.AppStudioEventSourceSnapshotCreated, appStudioEventKey(iapiserver.AppStudioEventSourceSnapshotCreated, snapshot.ID, snapshot.ResourceVersion), snapshot.ResourceVersion, studioSourceSnapshotPayload(snapshot))
	})
	return snapshot, err
}

func (s *appStudioStore) GetStudioWorkspaceRevisionByCommit(ctx context.Context, workspaceID, commitSHA, owner string) (*iapiserver.StudioWorkspaceRevision, error) {
	var item iapiserver.StudioWorkspaceRevision
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_workspaces ON studio_workspaces.id = studio_workspace_revisions.workspace_id").
		Joins("JOIN studio_applications ON studio_applications.id = studio_workspaces.studio_application_id").
		Where("studio_workspace_revisions.workspace_id = ? AND studio_workspace_revisions.commit_sha = ? AND studio_applications.owner_user_id = ?", workspaceID, commitSHA, owner).
		First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceRevisionConflict, "canonical source revision is unavailable")
	}
	return &item, nil
}

func (s *appStudioStore) GetStudioSourceSnapshot(ctx context.Context, id, owner string) (*iapiserver.StudioSourceSnapshot, error) {
	var item iapiserver.StudioSourceSnapshot
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_source_snapshots.studio_application_id").Where("studio_source_snapshots.id = ? AND studio_applications.owner_user_id = ?", id, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSnapshotNotVisible, "studio source snapshot not visible")
	}
	return &item, nil
}

func (s *appStudioStore) CreateStudioApplicationVersion(ctx context.Context, owner string, version *iapiserver.StudioApplicationVersion) (*iapiserver.StudioApplicationVersion, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.StudioApplication{}).Where("id = ? AND owner_user_id = ?", version.StudioApplicationID, owner).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.NewStatus(code.ErrAppStudioApplicationNotVisible, "studio application not visible")
		}
		var existing iapiserver.StudioApplicationVersion
		if err := tx.Where("studio_application_id = ? AND idempotency_key = ?", version.StudioApplicationID, version.IdempotencyKey).First(&existing).Error; err == nil {
			*version = existing
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(version).Error
	})
	return version, err
}

func (s *appStudioStore) ListStudioApplicationVersions(ctx context.Context, appID, owner string, req *iapiserver.StudioApplicationVersionListRequest) ([]*iapiserver.StudioApplicationVersion, int64, error) {
	var items []*iapiserver.StudioApplicationVersion
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.StudioApplicationVersion{}), func(query *gorm.DB) *gorm.DB {
		return query.Joins("JOIN studio_applications ON studio_applications.id = studio_application_versions.studio_application_id").Where("studio_application_versions.studio_application_id = ? AND studio_applications.owner_user_id = ?", appID, owner)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *appStudioStore) GetStudioApplicationVersion(ctx context.Context, id, owner string) (*iapiserver.StudioApplicationVersion, error) {
	var item iapiserver.StudioApplicationVersion
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_application_versions.studio_application_id").Where("studio_application_versions.id = ? AND studio_applications.owner_user_id = ?", id, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSnapshotNotVisible, "studio application version not visible")
	}
	return &item, nil
}

func (s *appStudioStore) CreateStudioBuild(ctx context.Context, owner string, build *iapiserver.StudioBuild) (*iapiserver.StudioBuild, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.StudioBuild
		if err := tx.Where("studio_application_id = ? AND idempotency_key = ?", build.StudioApplicationID, build.IdempotencyKey).First(&existing).Error; err == nil {
			*build = existing
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if build.OwnerUserID != owner {
			return errors.NewStatus(code.ErrAppStudioAccessDenied, "appstudio owner mismatch")
		}
		if err := tx.Create(build).Error; err != nil {
			return err
		}
		return appendStudioBuildOutbox(tx, nil, build)
	})
	return build, err
}

func (s *appStudioStore) ListStudioBuilds(ctx context.Context, appID, owner string, req *iapiserver.StudioBuildListRequest) ([]*iapiserver.StudioBuild, int64, error) {
	var items []*iapiserver.StudioBuild
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.StudioBuild{}), func(query *gorm.DB) *gorm.DB {
		return query.Where("studio_application_id = ? AND owner_user_id = ?", appID, owner)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *appStudioStore) GetStudioBuild(ctx context.Context, id, owner string) (*iapiserver.StudioBuild, error) {
	var item iapiserver.StudioBuild
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, owner).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioBuildNotVisible, "studio build not visible")
	}
	return &item, nil
}
func (s *appStudioStore) ResolveStudioBuildSummaries(ctx context.Context, owner string, ids []string) (map[string]*iapiserver.StudioBuildProducerProjection, error) {
	items := make([]*iapiserver.StudioBuildProducerProjection, 0, len(ids))
	if len(ids) > 0 {
		if err := s.ds.db.WithContext(ctx).Model(&iapiserver.StudioBuild{}).
			Select("id", "owner_user_id", "name", "status").
			Where("id IN ? AND owner_user_id = ?", ids, owner).
			Scan(&items).Error; err != nil {
			return nil, err
		}
	}
	summaries := make(map[string]*iapiserver.StudioBuildProducerProjection, len(items))
	for _, item := range items {
		summaries[item.ID] = item
	}
	return summaries, nil
}
func (s *appStudioStore) UpdateStudioBuild(ctx context.Context, build *iapiserver.StudioBuild) (*iapiserver.StudioBuild, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.StudioBuild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", build.ID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioBuildNotVisible, "studio build not visible")
		}
		build.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(build).Error; err != nil {
			return err
		}
		return appendStudioBuildOutbox(tx, &previous, build)
	})
	return build, err
}

func (s *appStudioStore) ProjectStudioBuildPipeline(ctx context.Context, appID, commitSHA string, pipelineID int64, pipelineURL, pipelineStatus string) (*iapiserver.StudioBuild, error) {
	var build iapiserver.StudioBuild
	found := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("studio_application_id = ? AND commit_sha = ?", appID, commitSHA).First(&build).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		found = true
		if build.PipelineID != 0 && build.PipelineID != pipelineID {
			return errors.NewStatus(code.ErrAppStudioBuildFailed, "pipeline projection conflicts with existing build")
		}
		previous := build
		build.PipelineID, build.PipelineURL = pipelineID, pipelineURL
		switch strings.ToLower(pipelineStatus) {
		case "failed", "canceled", "skipped":
			if build.Status != iapiserver.AppStudioBuildStatusSucceeded {
				build.Status = iapiserver.AppStudioBuildStatusFailed
			}
		case "created", "pending", "running":
			if build.Status == iapiserver.AppStudioBuildStatusPending {
				build.Status = iapiserver.AppStudioBuildStatusRunning
			}
		}
		if err := tx.Save(&build).Error; err != nil {
			return err
		}
		return appendStudioBuildOutbox(tx, &previous, &build)
	})
	if !found {
		return nil, err
	}
	return &build, err
}

func (s *appStudioStore) GetStudioPreviewRuntime(ctx context.Context, workspaceID, owner string) (*iapiserver.StudioPreviewRuntime, error) {
	var item iapiserver.StudioPreviewRuntime
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_preview_runtimes.studio_application_id").Where("studio_preview_runtimes.workspace_id = ? AND studio_applications.owner_user_id = ?", workspaceID, owner).Order("studio_preview_runtimes.created_at DESC").First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio preview runtime not visible")
	}
	if item.Status == iapiserver.AppStudioPreviewStatusRunning && item.EndpointRef != "" {
		item.EndpointSummary = &iapiserver.StudioEndpointSummary{DisplayRef: item.EndpointRef, Visibility: iapiserver.AppStudioEndpointVisibilityUser, Status: iapiserver.AppStudioEndpointStatusReady, ExpiresAt: item.ExpiresAt}
	}
	return &item, nil
}
func (s *appStudioStore) CreateStudioPreviewRuntime(ctx context.Context, owner string, runtime *iapiserver.StudioPreviewRuntime) (*iapiserver.StudioPreviewRuntime, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.StudioApplication{}).Where("id = ? AND owner_user_id = ?", runtime.StudioApplicationID, owner).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.NewStatus(code.ErrAppStudioSourceNotVisible, "studio preview runtime not visible")
		}
		if runtime.ID != "" {
			var existing iapiserver.StudioPreviewRuntime
			if err := tx.Where("id = ?", runtime.ID).First(&existing).Error; err == nil {
				if existing.StudioApplicationID != runtime.StudioApplicationID || existing.WorkspaceID != runtime.WorkspaceID || existing.WorkspaceRevision != runtime.WorkspaceRevision {
					return errors.NewStatus(code.ErrAppStudioRuntimeDeployFailed, "preview idempotency identity conflicts")
				}
				*runtime = existing
				return nil
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
		}
		if err := tx.Create(runtime).Error; err != nil {
			return err
		}
		return appendStudioPreviewOutbox(tx, nil, runtime)
	})
	return runtime, err
}
func (s *appStudioStore) UpdateStudioPreviewRuntime(ctx context.Context, runtime *iapiserver.StudioPreviewRuntime) (*iapiserver.StudioPreviewRuntime, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.StudioPreviewRuntime
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.ID).First(&previous).Error; err != nil {
			return err
		}
		runtime.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(runtime).Error; err != nil {
			return err
		}
		return appendStudioPreviewOutbox(tx, &previous, runtime)
	})
	return runtime, err
}

func (s *appStudioStore) GetStudioRuntimeConfig(ctx context.Context, versionID, environment, owner string) (*iapiserver.StudioRuntimeConfig, error) {
	var item iapiserver.StudioRuntimeConfig
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_application_versions ON studio_application_versions.id = studio_runtime_configs.studio_application_version_id").Joins("JOIN studio_applications ON studio_applications.id = studio_application_versions.studio_application_id").Where("studio_runtime_configs.studio_application_version_id = ? AND studio_runtime_configs.environment = ? AND studio_applications.owner_user_id = ?", versionID, environment, owner).Order("studio_runtime_configs.resource_version DESC").First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
func (s *appStudioStore) ReplaceStudioRuntimeConfig(ctx context.Context, owner string, config *iapiserver.StudioRuntimeConfig, expected int64) (*iapiserver.StudioRuntimeConfig, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var version iapiserver.StudioApplicationVersion
		if err := tx.Joins("JOIN studio_applications ON studio_applications.id = studio_application_versions.studio_application_id").Where("studio_application_versions.id = ? AND studio_applications.owner_user_id = ?", config.StudioApplicationVersionID, owner).First(&version).Error; err != nil {
			return mapNotFound(err, code.ErrAppStudioSnapshotNotVisible, "studio application version not visible")
		}
		var current iapiserver.StudioRuntimeConfig
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("studio_application_version_id = ? AND environment = ?", config.StudioApplicationVersionID, config.Environment).Order("resource_version DESC").First(&current).Error
		if err == nil {
			if current.ResourceVersion != expected {
				return errors.NewStatus(code.ErrAppStudioReleaseInvalid, "runtime config resource version conflicts")
			}
			config.ResourceVersion = current.ResourceVersion + 1
		} else if err != gorm.ErrRecordNotFound {
			return err
		} else if expected != 0 {
			return errors.NewStatus(code.ErrAppStudioReleaseInvalid, "runtime config does not exist")
		}
		return tx.Create(config).Error
	})
	return config, err
}

func (s *appStudioStore) CreateStudioReleaseAggregate(ctx context.Context, owner string, release *iapiserver.StudioRelease, runtime *iapiserver.StudioRuntimeInstance) (*iapiserver.StudioRelease, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.StudioRelease
		if err := tx.Where("studio_application_id = ? AND idempotency_key = ?", release.StudioApplicationID, release.IdempotencyKey).First(&existing).Error; err == nil {
			*release = existing
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if release.OwnerUserID != owner {
			return errors.NewStatus(code.ErrAppStudioAccessDenied, "appstudio owner mismatch")
		}
		if err := tx.Create(release).Error; err != nil {
			return err
		}
		if err := tx.Create(runtime).Error; err != nil {
			return err
		}
		release.RuntimeInstanceID = runtime.ID
		if err := appendStudioReleaseOutbox(tx, nil, release); err != nil {
			return err
		}
		return appendStudioRuntimeOutbox(tx, nil, runtime)
	})
	return release, err
}
func (s *appStudioStore) ListStudioReleases(ctx context.Context, appID, owner string, req *iapiserver.StudioReleaseListRequest) ([]*iapiserver.StudioRelease, int64, error) {
	var items []*iapiserver.StudioRelease
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.StudioRelease{}), func(q *gorm.DB) *gorm.DB {
		return q.Where("studio_application_id = ? AND owner_user_id = ?", appID, owner)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *appStudioStore) GetStudioRelease(ctx context.Context, id, owner string) (*iapiserver.StudioRelease, error) {
	var item iapiserver.StudioRelease
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, owner).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioReleaseInvalid, "studio release not visible")
	}
	var runtime iapiserver.StudioRuntimeInstance
	if err := s.ds.db.WithContext(ctx).Where("studio_release_id = ?", id).Order("created_at DESC").First(&runtime).Error; err == nil {
		item.RuntimeInstanceID = runtime.ID
	}
	return &item, nil
}
func (s *appStudioStore) UpdateStudioRelease(ctx context.Context, release *iapiserver.StudioRelease) (*iapiserver.StudioRelease, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.StudioRelease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", release.ID).First(&previous).Error; err != nil {
			return err
		}
		release.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(release).Error; err != nil {
			return err
		}
		return appendStudioReleaseOutbox(tx, &previous, release)
	})
	return release, err
}
func (s *appStudioStore) ListStudioRuntimeInstances(ctx context.Context, appID, owner string, req *iapiserver.StudioRuntimeInstanceListRequest) ([]*iapiserver.StudioRuntimeInstance, int64, error) {
	var items []*iapiserver.StudioRuntimeInstance
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.StudioRuntimeInstance{}), func(q *gorm.DB) *gorm.DB {
		q = q.Joins("JOIN studio_applications ON studio_applications.id = studio_runtime_instances.studio_application_id").Where("studio_runtime_instances.studio_application_id = ? AND studio_applications.owner_user_id = ?", appID, owner)
		if req.Environment != "" {
			q = q.Where("studio_runtime_instances.environment = ?", req.Environment)
		}
		if req.Status != "" {
			q = q.Where("studio_runtime_instances.status = ?", req.Status)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *appStudioStore) GetStudioRuntimeInstance(ctx context.Context, id, owner string) (*iapiserver.StudioRuntimeInstance, error) {
	var item iapiserver.StudioRuntimeInstance
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_runtime_instances.studio_application_id").Where("studio_runtime_instances.id = ? AND studio_applications.owner_user_id = ?", id, owner).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioRuntimeDeployFailed, "studio runtime instance not visible")
	}
	return &item, nil
}
func (s *appStudioStore) UpdateStudioRuntimeInstance(ctx context.Context, runtime *iapiserver.StudioRuntimeInstance) (*iapiserver.StudioRuntimeInstance, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.StudioRuntimeInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.ID).First(&previous).Error; err != nil {
			return err
		}
		runtime.ResourceVersion = previous.ResourceVersion
		if runtime.IsCurrent {
			if err := tx.Model(&iapiserver.StudioRuntimeInstance{}).Where("studio_application_id = ? AND environment = ? AND id <> ? AND is_current = TRUE", runtime.StudioApplicationID, runtime.Environment, runtime.ID).Update("is_current", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(runtime).Error; err != nil {
			return err
		}
		return appendStudioRuntimeOutbox(tx, &previous, runtime)
	})
	return runtime, err
}

// ListPendingStudioTerminalTaskIDs returns terminal Tasks still needed by durable Build or Production projections.
// Preview and stop operations cannot participate because their Task IDs are not persisted by the released model.
func (s *appStudioStore) ListPendingStudioTerminalTaskIDs(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	terminalStatuses := []string{
		iapiserver.AtomicTaskStatusSuccess,
		iapiserver.AtomicTaskStatusFailed,
		iapiserver.AtomicTaskStatusCanceled,
		iapiserver.AtomicTaskStatusTimeout,
		iapiserver.AtomicTaskStatusSkipped,
	}
	ids := make([]string, 0, limit)
	if err := s.ds.db.WithContext(ctx).Table("studio_builds").
		Select("studio_builds.atomic_task_id").
		Joins("JOIN atomic_tasks ON atomic_tasks.id = studio_builds.atomic_task_id").
		Where("studio_builds.atomic_task_id <> '' AND studio_builds.status = ? AND atomic_tasks.status IN ?",
			iapiserver.AppStudioBuildStatusRunning, terminalStatuses).
		Order("studio_builds.updated_at ASC").Limit(limit).
		Pluck("studio_builds.atomic_task_id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) >= limit {
		return ids, nil
	}
	runtimeIDs := make([]string, 0, limit-len(ids))
	if err := s.ds.db.WithContext(ctx).Table("studio_runtime_instances").
		Select("studio_runtime_instances.atomic_task_id").
		Joins("JOIN studio_releases ON studio_releases.id = studio_runtime_instances.studio_release_id").
		Joins("JOIN atomic_tasks ON atomic_tasks.id = studio_runtime_instances.atomic_task_id").
		Where("studio_runtime_instances.atomic_task_id <> '' AND studio_releases.status IN ? AND atomic_tasks.status IN ?",
			[]string{iapiserver.AppStudioReleaseStatusPending, iapiserver.AppStudioReleaseStatusDeploying}, terminalStatuses).
		Order("studio_runtime_instances.updated_at ASC").Limit(limit-len(ids)).
		Pluck("studio_runtime_instances.atomic_task_id", &runtimeIDs).Error; err != nil {
		return nil, err
	}
	return append(ids, runtimeIDs...), nil
}

func (s *appStudioStore) ProjectStudioTaskTerminal(ctx context.Context, task *iapiserver.AtomicTask) error {
	if task == nil || !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	switch task.FunctionRef {
	case iapiserver.AppStudioFunctionInitializationProjectEnsure,
		iapiserver.AppStudioFunctionInitializationWebhookEnsure,
		iapiserver.AppStudioFunctionInitializationFinalize,
		iapiserver.AppStudioFunctionInitializationInvocationStart:
		arguments, err := decodeAppStudioTaskValue[iapiserver.AppStudioInitializationTaskArguments](task.Arguments)
		if err != nil {
			return err
		}
		var app iapiserver.StudioApplication
		if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", arguments.StudioApplicationID, arguments.OwnerUserID).First(&app).Error; err != nil {
			return err
		}
		// 旧初始化 DAG 的迟到终态只保留 Task Center 历史，不得覆盖当前轮次。
		if task.OwnerID != app.InitializationDAGTaskGroupID {
			return nil
		}
		status := ""
		if task.Status == iapiserver.AtomicTaskStatusSuccess && task.FunctionRef == iapiserver.AppStudioFunctionInitializationInvocationStart {
			status = iapiserver.AppStudioApplicationStatusReady
		} else if task.Status != iapiserver.AtomicTaskStatusSuccess {
			status = iapiserver.AppStudioApplicationStatusError
		}
		if status == "" || app.Status == status || app.Status == iapiserver.AppStudioApplicationStatusReady {
			return nil
		}
		app.Status = status
		_, err = s.UpdateStudioApplication(ctx, &app, app.ResourceVersion)
		return err
	case iapiserver.AppStudioFunctionPreviewEnsure, iapiserver.AppStudioFunctionPreviewStop:
		arguments, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskProjectionArguments](task.Arguments)
		if err != nil {
			return err
		}
		id := arguments.PreviewRuntimeID
		var runtime iapiserver.StudioPreviewRuntime
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&runtime).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			output, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskOutput](task.Output)
			if err != nil {
				return err
			}
			runtime.InfraRuntimeID = output.InfraRuntimeID
			runtime.EndpointRef = output.EndpointRef
			if task.FunctionRef == iapiserver.AppStudioFunctionPreviewEnsure {
				healthStatus := output.HealthStatus
				if healthStatus != "" {
					diagnostics := make(map[string]any)
					if len(runtime.DiagnosticsSummary) > 0 {
						if err := json.Unmarshal(runtime.DiagnosticsSummary, &diagnostics); err != nil {
							return err
						}
					}
					diagnostics["health_status"] = healthStatus
					encoded, err := json.Marshal(diagnostics)
					if err != nil {
						return err
					}
					runtime.DiagnosticsSummary = encoded
				}
				runtime.Status = iapiserver.AppStudioPreviewStatusRunning
			} else {
				runtime.Status = iapiserver.AppStudioPreviewStatusStopped
			}
		} else {
			runtime.Status = iapiserver.AppStudioPreviewStatusFailed
		}
		_, err = s.UpdateStudioPreviewRuntime(ctx, &runtime)
		return err
	case iapiserver.AppStudioFunctionBuildExecute:
		arguments, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskProjectionArguments](task.Arguments)
		if err != nil {
			return err
		}
		id := arguments.StudioBuildID
		var build iapiserver.StudioBuild
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&build).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			output, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskOutput](task.Output)
			if err != nil {
				return err
			}
			build.ArtifactID = output.ArtifactID
			build.ArtifactDigest = output.ArtifactDigest
			if build.ArtifactID != "" && build.ArtifactDigest != "" {
				build.Status = iapiserver.AppStudioBuildStatusSucceeded
			} else {
				build.Status = iapiserver.AppStudioBuildStatusFailed
			}
		} else if task.Status == iapiserver.AtomicTaskStatusCanceled {
			build.Status = iapiserver.AppStudioBuildStatusCanceled
		} else {
			build.Status = iapiserver.AppStudioBuildStatusFailed
		}
		_, err = s.UpdateStudioBuild(ctx, &build)
		return err
	case iapiserver.AppStudioFunctionProductionEnsure, iapiserver.AppStudioFunctionProductionStop:
		arguments, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskProjectionArguments](task.Arguments)
		if err != nil {
			return err
		}
		id := arguments.StudioRuntimeInstanceID
		var runtime iapiserver.StudioRuntimeInstance
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&runtime).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			output, err := decodeAppStudioTaskValue[iapiserver.AppStudioTaskOutput](task.Output)
			if err != nil {
				return err
			}
			runtime.InfraRuntimeID = output.InfraRuntimeID
			runtime.EndpointRef = output.EndpointRef
			if task.FunctionRef == iapiserver.AppStudioFunctionProductionEnsure {
				runtime.Status, runtime.HealthStatus, runtime.IsCurrent = iapiserver.AppStudioRuntimeStatusReady, iapiserver.AppStudioRuntimeHealthHealthy, true
			} else {
				runtime.Status, runtime.HealthStatus, runtime.IsCurrent = iapiserver.AppStudioRuntimeStatusStopped, iapiserver.AppStudioRuntimeHealthUnknown, false
			}
		} else {
			runtime.Status, runtime.HealthStatus = iapiserver.AppStudioRuntimeStatusFailed, iapiserver.AppStudioRuntimeHealthUnhealthy
		}
		_, err = s.UpdateStudioRuntimeInstance(ctx, &runtime)
		if err != nil {
			return err
		}
		var release iapiserver.StudioRelease
		if err := s.ds.db.WithContext(ctx).Where("id = ?", runtime.StudioReleaseID).First(&release).Error; err != nil {
			return err
		}
		if runtime.Status == iapiserver.AppStudioRuntimeStatusReady {
			release.Status = iapiserver.AppStudioReleaseStatusReady
		} else if runtime.Status == iapiserver.AppStudioRuntimeStatusFailed {
			release.Status = iapiserver.AppStudioReleaseStatusFailed
		}
		release.RuntimeInstanceID = runtime.ID
		_, err = s.UpdateStudioRelease(ctx, &release)
		return err
	default:
		return nil
	}
}

func decodeAppStudioTaskValue[T any](value map[string]any) (T, error) {
	var result T
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &result, TagName: "json"})
	if err != nil {
		return result, err
	}
	return result, decoder.Decode(value)
}

func appStudioEventKey(eventType string, components ...any) string {
	key := eventType
	for _, component := range components {
		key += ":" + fmt.Sprint(component)
	}
	return key
}

func marshalAppStudioEventPayload(payload any, version int64, occurredAt time.Time) ([]byte, error) {
	metadata := iapiserver.AppStudioEventMetadata{ResourceVersion: version, OccurredAt: occurredAt.UTC().Format(time.RFC3339Nano)}
	switch value := payload.(type) {
	case *iapiserver.AppStudioApplicationLifecycleEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioSourceRevisionEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioSourceSnapshotEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioBuildEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioPreviewEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioReleaseEventPayload:
		value.AppStudioEventMetadata = metadata
	case *iapiserver.AppStudioRuntimeEventPayload:
		value.AppStudioEventMetadata = metadata
	default:
		return nil, fmt.Errorf("unsupported appstudio event payload %T", payload)
	}
	return json.Marshal(payload)
}

func appendAppStudioOutbox(tx *gorm.DB, aggregateType, aggregateID, eventType, idempotencyKey string, version int64, payload any) error {
	raw, err := marshalAppStudioEventPayload(payload, version, time.Now())
	if err != nil {
		return err
	}
	event := &iapiserver.AppStudioOutbox{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AggregateType: aggregateType, AggregateID: aggregateID, EventType: eventType, Payload: raw, IdempotencyKey: idempotencyKey, DeliveryStatus: iapiserver.AppStudioOutboxDeliveryPending, NextAttemptAt: imachinery.Now()}
	return tx.Create(event).Error
}

func studioApplicationLifecyclePayload(app *iapiserver.StudioApplication, fromStatus any) *iapiserver.AppStudioApplicationLifecycleEventPayload {
	return &iapiserver.AppStudioApplicationLifecycleEventPayload{StudioApplicationID: app.ID, OwnerUserID: app.OwnerUserID, FromStatus: appStudioNullableStatus(fromStatus), ToStatus: app.Status}
}

func studioSourceRevisionPayload(appID string, currentRevision int64, changeSet *iapiserver.StudioChangeSet) *iapiserver.AppStudioSourceRevisionEventPayload {
	payload := &iapiserver.AppStudioSourceRevisionEventPayload{StudioApplicationID: appID, CurrentRevision: currentRevision}
	if changeSet != nil {
		payload.PreviousRevision = &changeSet.BaseRevision
		payload.ChangeSetID = appStudioNullableString(changeSet.ID)
		payload.AgentID = appStudioNullableString(changeSet.AgentID)
		payload.AgentInvocationID = appStudioNullableString(changeSet.AgentInvocationID)
	}
	return payload
}

func studioSourceSnapshotPayload(snapshot *iapiserver.StudioSourceSnapshot) *iapiserver.AppStudioSourceSnapshotEventPayload {
	return &iapiserver.AppStudioSourceSnapshotEventPayload{SourceSnapshotID: snapshot.ID, StudioApplicationID: snapshot.StudioApplicationID, SourceRevision: snapshot.WorkspaceRevision, ContentDigest: snapshot.ContentDigest, ManifestDigest: snapshot.ManifestDigest, CreatedBy: snapshot.CreatedBy}
}

func appendStudioBuildOutbox(tx *gorm.DB, previous, build *iapiserver.StudioBuild) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeBuild, build.ID, iapiserver.AppStudioEventBuildProjectionChanged, appStudioEventKey(iapiserver.AppStudioEventBuildProjectionChanged, build.ID, build.ResourceVersion), build.ResourceVersion, studioBuildPayload(build, from))
}
func studioBuildPayload(build *iapiserver.StudioBuild, fromStatus any) *iapiserver.AppStudioBuildEventPayload {
	return &iapiserver.AppStudioBuildEventPayload{StudioBuildID: build.ID, StudioApplicationID: build.StudioApplicationID, SourceSnapshotID: build.SourceSnapshotID, AtomicTaskID: appStudioNullableString(build.AtomicTaskID), ArtifactID: appStudioNullableString(build.ArtifactID), ArtifactDigest: appStudioNullableString(build.ArtifactDigest), FromStatus: appStudioNullableStatus(fromStatus), ToStatus: build.Status}
}
func appendStudioPreviewOutbox(tx *gorm.DB, previous, runtime *iapiserver.StudioPreviewRuntime) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypePreviewRuntime, runtime.ID, iapiserver.AppStudioEventPreviewRuntimeChanged, appStudioEventKey(iapiserver.AppStudioEventPreviewRuntimeChanged, runtime.ID, runtime.ResourceVersion), runtime.ResourceVersion, studioPreviewPayload(runtime, from))
}
func studioPreviewPayload(runtime *iapiserver.StudioPreviewRuntime, fromStatus any) *iapiserver.AppStudioPreviewEventPayload {
	diagnosticsSummary := runtime.DiagnosticsSummary
	if len(diagnosticsSummary) == 0 {
		diagnosticsSummary = json.RawMessage("{}")
	}
	return &iapiserver.AppStudioPreviewEventPayload{PreviewRuntimeID: runtime.ID, StudioApplicationID: runtime.StudioApplicationID, SourceRevision: runtime.WorkspaceRevision, FromStatus: appStudioNullableStatus(fromStatus), ToStatus: runtime.Status, DiagnosticsSummary: diagnosticsSummary}
}
func appendStudioReleaseOutbox(tx *gorm.DB, previous, release *iapiserver.StudioRelease) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeRelease, release.ID, iapiserver.AppStudioEventReleaseStatusChanged, appStudioEventKey(iapiserver.AppStudioEventReleaseStatusChanged, release.ID, release.ResourceVersion), release.ResourceVersion, studioReleasePayload(release, from))
}
func studioReleasePayload(release *iapiserver.StudioRelease, fromStatus any) *iapiserver.AppStudioReleaseEventPayload {
	return &iapiserver.AppStudioReleaseEventPayload{StudioReleaseID: release.ID, StudioApplicationID: release.StudioApplicationID, StudioApplicationVersionID: release.StudioApplicationVersionID, StudioBuildID: release.StudioBuildID, RuntimeConfigID: release.RuntimeConfigID, ArtifactID: release.ArtifactID, ArtifactDigest: release.ArtifactDigest, Environment: release.Environment, RuntimeInstanceID: appStudioNullableString(release.RuntimeInstanceID), RollbackOfReleaseID: appStudioNullableString(release.RollbackOfReleaseID), FromStatus: appStudioNullableStatus(fromStatus), ToStatus: release.Status}
}
func appendStudioRuntimeOutbox(tx *gorm.DB, previous, runtime *iapiserver.StudioRuntimeInstance) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, iapiserver.AppStudioAggregateTypeRuntime, runtime.ID, iapiserver.AppStudioEventRuntimeInstanceChanged, appStudioEventKey(iapiserver.AppStudioEventRuntimeInstanceChanged, runtime.ID, runtime.ResourceVersion), runtime.ResourceVersion, studioRuntimePayload(runtime, from))
}
func studioRuntimePayload(runtime *iapiserver.StudioRuntimeInstance, fromStatus any) *iapiserver.AppStudioRuntimeEventPayload {
	return &iapiserver.AppStudioRuntimeEventPayload{RuntimeInstanceID: runtime.ID, StudioReleaseID: runtime.StudioReleaseID, StudioApplicationID: runtime.StudioApplicationID, Environment: runtime.Environment, AtomicTaskID: appStudioNullableString(runtime.AtomicTaskID), InfraRuntimeID: appStudioNullableString(runtime.InfraRuntimeID), FromStatus: appStudioNullableStatus(fromStatus), ToStatus: runtime.Status, HealthStatus: runtime.HealthStatus, IsCurrent: runtime.IsCurrent, ErrorCode: appStudioNullableString(runtime.ErrorCode)}
}

func appStudioNullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func appStudioNullableStatus(value any) *string {
	status, ok := value.(string)
	if !ok || status == "" {
		return nil
	}
	return &status
}

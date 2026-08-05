package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type appStudioStore struct{ ds *datastore }

const (
	studioApplicationLifecycleChangedEvent = "studio_application_lifecycle_changed"
	studioSourceRevisionChangedEvent       = "studio_source_revision_changed"
	studioSourceSnapshotCreatedEvent       = "studio_source_snapshot_created"
	studioBuildProjectionChangedEvent      = "studio_build_projection_changed"
	studioPreviewRuntimeChangedEvent       = "studio_preview_runtime_status_changed"
	studioReleaseStatusChangedEvent        = "studio_release_status_changed"
	studioRuntimeInstanceChangedEvent      = "studio_runtime_instance_status_changed"
)

func newAppStudioStore(ds *datastore) *appStudioStore { return &appStudioStore{ds: ds} }

func (s *appStudioStore) CreateStudioApplicationAggregate(ctx context.Context, app *iapiserver.StudioApplication, repository *iapiserver.StudioSourceRepository, workspace *iapiserver.StudioWorkspace, revision *iapiserver.StudioWorkspaceRevision) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, value := range []any{app, repository, workspace, revision} {
			if err := tx.Create(value).Error; err != nil {
				return err
			}
		}
		if err := appendAppStudioOutbox(tx, "StudioApplication", app.ID, studioApplicationLifecycleChangedEvent, appStudioEventKey(studioApplicationLifecycleChangedEvent, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(app, nil)); err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, "StudioApplication", app.ID, studioSourceRevisionChangedEvent, appStudioEventKey(studioSourceRevisionChangedEvent, app.ID, revision.Revision), revision.ResourceVersion, studioSourceRevisionPayload(app.ID, revision.Revision, nil))
	})
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
		return appendAppStudioOutbox(tx, "StudioApplication", app.ID, studioApplicationLifecycleChangedEvent, appStudioEventKey(studioApplicationLifecycleChangedEvent, app.ID, app.ResourceVersion), app.ResourceVersion, studioApplicationLifecyclePayload(app, previous.Status))
	})
	return app, err
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
		return appendAppStudioOutbox(tx, "StudioApplication", workspace.StudioApplicationID, studioSourceRevisionChangedEvent, appStudioEventKey(studioSourceRevisionChangedEvent, workspace.StudioApplicationID, revision.Revision), revision.ResourceVersion, studioSourceRevisionPayload(workspace.StudioApplicationID, revision.Revision, changeSet))
	})
	return result, err
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
		if snapshot.ContentDigest != "" {
			var existing iapiserver.StudioSourceSnapshot
			if err := tx.Where("studio_application_id = ? AND content_digest = ?", snapshot.StudioApplicationID, snapshot.ContentDigest).First(&existing).Error; err == nil {
				*snapshot = existing
				return nil
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
		}
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}
		return appendAppStudioOutbox(tx, "StudioSourceSnapshot", snapshot.ID, studioSourceSnapshotCreatedEvent, appStudioEventKey(studioSourceSnapshotCreatedEvent, snapshot.ID, snapshot.ResourceVersion), snapshot.ResourceVersion, studioSourceSnapshotPayload(snapshot))
	})
	return snapshot, err
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

func (s *appStudioStore) GetStudioPreviewRuntime(ctx context.Context, workspaceID, owner string) (*iapiserver.StudioPreviewRuntime, error) {
	var item iapiserver.StudioPreviewRuntime
	err := s.ds.db.WithContext(ctx).Joins("JOIN studio_applications ON studio_applications.id = studio_preview_runtimes.studio_application_id").Where("studio_preview_runtimes.workspace_id = ? AND studio_applications.owner_user_id = ?", workspaceID, owner).Order("studio_preview_runtimes.created_at DESC").First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, mapNotFound(err, code.ErrAppStudioSourceNotVisible, "studio preview runtime not visible")
	}
	if item.Status == "RUNNING" && item.EndpointRef != "" {
		item.EndpointSummary = &iapiserver.StudioEndpointSummary{DisplayRef: item.EndpointRef, Visibility: "USER_ACCESSIBLE", Status: "READY", ExpiresAt: item.ExpiresAt}
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

func (s *appStudioStore) ProjectStudioTaskTerminal(ctx context.Context, task *iapiserver.AtomicTask) error {
	if task == nil || !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	switch task.FunctionRef {
	case "appstudio.preview.ensure", "appstudio.preview.stop":
		id, _ := task.Arguments["preview_runtime_id"].(string)
		var runtime iapiserver.StudioPreviewRuntime
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&runtime).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			runtime.InfraRuntimeID, _ = task.Output["infra_runtime_id"].(string)
			runtime.EndpointRef, _ = task.Output["endpoint_ref"].(string)
			if task.FunctionRef == "appstudio.preview.ensure" {
				healthStatus, _ := task.Output["health_status"].(string)
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
				runtime.Status = "RUNNING"
			} else {
				runtime.Status = "STOPPED"
			}
		} else {
			runtime.Status = "FAILED"
		}
		_, err := s.UpdateStudioPreviewRuntime(ctx, &runtime)
		return err
	case "appstudio.build.execute":
		id, _ := task.Arguments["studio_build_id"].(string)
		var build iapiserver.StudioBuild
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&build).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			build.ArtifactID, _ = task.Output["artifact_id"].(string)
			build.ArtifactDigest, _ = task.Output["artifact_digest"].(string)
			if build.ArtifactID != "" && build.ArtifactDigest != "" {
				build.Status = "SUCCEEDED"
			} else {
				build.Status = "FAILED"
			}
		} else if task.Status == iapiserver.AtomicTaskStatusCanceled {
			build.Status = "CANCELED"
		} else {
			build.Status = "FAILED"
		}
		_, err := s.UpdateStudioBuild(ctx, &build)
		return err
	case "appstudio.production.reconcile", "appstudio.production.stop":
		id, _ := task.Arguments["studio_runtime_instance_id"].(string)
		var runtime iapiserver.StudioRuntimeInstance
		if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&runtime).Error; err != nil {
			return err
		}
		if task.Status == iapiserver.AtomicTaskStatusSuccess {
			runtime.InfraRuntimeID, _ = task.Output["infra_runtime_id"].(string)
			runtime.EndpointRef, _ = task.Output["endpoint_ref"].(string)
			if task.FunctionRef == "appstudio.production.reconcile" {
				runtime.Status, runtime.HealthStatus, runtime.IsCurrent = "READY", "HEALTHY", true
			} else {
				runtime.Status, runtime.HealthStatus, runtime.IsCurrent = "STOPPED", "UNKNOWN", false
			}
		} else {
			runtime.Status, runtime.HealthStatus = "FAILED", "UNHEALTHY"
		}
		_, err := s.UpdateStudioRuntimeInstance(ctx, &runtime)
		if err != nil {
			return err
		}
		var release iapiserver.StudioRelease
		if err := s.ds.db.WithContext(ctx).Where("id = ?", runtime.StudioReleaseID).First(&release).Error; err != nil {
			return err
		}
		if runtime.Status == "READY" {
			release.Status = "READY"
		} else if runtime.Status == "FAILED" {
			release.Status = "FAILED"
		}
		release.RuntimeInstanceID = runtime.ID
		_, err = s.UpdateStudioRelease(ctx, &release)
		return err
	default:
		return nil
	}
}

func appStudioEventKey(eventType string, components ...any) string {
	key := eventType
	for _, component := range components {
		key += ":" + fmt.Sprint(component)
	}
	return key
}

func marshalAppStudioEventPayload(payload map[string]any, version int64, occurredAt time.Time) ([]byte, error) {
	payload["resource_version"] = version
	payload["occurred_at"] = occurredAt.UTC().Format(time.RFC3339Nano)
	return json.Marshal(payload)
}

func appendAppStudioOutbox(tx *gorm.DB, aggregateType, aggregateID, eventType, idempotencyKey string, version int64, payload map[string]any) error {
	raw, err := marshalAppStudioEventPayload(payload, version, time.Now())
	if err != nil {
		return err
	}
	event := &iapiserver.AppStudioOutbox{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AggregateType: aggregateType, AggregateID: aggregateID, EventType: eventType, Payload: raw, IdempotencyKey: idempotencyKey, DeliveryStatus: "PENDING", NextAttemptAt: imachinery.Now()}
	return tx.Create(event).Error
}

func studioApplicationLifecyclePayload(app *iapiserver.StudioApplication, fromStatus any) map[string]any {
	return map[string]any{"studio_application_id": app.ID, "owner_user_id": app.OwnerUserID, "from_status": fromStatus, "to_status": app.Status}
}

func studioSourceRevisionPayload(appID string, currentRevision int64, changeSet *iapiserver.StudioChangeSet) map[string]any {
	payload := map[string]any{"studio_application_id": appID, "previous_revision": nil, "current_revision": currentRevision, "change_set_id": nil, "agent_id": nil, "agent_invocation_id": nil}
	if changeSet != nil {
		payload["previous_revision"] = changeSet.BaseRevision
		payload["change_set_id"] = changeSet.ID
		payload["agent_id"] = agentNullableString(changeSet.AgentID)
		payload["agent_invocation_id"] = agentNullableString(changeSet.AgentInvocationID)
	}
	return payload
}

func studioSourceSnapshotPayload(snapshot *iapiserver.StudioSourceSnapshot) map[string]any {
	return map[string]any{"source_snapshot_id": snapshot.ID, "studio_application_id": snapshot.StudioApplicationID, "source_revision": snapshot.WorkspaceRevision, "content_digest": snapshot.ContentDigest, "manifest_digest": snapshot.ManifestDigest, "created_by": snapshot.CreatedBy}
}

func appendStudioBuildOutbox(tx *gorm.DB, previous, build *iapiserver.StudioBuild) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, "StudioBuild", build.ID, studioBuildProjectionChangedEvent, appStudioEventKey(studioBuildProjectionChangedEvent, build.ID, build.ResourceVersion), build.ResourceVersion, studioBuildPayload(build, from))
}
func studioBuildPayload(build *iapiserver.StudioBuild, fromStatus any) map[string]any {
	return map[string]any{"studio_build_id": build.ID, "studio_application_id": build.StudioApplicationID, "source_snapshot_id": build.SourceSnapshotID, "atomic_task_id": agentNullableString(build.AtomicTaskID), "artifact_id": agentNullableString(build.ArtifactID), "artifact_digest": agentNullableString(build.ArtifactDigest), "from_status": fromStatus, "to_status": build.Status, "error_code": nil}
}
func appendStudioPreviewOutbox(tx *gorm.DB, previous, runtime *iapiserver.StudioPreviewRuntime) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, "StudioPreviewRuntime", runtime.ID, studioPreviewRuntimeChangedEvent, appStudioEventKey(studioPreviewRuntimeChangedEvent, runtime.ID, runtime.ResourceVersion), runtime.ResourceVersion, studioPreviewPayload(runtime, from))
}
func studioPreviewPayload(runtime *iapiserver.StudioPreviewRuntime, fromStatus any) map[string]any {
	diagnosticsSummary := runtime.DiagnosticsSummary
	if len(diagnosticsSummary) == 0 {
		diagnosticsSummary = json.RawMessage("{}")
	}
	return map[string]any{"preview_runtime_id": runtime.ID, "studio_application_id": runtime.StudioApplicationID, "source_revision": runtime.WorkspaceRevision, "from_status": fromStatus, "to_status": runtime.Status, "diagnostics_summary": diagnosticsSummary, "error_code": nil}
}
func appendStudioReleaseOutbox(tx *gorm.DB, previous, release *iapiserver.StudioRelease) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, "StudioRelease", release.ID, studioReleaseStatusChangedEvent, appStudioEventKey(studioReleaseStatusChangedEvent, release.ID, release.ResourceVersion), release.ResourceVersion, studioReleasePayload(release, from))
}
func studioReleasePayload(release *iapiserver.StudioRelease, fromStatus any) map[string]any {
	return map[string]any{"studio_release_id": release.ID, "studio_application_id": release.StudioApplicationID, "studio_application_version_id": release.StudioApplicationVersionID, "studio_build_id": release.StudioBuildID, "runtime_config_id": release.RuntimeConfigID, "artifact_id": release.ArtifactID, "artifact_digest": release.ArtifactDigest, "environment": release.Environment, "runtime_instance_id": agentNullableString(release.RuntimeInstanceID), "rollback_of_release_id": agentNullableString(release.RollbackOfReleaseID), "from_status": fromStatus, "to_status": release.Status, "error_code": nil}
}
func appendStudioRuntimeOutbox(tx *gorm.DB, previous, runtime *iapiserver.StudioRuntimeInstance) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	return appendAppStudioOutbox(tx, "StudioRuntimeInstance", runtime.ID, studioRuntimeInstanceChangedEvent, appStudioEventKey(studioRuntimeInstanceChangedEvent, runtime.ID, runtime.ResourceVersion), runtime.ResourceVersion, studioRuntimePayload(runtime, from))
}
func studioRuntimePayload(runtime *iapiserver.StudioRuntimeInstance, fromStatus any) map[string]any {
	return map[string]any{"runtime_instance_id": runtime.ID, "studio_release_id": runtime.StudioReleaseID, "studio_application_id": runtime.StudioApplicationID, "environment": runtime.Environment, "atomic_task_id": agentNullableString(runtime.AtomicTaskID), "infra_runtime_id": agentNullableString(runtime.InfraRuntimeID), "from_status": fromStatus, "to_status": runtime.Status, "health_status": runtime.HealthStatus, "is_current": runtime.IsCurrent, "error_code": agentNullableString(runtime.ErrorCode)}
}

package assetlibrary

import (
	"context"
	"encoding/json"
	"testing"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestStorageInspectionRejectsNonAdminBeforeRepository(t *testing.T) {
	repository := &storageInspectionRepositoryFake{}
	authorizer := NewRoleStorageAdminAuthorizer(
		roleStoreFake{items: []*iapiserver.Role{{ObjectMeta: objectMeta("role-user", "USER")}}},
		userRoleStoreFake{items: []*iapiserver.UserRole{{UserID: "user-1", RoleID: "role-user"}}},
	)
	service := &service{storageInspection: repository, storageAdmin: authorizer}
	ctx := storageUserContext("user-1")

	if _, err := service.GetBlob(ctx, "blob-1"); toolerrors.ToStatus(err).Code != code.ErrAssetStoragePermissionDenied {
		t.Fatalf("GetBlob error = %v, want storage permission denied", err)
	}
	if _, err := service.ListStorageBackends(ctx, &iapiserver.StorageBackendListRequest{}); toolerrors.ToStatus(err).Code != code.ErrAssetStoragePermissionDenied {
		t.Fatalf("ListStorageBackends error = %v, want storage permission denied", err)
	}
	if _, err := service.CreateStorageBackend(ctx, &iapiserver.StorageBackendCreateRequest{}); toolerrors.ToStatus(err).Code != code.ErrAssetStoragePermissionDenied {
		t.Fatalf("CreateStorageBackend error = %v, want storage permission denied", err)
	}
	if _, err := service.GetStorageBackend(ctx, "backend-1"); toolerrors.ToStatus(err).Code != code.ErrAssetStoragePermissionDenied {
		t.Fatalf("GetStorageBackend error = %v, want storage permission denied", err)
	}
	if _, err := service.UpdateStorageBackend(ctx, "backend-1", &iapiserver.StorageBackendUpdateRequest{}); toolerrors.ToStatus(err).Code != code.ErrAssetStoragePermissionDenied {
		t.Fatalf("UpdateStorageBackend error = %v, want storage permission denied", err)
	}
	if repository.calls != 0 {
		t.Fatalf("repository was accessed %d times before administrator authorization", repository.calls)
	}
}

func TestStorageInspectionAcceptsReleasedAdministratorRoles(t *testing.T) {
	for _, roleName := range []string{"ADMIN", "SUPER_ADMIN"} {
		t.Run(roleName, func(t *testing.T) {
			repository := &storageInspectionRepositoryFake{}
			authorizer := NewRoleStorageAdminAuthorizer(
				roleStoreFake{items: []*iapiserver.Role{{ObjectMeta: objectMeta("role-admin", roleName)}}},
				userRoleStoreFake{items: []*iapiserver.UserRole{{UserID: "admin-1", RoleID: "role-admin"}}},
			)
			service := &service{storageInspection: repository, storageAdmin: authorizer}

			if _, err := service.ListStorageBackends(storageUserContext("admin-1"), &iapiserver.StorageBackendListRequest{}); err != nil {
				t.Fatalf("ListStorageBackends for %s: %v", roleName, err)
			}
			if repository.calls != 1 {
				t.Fatalf("repository calls for %s = %d, want 1", roleName, repository.calls)
			}
		})
	}
}

func TestStorageInspectionReturnsCompleteAdminProjection(t *testing.T) {
	backend := &iapiserver.StorageBackend{
		ObjectMeta: objectMeta("backend-1", "primary"), Type: iapiserver.StorageBackendTypeLocal,
		Root: "/srv/omnimam", Config: map[string]any{"access_key": "secret"}, Enabled: true, Quota: 1024,
	}
	blob := &iapiserver.AssetBlob{
		ObjectMeta: objectMeta("blob-1", "blob"), StorageBackendID: backend.ID,
		ObjectKey: "blobs/aa/content", SHA256: "aabb", SizeBytes: 42, MIMEType: "image/png", Status: "available",
	}
	repository := &storageInspectionRepositoryFake{backend: backend, blob: blob}
	authorizer := NewRoleStorageAdminAuthorizer(
		roleStoreFake{items: []*iapiserver.Role{{ObjectMeta: objectMeta("role-admin", "ADMIN")}}},
		userRoleStoreFake{items: []*iapiserver.UserRole{{UserID: "admin-1", RoleID: "role-admin"}}},
	)
	service := &service{storageInspection: repository, storageAdmin: authorizer}
	ctx := storageUserContext("admin-1")

	blobDetail, err := service.GetBlob(ctx, blob.ID)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if blobDetail.ObjectKey != blob.ObjectKey || blobDetail.StorageBackendID != backend.ID {
		t.Fatalf("blob detail = %#v", blobDetail)
	}

	list, err := service.ListStorageBackends(ctx, &iapiserver.StorageBackendListRequest{})
	if err != nil {
		t.Fatalf("ListStorageBackends: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 || len(list.Backends) != 1 || list.Items[0] != list.Backends[0] {
		t.Fatalf("list aliases are not the same single-query projection: %#v", list)
	}
	if list.Items[0].Root != backend.Root || list.Items[0].Config["access_key"] != "secret" {
		t.Fatalf("administrator projection omitted physical configuration: %#v", list.Items[0])
	}
	encoded, err := json.Marshal(list.Items[0])
	if err != nil {
		t.Fatalf("marshal storage backend: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode storage backend: %v", err)
	}
	for _, field := range []string{"description", "extend", "root", "config"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("required field %q is absent from response: %s", field, encoded)
		}
	}
}

func TestStorageInspectionMapsReleasedNotFoundErrors(t *testing.T) {
	repository := &storageInspectionRepositoryFake{getBlobErr: gorm.ErrRecordNotFound, getBackendErr: gorm.ErrRecordNotFound}
	service := &service{
		storageInspection: repository,
		storageAdmin:      NewRoleStorageAdminAuthorizer(nil, nil),
	}
	ctx := storageUserContext("system-admin")

	if _, err := service.GetBlob(ctx, "missing"); toolerrors.ToStatus(err).Code != code.ErrAssetBlobNotFound {
		t.Fatalf("GetBlob error = %v, want asset blob not found", err)
	}
	if _, err := service.GetStorageBackend(ctx, "missing"); toolerrors.ToStatus(err).Code != code.ErrAssetStorageBackendNotFound {
		t.Fatalf("GetStorageBackend error = %v, want storage backend not found", err)
	}
}

func TestStorageInspectionCreateAndUpdatePreserveReleasedFields(t *testing.T) {
	repository := &storageInspectionRepositoryFake{}
	service := &service{
		storageInspection: repository,
		storageAdmin:      NewRoleStorageAdminAuthorizer(nil, nil),
	}
	ctx := storageUserContext("system-admin")

	created, err := service.CreateStorageBackend(ctx, &iapiserver.StorageBackendCreateRequest{
		Name: "archive", Type: iapiserver.StorageBackendTypeS3,
		Config: map[string]any{"bucket": "media", "secret": "value"}, Quota: 2048,
	})
	if err != nil {
		t.Fatalf("CreateStorageBackend: %v", err)
	}
	if !created.Enabled || created.Readonly || created.Config["secret"] != "value" || created.Quota != 2048 {
		t.Fatalf("created backend = %#v", created)
	}

	readonly := true
	quota := int64(4096)
	config := map[string]any{"bucket": "cold", "secret": "rotated"}
	updated, err := service.UpdateStorageBackend(ctx, repository.backend.ID, &iapiserver.StorageBackendUpdateRequest{
		Config: &config, Readonly: &readonly, Quota: &quota,
	})
	if err != nil {
		t.Fatalf("UpdateStorageBackend: %v", err)
	}
	if !updated.Readonly || updated.Quota != quota || updated.Config["secret"] != "rotated" {
		t.Fatalf("updated backend = %#v", updated)
	}
}

func objectMeta(id, name string) imachinery.ObjectMeta {
	return imachinery.ObjectMeta{ID: id, Name: name}
}

type storageInspectionRepositoryFake struct {
	backend       *iapiserver.StorageBackend
	blob          *iapiserver.AssetBlob
	getBlobErr    error
	getBackendErr error
	calls         int
}

func (s *storageInspectionRepositoryFake) GetBlob(context.Context, string) (*iapiserver.AssetBlob, error) {
	s.calls++
	return s.blob, s.getBlobErr
}

func (s *storageInspectionRepositoryFake) List(context.Context, *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error) {
	s.calls++
	if s.backend == nil {
		return []*iapiserver.StorageBackend{}, 0, nil
	}
	return []*iapiserver.StorageBackend{s.backend}, 1, nil
}

func (s *storageInspectionRepositoryFake) Get(context.Context, string) (*iapiserver.StorageBackend, error) {
	s.calls++
	return s.backend, s.getBackendErr
}

func (s *storageInspectionRepositoryFake) Add(_ context.Context, backend *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error) {
	s.calls++
	backend.ID = "backend-created"
	s.backend = backend
	return backend, nil
}

func (s *storageInspectionRepositoryFake) Update(_ context.Context, backend *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error) {
	s.calls++
	s.backend = backend
	return backend, nil
}

type roleStoreFake struct{ items []*iapiserver.Role }

func (s roleStoreFake) List(context.Context) ([]*iapiserver.Role, error) { return s.items, nil }

type userRoleStoreFake struct{ items []*iapiserver.UserRole }

func (s userRoleStoreFake) ListByUser(context.Context, string) ([]*iapiserver.UserRole, error) {
	return s.items, nil
}

func storageUserContext(id string) context.Context {
	user := &iapiserver.User{}
	user.ID = id
	return context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
}

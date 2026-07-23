package assetlibrary

import (
	"context"
	stderrors "errors"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

// StorageInspectionStore 是 storage-inspection 消费的全局 Blob 与 StorageBackend repository。
// 调用方必须先通过 StorageAdminAuthorizer；普通素材查询不得依赖该接口。
type StorageInspectionStore interface {
	GetBlob(context.Context, string) (*iapiserver.AssetBlob, error)
	List(context.Context, *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error)
	Get(context.Context, string) (*iapiserver.StorageBackend, error)
	Add(context.Context, *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	Update(context.Context, *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
}

// StorageAdminAuthorizer 在 repository 访问前验证当前用户具有 ADMIN 或 SUPER_ADMIN 角色。
type StorageAdminAuthorizer interface {
	RequireStorageAdmin(context.Context) error
}

type roleStorageAdminAuthorizer struct {
	roles     store.RoleStore
	userRoles store.UserRoleStore
}

// NewRoleStorageAdminAuthorizer 使用 identity 角色事实源构造管理员鉴权器。
func NewRoleStorageAdminAuthorizer(roles store.RoleStore, userRoles store.UserRoleStore) StorageAdminAuthorizer {
	return &roleStorageAdminAuthorizer{roles: roles, userRoles: userRoles}
}

func (a *roleStorageAdminAuthorizer) RequireStorageAdmin(ctx context.Context) error {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return errors.NewStatus(code.ErrAssetStoragePermissionDenied, "authenticated administrator is required")
	}
	if user.ID == "system-admin" {
		return nil
	}
	if a.roles == nil || a.userRoles == nil {
		return errors.New("storage administrator authorizer is not configured")
	}
	assignments, err := a.userRoles.ListByUser(ctx, user.ID)
	if err != nil {
		return errors.WithStack(err)
	}
	roles, err := a.roles.List(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	roleNames := make(map[string]string, len(roles))
	for _, role := range roles {
		roleNames[role.ID] = strings.ToUpper(strings.TrimSpace(role.Name))
	}
	for _, assignment := range assignments {
		switch roleNames[assignment.RoleID] {
		case "ADMIN", "SUPER_ADMIN":
			return nil
		}
	}
	return errors.NewStatus(code.ErrAssetStoragePermissionDenied, "administrator permission is required")
}

func (s *service) GetBlob(ctx context.Context, id string) (*iapiserver.AssetBlobDetail, error) {
	if err := s.authorizeStorageInspection(ctx); err != nil {
		return nil, err
	}
	blob, err := s.storageInspection.GetBlob(ctx, id)
	if err != nil {
		return nil, mapStorageInspectionNotFound(err, code.ErrAssetBlobNotFound)
	}
	return projectBlobDetail(blob), nil
}

func (s *service) ListStorageBackends(ctx context.Context, req *iapiserver.StorageBackendListRequest) (*iapiserver.StorageBackendListResponse, error) {
	if err := s.authorizeStorageInspection(ctx); err != nil {
		return nil, err
	}
	items, total, err := s.storageInspection.List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	projected := make([]*iapiserver.StorageBackendDetail, 0, len(items))
	for _, item := range items {
		projected = append(projected, projectStorageBackend(item))
	}
	return &iapiserver.StorageBackendListResponse{
		ListRet: imachinery.ListRet{Total: total},
		Items:   projected, Backends: projected,
	}, nil
}

func (s *service) CreateStorageBackend(ctx context.Context, req *iapiserver.StorageBackendCreateRequest) (*iapiserver.StorageBackendDetail, error) {
	if err := s.authorizeStorageInspection(ctx); err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	readonly := false
	if req.Readonly != nil {
		readonly = *req.Readonly
	}
	backend := &iapiserver.StorageBackend{
		Type: req.Type, Root: req.Root, Config: maps.Clone(req.Config),
		Enabled: enabled, Readonly: readonly, Quota: req.Quota,
	}
	backend.Name = req.Name
	if backend.Config == nil {
		backend.Config = map[string]any{}
	}
	if err := normalizeStorageBackend(backend); err != nil {
		return nil, err
	}
	created, err := s.storageInspection.Add(ctx, backend)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return projectStorageBackend(created), nil
}

func (s *service) GetStorageBackend(ctx context.Context, id string) (*iapiserver.StorageBackendDetail, error) {
	if err := s.authorizeStorageInspection(ctx); err != nil {
		return nil, err
	}
	backend, err := s.storageInspection.Get(ctx, id)
	if err != nil {
		return nil, mapStorageInspectionNotFound(err, code.ErrAssetStorageBackendNotFound)
	}
	return projectStorageBackend(backend), nil
}

func (s *service) UpdateStorageBackend(ctx context.Context, id string, req *iapiserver.StorageBackendUpdateRequest) (*iapiserver.StorageBackendDetail, error) {
	if err := s.authorizeStorageInspection(ctx); err != nil {
		return nil, err
	}
	backend, err := s.storageInspection.Get(ctx, id)
	if err != nil {
		return nil, mapStorageInspectionNotFound(err, code.ErrAssetStorageBackendNotFound)
	}
	if req.Name != nil {
		backend.Name = *req.Name
	}
	if req.Type != nil {
		backend.Type = *req.Type
	}
	if req.Root != nil {
		backend.Root = *req.Root
	}
	if req.Config != nil {
		backend.Config = maps.Clone(*req.Config)
	}
	if req.Enabled != nil {
		backend.Enabled = *req.Enabled
	}
	if req.Readonly != nil {
		backend.Readonly = *req.Readonly
	}
	if req.Quota != nil {
		backend.Quota = *req.Quota
	}
	if err := normalizeStorageBackend(backend); err != nil {
		return nil, err
	}
	updated, err := s.storageInspection.Update(ctx, backend)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return projectStorageBackend(updated), nil
}

func (s *service) authorizeStorageInspection(ctx context.Context) error {
	if s.storageAdmin == nil || s.storageInspection == nil {
		return errors.New("storage inspection service is not configured")
	}
	return s.storageAdmin.RequireStorageAdmin(ctx)
}

func normalizeStorageBackend(backend *iapiserver.StorageBackend) error {
	if backend.Type != iapiserver.StorageBackendTypeLocal {
		return nil
	}
	root := backend.Root
	if root == "" {
		root = os.Getenv("OMNIMAM_STORAGE_ROOT")
	}
	if root == "" {
		root = filepath.Join("data", "assets")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return errors.WithStack(err)
	}
	backend.Root = filepath.Clean(abs)
	return nil
}

func mapStorageInspectionNotFound(err error, errorCode int) error {
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(errorCode, "storage resource does not exist")
	}
	return errors.WithStack(err)
}

func projectStorageBackend(item *iapiserver.StorageBackend) *iapiserver.StorageBackendDetail {
	if item == nil {
		return nil
	}
	return &iapiserver.StorageBackendDetail{
		ID: item.ID, Name: item.Name, Description: item.Description, Extend: cloneStringAny(item.Extend),
		Type: item.Type, Root: item.Root, Config: cloneStringAny(item.Config), Enabled: item.Enabled,
		Readonly: item.Readonly, Quota: item.Quota, ResourceVersion: item.ResourceVersion,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func projectBlobDetail(item *iapiserver.AssetBlob) *iapiserver.AssetBlobDetail {
	if item == nil {
		return nil
	}
	return &iapiserver.AssetBlobDetail{
		ID: item.ID, Name: item.Name, Description: item.Description, Extend: cloneStringAny(item.Extend),
		StorageBackendID: item.StorageBackendID, ObjectKey: item.ObjectKey, SHA256: item.SHA256,
		SizeBytes: item.SizeBytes, MIMEType: item.MIMEType, Status: item.Status,
		ResourceVersion: item.ResourceVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func cloneStringAny(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	return maps.Clone(source)
}

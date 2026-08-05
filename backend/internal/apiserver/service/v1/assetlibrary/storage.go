package assetlibrary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ContentStorage 是 asset-library 消费的受控对象存储边界。
type ContentStorage interface {
	WriteUploadPart(context.Context, *iapiserver.AssetUploadSession, int, io.Reader) (iapiserver.UploadedPart, error)
	FinalizeUpload(context.Context, *iapiserver.AssetUploadSession) (store.StoredAssetContent, error)
	CancelUpload(context.Context, *iapiserver.AssetUploadSession) error
	WriteArtifact(context.Context, string, string, io.Reader) (store.StoredAssetContent, error)
	WriteDerived(context.Context, string, io.Reader) (store.StoredAssetContent, error)
	Open(context.Context, store.StoredAssetContent) (io.ReadCloser, error)
	Delete(context.Context, store.StoredAssetContent) error
}

// LocalContentStorage 只实现 local StorageBackend；路径解析和文件系统调用不进入业务 service。
type LocalContentStorage struct{ factory store.Factory }

func NewLocalContentStorage(factory store.Factory) *LocalContentStorage {
	return &LocalContentStorage{factory: factory}
}

// ReconcileDefaultLocalStorageBackend 在 API Server 启动时确保首期 local StorageAdapter 有可写后端。
// 已有可写 local 后端时保持管理员配置不变；缺失时使用部署环境根目录创建默认配置。
func ReconcileDefaultLocalStorageBackend(
	ctx context.Context,
	target store.StorageBackendStore,
) (*iapiserver.StorageBackend, error) {
	if target == nil {
		return nil, errors.Errorf("storage backend store is unavailable")
	}
	root, err := defaultLocalStorageRoot()
	if err != nil {
		return nil, err
	}
	desired := &iapiserver.StorageBackend{
		Type:    iapiserver.StorageBackendTypeLocal,
		Root:    root,
		Config:  map[string]any{},
		Enabled: true,
	}
	desired.Name = "default-local"
	return target.EnsureDefaultLocal(ctx, desired)
}

func defaultLocalStorageRoot() (string, error) {
	root := os.Getenv("OMNIMAM_STORAGE_ROOT")
	if strings.TrimSpace(root) == "" {
		root = filepath.Join("data", "assets")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return filepath.Clean(abs), nil
}

func (s *LocalContentStorage) WriteUploadPart(ctx context.Context, upload *iapiserver.AssetUploadSession, partNumber int, reader io.Reader) (iapiserver.UploadedPart, error) {
	_, root, err := s.localBackend(ctx)
	if err != nil {
		return iapiserver.UploadedPart{}, err
	}
	if partNumber < 1 {
		return iapiserver.UploadedPart{}, errors.Errorf("part number must be positive")
	}
	relative := filepath.ToSlash(filepath.Join(".uploads", upload.ID, fmt.Sprintf("part-%08d", partNumber)))
	path, err := secureJoin(root, relative)
	if err != nil {
		return iapiserver.UploadedPart{}, err
	}
	size, checksum, err := writeAtomic(ctx, path, reader)
	if err != nil {
		return iapiserver.UploadedPart{}, err
	}
	return iapiserver.UploadedPart{PartNumber: partNumber, SizeBytes: size, SHA256: checksum}, nil
}

func (s *LocalContentStorage) FinalizeUpload(ctx context.Context, upload *iapiserver.AssetUploadSession) (store.StoredAssetContent, error) {
	backend, root, err := s.localBackend(ctx)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	parts := append([]iapiserver.UploadedPart(nil), upload.UploadedParts...)
	if upload.UploadMode == "single" && len(parts) == 0 {
		parts = []iapiserver.UploadedPart{{PartNumber: 1}}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	objectKey := filepath.ToSlash(filepath.Join("blobs", upload.SHA256[:2], upload.SHA256))
	destination, err := secureJoin(root, objectKey)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".asset-upload-*")
	if err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	tempName := temp.Name()
	defer removeTemporary(tempName)
	hasher := sha256.New()
	writer := io.MultiWriter(temp, hasher)
	var total int64
	for _, part := range parts {
		partPath, joinErr := secureJoin(root, filepath.ToSlash(filepath.Join(".uploads", upload.ID, fmt.Sprintf("part-%08d", part.PartNumber))))
		if joinErr != nil {
			closeAfterFailure(temp)
			return store.StoredAssetContent{}, joinErr
		}
		file, openErr := os.Open(partPath)
		if openErr != nil {
			closeAfterFailure(temp)
			return store.StoredAssetContent{}, errors.WithStack(openErr)
		}
		written, copyErr := copyWithContext(ctx, writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			closeAfterFailure(temp)
			return store.StoredAssetContent{}, copyErr
		}
		if closeErr != nil {
			closeAfterFailure(temp)
			return store.StoredAssetContent{}, errors.WithStack(closeErr)
		}
		total += written
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if total != upload.SizeBytes || !strings.EqualFold(checksum, upload.SHA256) {
		closeAfterFailure(temp)
		return store.StoredAssetContent{}, errors.Errorf("uploaded content checksum or size mismatch")
	}
	if err := temp.Sync(); err != nil {
		closeAfterFailure(temp)
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	if err := temp.Close(); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	if err := os.Rename(tempName, destination); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	if err := s.CancelUpload(ctx, upload); err != nil {
		return store.StoredAssetContent{}, err
	}
	return store.StoredAssetContent{StorageBackendID: backend.ID, ObjectKey: objectKey, SHA256: checksum, SizeBytes: total, MIMEType: upload.MIMEType}, nil
}

func (s *LocalContentStorage) CancelUpload(ctx context.Context, upload *iapiserver.AssetUploadSession) error {
	_, root, err := s.localBackend(ctx)
	if err != nil {
		return err
	}
	dir, err := secureJoin(root, filepath.ToSlash(filepath.Join(".uploads", upload.ID)))
	if err != nil {
		return err
	}
	return errors.WithStack(os.RemoveAll(dir))
}

func (s *LocalContentStorage) WriteArtifact(ctx context.Context, artifactID, mimeType string, reader io.Reader) (store.StoredAssetContent, error) {
	backend, root, err := s.localBackend(ctx)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	temporary := filepath.ToSlash(filepath.Join(".artifacts", artifactID, "incoming"))
	path, err := secureJoin(root, temporary)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	size, checksum, err := writeAtomic(ctx, path, reader)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("blobs", checksum[:2], checksum))
	destination, err := secureJoin(root, objectKey)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	if err := os.Rename(path, destination); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	return store.StoredAssetContent{StorageBackendID: backend.ID, ObjectKey: objectKey, SHA256: checksum, SizeBytes: size, MIMEType: mimeType}, nil
}

// WriteDerived 将 Worker 生成的派生媒体按内容摘要写入同一受控 Blob 空间。
func (s *LocalContentStorage) WriteDerived(ctx context.Context, mimeType string, reader io.Reader) (store.StoredAssetContent, error) {
	backend, root, err := s.localBackend(ctx)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	temporary := filepath.ToSlash(filepath.Join(".representations", uuid.NewString()))
	path, err := secureJoin(root, temporary)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	size, checksum, err := writeAtomic(ctx, path, reader)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("blobs", checksum[:2], checksum))
	destination, err := secureJoin(root, objectKey)
	if err != nil {
		return store.StoredAssetContent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	if err := os.Rename(path, destination); err != nil {
		return store.StoredAssetContent{}, errors.WithStack(err)
	}
	return store.StoredAssetContent{StorageBackendID: backend.ID, ObjectKey: objectKey, SHA256: checksum, SizeBytes: size, MIMEType: mimeType}, nil
}

func (s *LocalContentStorage) Open(ctx context.Context, content store.StoredAssetContent) (io.ReadCloser, error) {
	backend, err := s.factory.StorageBackends().Get(ctx, content.StorageBackendID)
	if err != nil {
		return nil, err
	}
	root := storageRoot(backend)
	path, err := secureJoin(root, content.ObjectKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return file, nil
}

func (s *LocalContentStorage) Delete(ctx context.Context, content store.StoredAssetContent) error {
	backend, err := s.factory.StorageBackends().Get(ctx, content.StorageBackendID)
	if err != nil {
		return err
	}
	path, err := secureJoin(storageRoot(backend), content.ObjectKey)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return errors.WithStack(err)
}

func (s *LocalContentStorage) localBackend(ctx context.Context) (*iapiserver.StorageBackend, string, error) {
	backend, err := s.factory.StorageBackends().GetDefaultLocal(ctx)
	if err != nil {
		return nil, "", err
	}
	if backend == nil || !backend.Enabled || backend.Readonly || backend.Type != "local" {
		return nil, "", errors.Errorf("writable local storage backend is unavailable")
	}
	root := storageRoot(backend)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, "", errors.WithStack(err)
	}
	return backend, root, nil
}

func storageRoot(backend *iapiserver.StorageBackend) string {
	if value, ok := backend.Config["root_path"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if strings.TrimSpace(backend.Root) != "" {
		return backend.Root
	}
	return filepath.Join("data", "assets")
}

func secureJoin(root, objectKey string) (string, error) {
	if root == "" || objectKey == "" || filepath.IsAbs(objectKey) || !filepath.IsLocal(objectKey) {
		return "", errors.Errorf("invalid storage object key")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", errors.WithStack(err)
	}
	path := filepath.Join(rootAbs, filepath.FromSlash(objectKey))
	relative, err := filepath.Rel(rootAbs, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.Errorf("storage object key escapes backend root")
	}
	return path, nil
}

func writeAtomic(ctx context.Context, path string, reader io.Reader) (int64, string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0, "", errors.WithStack(err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".asset-content-*")
	if err != nil {
		return 0, "", errors.WithStack(err)
	}
	name := temp.Name()
	defer removeTemporary(name)
	hasher := sha256.New()
	size, err := copyWithContext(ctx, io.MultiWriter(temp, hasher), reader)
	if err != nil {
		closeAfterFailure(temp)
		return 0, "", err
	}
	if err := temp.Sync(); err != nil {
		closeAfterFailure(temp)
		return 0, "", errors.WithStack(err)
	}
	if err := temp.Close(); err != nil {
		return 0, "", errors.WithStack(err)
	}
	if err := os.Rename(name, path); err != nil {
		return 0, "", errors.WithStack(err)
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 128*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := src.Read(buffer)
		if read > 0 {
			written, writeErr := dst.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, errors.WithStack(writeErr)
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, errors.WithStack(readErr)
		}
	}
}

func removeTemporary(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Warnf("remove temporary asset content %s: %v", path, err)
	}
}

func closeAfterFailure(file *os.File) {
	if err := file.Close(); err != nil {
		log.Warnf("close temporary asset content %s: %v", file.Name(), err)
	}
}

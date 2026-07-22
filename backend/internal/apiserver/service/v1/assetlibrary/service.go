package assetlibrary

import (
	"context"
	stderrors "errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const defaultUploadChunkSize = int64(8 * 1024 * 1024)

type ContentRead struct {
	Reader    io.ReadCloser
	MIMEType  string
	SizeBytes int64
	SHA256    string
	Filename  string
}

// Service 实现 spec-v1.5.1 asset-library 公共契约并强制当前用户隔离。
type Service interface {
	ListAssets(context.Context, *iapiserver.UserAssetListRequest) (*iapiserver.UserAssetListResponse, error)
	CreateAsset(context.Context, *iapiserver.CreateCanonicalAssetRequest) (*iapiserver.AssetDetail, error)
	GetAsset(context.Context, string) (*iapiserver.AssetDetail, error)
	UpdateAsset(context.Context, string, *iapiserver.UpdateUserAssetRequest) (*iapiserver.UserAsset, error)
	DeleteAsset(context.Context, string) (*iapiserver.UserAsset, error)
	RestoreAsset(context.Context, string) (*iapiserver.UserAsset, error)
	PermanentlyDeleteAsset(context.Context, string) (*iapiserver.PermanentDeleteResult, error)
	BatchLabels(context.Context, *iapiserver.BatchLabelRequest) (*iapiserver.BatchLabelResponse, error)
	CreateUploads(context.Context, *iapiserver.CreateAssetUploadsRequest) (*iapiserver.CreateAssetUploadsResponse, error)
	UploadContent(context.Context, string, int, string, string, io.Reader) (*iapiserver.AssetUploadSession, error)
	CompleteUpload(context.Context, string, *iapiserver.CompleteAssetUploadRequest) (*iapiserver.CompleteAssetUploadResponse, error)
	CancelUpload(context.Context, string) (*iapiserver.AssetUploadSession, error)
	ListCollections(context.Context, *iapiserver.CollectionListRequest) (*iapiserver.CollectionListResponse, error)
	GetCollection(context.Context, string, imachinery.PagingParams) (*iapiserver.CollectionDetail, error)
	CreateCollection(context.Context, *iapiserver.CreateCollectionRequest) (*iapiserver.AssetCollection, error)
	UpdateCollection(context.Context, string, *iapiserver.UpdateCollectionRequest) (*iapiserver.AssetCollection, error)
	DeleteCollection(context.Context, string) (*iapiserver.DeleteResult, error)
	AddCollectionItems(context.Context, string, *iapiserver.AddCollectionItemsRequest) (*iapiserver.CollectionItemBatchResponse, error)
	UpdateCollectionItem(context.Context, string, string, *iapiserver.UpdateCollectionItemRequest) (*iapiserver.AssetCollectionItem, error)
	DeleteCollectionItem(context.Context, string, string) (*iapiserver.DeleteResult, error)
	ReplaceLabels(context.Context, string, *iapiserver.ReplaceLabelsRequest) (*iapiserver.BatchLabelData, error)
	DeleteLabel(context.Context, string, string) (*iapiserver.BatchLabelData, error)
	AddTags(context.Context, string, *iapiserver.AddTagsRequest) (*iapiserver.BatchLabelData, error)
	DeleteTag(context.Context, string, string) (*iapiserver.BatchLabelData, error)
	ListArtifacts(context.Context, *iapiserver.ArtifactListRequest) (*iapiserver.ArtifactListResponse, error)
	BatchArtifactSummaries(context.Context, *iapiserver.BatchArtifactSummaryRequest) (*iapiserver.BatchArtifactSummaryResponse, error)
	CreateArtifact(context.Context, *iapiserver.CreateArtifactRequest) (*iapiserver.Artifact, error)
	GetArtifact(context.Context, string) (*iapiserver.Artifact, error)
	UploadArtifactContent(context.Context, string, string, io.Reader) (*iapiserver.Artifact, error)
	CompleteArtifact(context.Context, string, *iapiserver.CompleteArtifactRequest) (*iapiserver.Artifact, error)
	DeleteArtifact(context.Context, string) (*iapiserver.Artifact, error)
	RegisterArtifact(context.Context, string, *iapiserver.RegisterArtifactRequest) (*iapiserver.ArtifactRegistrationResponse, error)
	ListVersions(context.Context, string) (*iapiserver.AssetVersionListResponse, error)
	CreateVersion(context.Context, string, *iapiserver.CreateCanonicalVersionRequest) (*iapiserver.AssetVersion, error)
	GetVersion(context.Context, string) (*iapiserver.AssetVersionDetail, error)
	SetCurrentVersion(context.Context, string, string) (*iapiserver.UserAsset, error)
	ListRepresentations(context.Context, string) (*iapiserver.AssetRepresentationListResponse, error)
	RegisterRepresentation(context.Context, string, *iapiserver.RegisterRepresentationRequest) (*iapiserver.AssetRepresentation, error)
	GetRepresentation(context.Context, string) (*iapiserver.AssetRepresentation, error)
	ReadRepresentation(context.Context, string) (*ContentRead, error)
	RepresentationAccess(context.Context, string, string) (*iapiserver.RepresentationAccess, error)
	ListRelations(context.Context, string, imachinery.PagingParams) (*iapiserver.AssetRelationListResponse, error)
	Lineage(context.Context, string) (*iapiserver.AssetLineage, error)
	ListReferences(context.Context, string, imachinery.PagingParams) (*iapiserver.AssetReferenceListResponse, error)
	ListUsages(context.Context, string, imachinery.PagingParams) (*iapiserver.AssetUsageListResponse, error)
}

type service struct {
	store   store.AssetV1Store
	storage ContentStorage
	readers RelationReaders
}

func New(factory store.Factory, storage ContentStorage) Service {
	return NewStore(factory.AssetsV1(), storage)
}

// NewStore 使用消费方接口构造服务，便于路由能力探测和单元测试替换。
func NewStore(assetStore store.AssetV1Store, storage ContentStorage) Service {
	return &service{store: assetStore, storage: storage}
}

// NewStoreWithRelations 注入 Artifact 跨领域受控摘要读取器；存储层仍只读取 asset-library 自有表。
func NewStoreWithRelations(assetStore store.AssetV1Store, storage ContentStorage, readers RelationReaders) Service {
	return &service{store: assetStore, storage: storage, readers: readers}
}

func (s *service) ListAssets(ctx context.Context, req *iapiserver.UserAssetListRequest) (*iapiserver.UserAssetListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.NaturalLanguageQuery != "" && (req.Selector != "" || req.Keyword != "") {
		return nil, errors.NewStatus(code.ErrAssetQueryParametersInvalid, "natural_language_query conflicts with selector or keyword")
	}
	if req.NaturalLanguageQuery != "" {
		return nil, errors.NewStatus(code.ErrAssetSearchDependencyFailed, "natural language asset search dependency is unavailable")
	}
	var resolvedSelector *string
	if req.Selector != "" {
		expression, canonical, parseErr := parseAssetSelector(req.Selector)
		if parseErr != nil {
			if strings.Contains(parseErr.Error(), "exceeds") {
				return nil, errors.NewStatus(code.ErrAssetSelectorTooComplex, parseErr.Error())
			}
			return nil, errors.NewStatus(code.ErrAssetSelectorInvalid, parseErr.Error())
		}
		req.SelectorExpression, resolvedSelector = expression, &canonical
	}
	items, total, err := s.store.ListUserAssets(ctx, owner, req)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAssetListFailed, "asset list query failed")
	}
	mode := "filters_only"
	if req.Selector != "" {
		mode = "selector"
	}
	if req.Keyword != "" {
		if req.Selector != "" {
			mode = "combined"
		} else {
			mode = "keyword"
		}
	}
	return &iapiserver.UserAssetListResponse{Total: total, Items: items, QueryResolution: &iapiserver.QueryResolution{RequestedMode: mode, AppliedMode: mode, ResolvedSelector: resolvedSelector}}, nil
}

func (s *service) CreateAsset(ctx context.Context, req *iapiserver.CreateCanonicalAssetRequest) (*iapiserver.AssetDetail, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateLabelsAndTags(req.Labels, req.Tags); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return nil, errors.NewStatus(code.ErrAssetNameInvalid, "asset name is required")
	}
	result, err := s.store.CreateCanonicalAsset(ctx, owner, req)
	return result, mapAssetWriteError(err)
}
func (s *service) GetAsset(ctx context.Context, id string) (*iapiserver.AssetDetail, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetAssetDetail(ctx, owner, id)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
}
func (s *service) UpdateAsset(ctx context.Context, id string, req *iapiserver.UpdateUserAssetRequest) (*iapiserver.UserAsset, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) == "" {
		return nil, errors.NewStatus(code.ErrAssetNameInvalid, "asset name is required")
	}
	result, err := s.store.UpdateUserAsset(ctx, owner, id, req)
	return result, mapAssetWriteError(err)
}
func (s *service) DeleteAsset(ctx context.Context, id string) (*iapiserver.UserAsset, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.SetUserAssetDeleted(ctx, owner, id, true)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotWritable)
}
func (s *service) RestoreAsset(ctx context.Context, id string) (*iapiserver.UserAsset, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.SetUserAssetDeleted(ctx, owner, id, false)
	if err != nil && strings.Contains(err.Error(), "not deleted") {
		return nil, errors.NewStatus(code.ErrAssetStateInvalid, "asset is not deleted")
	}
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotWritable)
}
func (s *service) PermanentlyDeleteAsset(ctx context.Context, id string) (*iapiserver.PermanentDeleteResult, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, contents, err := s.store.PermanentlyDeleteUserAsset(ctx, owner, id)
	if err != nil {
		if strings.Contains(err.Error(), "blocking") {
			return nil, errors.NewStatus(code.ErrAssetPermanentDeleteBlocked, "asset has blocking references")
		}
		return nil, mapNotFound(err, code.ErrAssetNotFoundOrNotWritable)
	}
	for _, content := range contents {
		if err := s.storage.Delete(ctx, content); err != nil {
			return result, errors.NewStatus(code.ErrAssetContentUnavailable, "deleted asset content cleanup failed")
		}
	}
	return result, nil
}

func (s *service) BatchLabels(ctx context.Context, req *iapiserver.BatchLabelRequest) (*iapiserver.BatchLabelResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateBatchLabels(req); err != nil {
		return nil, err
	}
	response := &iapiserver.BatchLabelResponse{Total: len(req.Items), Results: make([]iapiserver.BatchLabelResult, 0, len(req.Items))}
	for _, item := range req.Items {
		data, itemErr := s.store.ApplyLabels(ctx, owner, item.ID, req.LabelsToUpsert, req.TagsToAdd, req.TagsToRemove)
		result := iapiserver.BatchLabelResult{ID: item.ID, Success: itemErr == nil, Data: data}
		if itemErr != nil {
			response.Fail++
			result.Error = businessError(code.ErrAssetNotFoundOrNotWritable, "asset is not writable")
		} else {
			response.Success++
		}
		response.Results = append(response.Results, result)
	}
	return response, nil
}

func (s *service) CreateUploads(ctx context.Context, req *iapiserver.CreateAssetUploadsRequest) (*iapiserver.CreateAssetUploadsResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, item := range req.Items {
		if seen[item.ClientUploadKey] {
			return nil, errors.NewStatus(code.ErrAssetUploadRequestInvalid, "client upload keys must be unique")
		}
		seen[item.ClientUploadKey] = true
		if err := validateLabelsAndTags(item.Labels, item.Tags); err != nil {
			return nil, err
		}
	}
	results, err := s.store.CreateAssetUploads(ctx, owner, req.Items, defaultUploadChunkSize)
	if err != nil {
		return nil, mapUploadError(err)
	}
	response := &iapiserver.CreateAssetUploadsResponse{Total: len(results), Results: results}
	for _, result := range results {
		if result.Error == nil {
			response.Success++
		} else {
			response.Fail++
		}
	}
	return response, nil
}
func (s *service) UploadContent(ctx context.Context, id string, partNumber int, partSHA, contentRange string, reader io.Reader) (*iapiserver.AssetUploadSession, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	upload, err := s.store.GetAssetUpload(ctx, owner, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetUploadNotFoundOrNotVisible)
	}
	if upload.UploadMode == "single" {
		if partNumber > 0 || partSHA != "" || contentRange != "" {
			return nil, errors.NewStatus(code.ErrAssetUploadPartInvalid, "single upload must not include part headers")
		}
		partNumber = 1
	} else {
		if partNumber < 1 || partSHA == "" || contentRange == "" {
			return nil, errors.NewStatus(code.ErrAssetUploadPartInvalid, "chunked upload requires part metadata")
		}
	}
	maxPartSize := upload.SizeBytes
	if upload.UploadMode == "chunked" {
		maxPartSize = upload.ChunkSizeBytes
	}
	part, err := s.storage.WriteUploadPart(ctx, upload, partNumber, io.LimitReader(reader, maxPartSize+1))
	if err != nil {
		return nil, errors.NewStatus(code.ErrAssetUploadStorageFailed, "upload storage write failed")
	}
	if part.SizeBytes > maxPartSize {
		return nil, errors.NewStatus(code.ErrAssetUploadPartInvalid, "uploaded part exceeds session size limit")
	}
	if partSHA != "" && !strings.EqualFold(partSHA, part.SHA256) {
		return nil, errors.NewStatus(code.ErrAssetUploadPartInvalid, "part sha256 mismatch")
	}
	if contentRange != "" {
		if err := validateContentRange(contentRange, part.SizeBytes, upload.SizeBytes, partNumber, upload.ChunkSizeBytes); err != nil {
			return nil, err
		}
	}
	result, err := s.store.RecordAssetUploadPart(ctx, owner, id, part)
	return result, mapUploadError(err)
}
func (s *service) CompleteUpload(ctx context.Context, id string, req *iapiserver.CompleteAssetUploadRequest) (*iapiserver.CompleteAssetUploadResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	upload, err := s.store.GetAssetUpload(ctx, owner, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetUploadNotFoundOrNotVisible)
	}
	if upload.Status == "completed" {
		result, err := s.store.CompleteAssetUpload(ctx, owner, id, req, store.StoredAssetContent{})
		return result, mapUploadError(err)
	}
	content, err := s.storage.FinalizeUpload(ctx, upload)
	if err != nil {
		if strings.Contains(err.Error(), "checksum") {
			return nil, errors.NewStatus(code.ErrAssetUploadChecksumMismatch, "upload checksum mismatch")
		}
		return nil, errors.NewStatus(code.ErrAssetUploadStorageFailed, "upload finalization failed")
	}
	result, err := s.store.CompleteAssetUpload(ctx, owner, id, req, content)
	return result, mapUploadError(err)
}
func (s *service) CancelUpload(ctx context.Context, id string) (*iapiserver.AssetUploadSession, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	upload, err := s.store.GetAssetUpload(ctx, owner, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetUploadNotFoundOrNotVisible)
	}
	if err := s.storage.CancelUpload(ctx, upload); err != nil {
		return nil, errors.NewStatus(code.ErrAssetUploadStorageFailed, "upload cleanup failed")
	}
	result, err := s.store.CancelAssetUpload(ctx, owner, id)
	return result, mapUploadError(err)
}

func (s *service) ListCollections(ctx context.Context, req *iapiserver.CollectionListRequest) (*iapiserver.CollectionListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCollections(ctx, owner, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.CollectionListResponse{Total: total, Items: items}, nil
}
func (s *service) GetCollection(ctx context.Context, id string, paging imachinery.PagingParams) (*iapiserver.CollectionDetail, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetCollection(ctx, owner, id, paging)
	return result, mapNotFound(err, code.ErrCollectionNotFoundOrNotVisible)
}
func (s *service) CreateCollection(ctx context.Context, req *iapiserver.CreateCollectionRequest) (*iapiserver.AssetCollection, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.NewStatus(code.ErrAssetNameInvalid, "collection name is required")
	}
	result, err := s.store.CreateCollection(ctx, owner, req)
	return result, mapCollectionError(err)
}
func (s *service) UpdateCollection(ctx context.Context, id string, req *iapiserver.UpdateCollectionRequest) (*iapiserver.AssetCollection, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.UpdateCollection(ctx, owner, id, req)
	return result, mapCollectionError(err)
}
func (s *service) DeleteCollection(ctx context.Context, id string) (*iapiserver.DeleteResult, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	err = s.store.DeleteCollection(ctx, owner, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrCollectionNotFoundOrNotVisible)
	}
	return &iapiserver.DeleteResult{ID: id, Deleted: true}, nil
}
func (s *service) AddCollectionItems(ctx context.Context, id string, req *iapiserver.AddCollectionItemsRequest) (*iapiserver.CollectionItemBatchResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.AddCollectionItems(ctx, owner, id, req.Items)
}
func (s *service) UpdateCollectionItem(ctx context.Context, id, itemID string, req *iapiserver.UpdateCollectionItemRequest) (*iapiserver.AssetCollectionItem, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.UpdateCollectionItem(ctx, owner, id, itemID, req)
	return result, mapCollectionError(err)
}
func (s *service) DeleteCollectionItem(ctx context.Context, id, itemID string) (*iapiserver.DeleteResult, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.DeleteCollectionItem(ctx, owner, id, itemID); err != nil {
		return nil, mapCollectionError(err)
	}
	return &iapiserver.DeleteResult{ID: itemID, Deleted: true}, nil
}

func (s *service) ReplaceLabels(ctx context.Context, assetID string, req *iapiserver.ReplaceLabelsRequest) (*iapiserver.BatchLabelData, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateLabelsAndTags(req.Labels, nil); err != nil {
		return nil, err
	}
	result, err := s.store.ReplaceLabels(ctx, owner, assetID, req.Labels)
	return result, mapAssetWriteError(err)
}
func (s *service) DeleteLabel(ctx context.Context, assetID, labelID string) (*iapiserver.BatchLabelData, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.DeleteLabel(ctx, owner, assetID, labelID)
	return result, mapAssetWriteError(err)
}
func (s *service) AddTags(ctx context.Context, assetID string, req *iapiserver.AddTagsRequest) (*iapiserver.BatchLabelData, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateLabelsAndTags(nil, req.Tags); err != nil {
		return nil, err
	}
	result, err := s.store.AddTags(ctx, owner, assetID, req.Tags)
	return result, mapAssetWriteError(err)
}
func (s *service) DeleteTag(ctx context.Context, assetID, tagID string) (*iapiserver.BatchLabelData, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.DeleteTag(ctx, owner, assetID, tagID)
	return result, mapAssetWriteError(err)
}

func (s *service) ListArtifacts(ctx context.Context, req *iapiserver.ArtifactListRequest) (*iapiserver.ArtifactListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListArtifacts(ctx, owner, req)
	if err != nil {
		return nil, err
	}
	if err := s.attachArtifactRelations(ctx, owner, items); err != nil {
		return nil, err
	}
	return &iapiserver.ArtifactListResponse{Total: total, Items: items}, nil
}

// BatchArtifactSummaries 保持请求顺序返回 owner 裁剪摘要；缺失、删除和不可见目标统一返回 artifact=null。
func (s *service) BatchArtifactSummaries(ctx context.Context, req *iapiserver.BatchArtifactSummaryRequest) (*iapiserver.BatchArtifactSummaryResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		ids = append(ids, item.ID)
	}
	summaries, err := s.store.ResolveArtifactSummaries(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	items := make([]*iapiserver.BatchArtifactSummaryResult, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, &iapiserver.BatchArtifactSummaryResult{ID: item.ID, Artifact: summaries[item.ID]})
	}
	return &iapiserver.BatchArtifactSummaryResponse{Total: len(items), Items: items}, nil
}
func (s *service) CreateArtifact(ctx context.Context, req *iapiserver.CreateArtifactRequest) (*iapiserver.Artifact, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if containsForbiddenArtifactData(req.Metadata) {
		return nil, errors.NewStatus(code.ErrArtifactSourceForbidden, "artifact metadata contains forbidden source data")
	}
	artifact := &iapiserver.Artifact{OwnerUserID: owner, ProducerType: req.ProducerType, ProducerID: req.ProducerID, ProducerIdempotencyKey: req.ProducerIdempotencyKey, AtomicTaskID: req.AtomicTaskID, TaskAttemptID: req.TaskAttemptID, ApplicationRunID: req.ApplicationRunID, CanvasRunID: req.CanvasRunID, NodeRunID: req.NodeRunID, NodeID: req.NodeID, OutputKey: req.OutputKey, Sequence: req.Sequence, ArtifactType: req.ArtifactType, MediaType: req.MediaType, SavePolicy: req.SavePolicy, ProcessingProfileVersion: req.ProcessingProfileVersion, ExpiresAt: req.ExpiresAt, Metadata: req.Metadata}
	result, _, err := s.store.CreateArtifact(ctx, artifact)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") {
			return nil, errors.NewStatus(code.ErrArtifactIdempotencyConflict, "artifact idempotency conflict")
		}
		return nil, errors.NewStatus(code.ErrArtifactRegistrationInvalid, "artifact request is invalid")
	}
	if err := s.attachArtifactRelations(ctx, owner, []*iapiserver.Artifact{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) GetArtifact(ctx context.Context, id string) (*iapiserver.Artifact, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetArtifact(ctx, owner, id)
	if err = mapNotFound(err, code.ErrArtifactOwnerMismatch); err != nil {
		return nil, err
	}
	if err := s.attachArtifactRelations(ctx, owner, []*iapiserver.Artifact{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) UploadArtifactContent(ctx context.Context, id, mimeType string, reader io.Reader) (*iapiserver.Artifact, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetArtifact(ctx, owner, id); err != nil {
		return nil, mapNotFound(err, code.ErrArtifactOwnerMismatch)
	}
	content, err := s.storage.WriteArtifact(ctx, id, mimeType, reader)
	if err != nil {
		return nil, errors.NewStatus(code.ErrArtifactContentUnavailable, "artifact content write failed")
	}
	result, err := s.store.StoreArtifactContent(ctx, owner, id, content)
	if err != nil {
		return nil, mapArtifactError(err)
	}
	if err := s.attachArtifactRelations(ctx, owner, []*iapiserver.Artifact{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) CompleteArtifact(ctx context.Context, id string, req *iapiserver.CompleteArtifactRequest) (*iapiserver.Artifact, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if containsForbiddenArtifactData(req.Metadata) {
		return nil, errors.NewStatus(code.ErrArtifactSourceForbidden, "artifact metadata contains forbidden source data")
	}
	result, err := s.store.CompleteArtifact(ctx, owner, id, req)
	if err = mapArtifactError(err); err != nil {
		return nil, err
	}
	if err := s.attachArtifactRelations(ctx, owner, []*iapiserver.Artifact{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) DeleteArtifact(ctx context.Context, id string) (*iapiserver.Artifact, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.DeleteArtifact(ctx, owner, id)
	if err = mapArtifactError(err); err != nil {
		return nil, err
	}
	if err := s.attachArtifactRelations(ctx, owner, []*iapiserver.Artifact{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) RegisterArtifact(ctx context.Context, id string, req *iapiserver.RegisterArtifactRequest) (*iapiserver.ArtifactRegistrationResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Mode == "append_version" && req.AssetID == "" {
		return nil, errors.NewStatus(code.ErrArtifactRegistrationInvalid, "append_version requires asset_id")
	}
	result, err := s.store.RegisterArtifactLifecycle(ctx, owner, id, req)
	return result, mapArtifactError(err)
}

func (s *service) ListVersions(ctx context.Context, assetID string) (*iapiserver.AssetVersionListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListAssetVersions(ctx, owner, assetID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
	}
	return &iapiserver.AssetVersionListResponse{Total: int64(len(items)), Items: items}, nil
}
func (s *service) CreateVersion(ctx context.Context, assetID string, req *iapiserver.CreateCanonicalVersionRequest) (*iapiserver.AssetVersion, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.CreateCanonicalVersion(ctx, owner, assetID, req)
	if err != nil && strings.Contains(err.Error(), "canonical") {
		return nil, errors.NewStatus(code.ErrAssetVersionContentInvalid, "asset does not accept canonical versions")
	}
	return result, mapAssetWriteError(err)
}
func (s *service) GetVersion(ctx context.Context, id string) (*iapiserver.AssetVersionDetail, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetAssetVersionDetail(ctx, owner, id)
	return result, mapNotFound(err, code.ErrAssetVersionNotFoundOrNotVisible)
}
func (s *service) SetCurrentVersion(ctx context.Context, assetID, versionID string) (*iapiserver.UserAsset, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.SetCurrentAssetVersion(ctx, owner, assetID, versionID)
	return result, mapNotFound(err, code.ErrAssetVersionNotFoundOrNotVisible)
}
func (s *service) ListRepresentations(ctx context.Context, versionID string) (*iapiserver.AssetRepresentationListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListRepresentations(ctx, owner, versionID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetVersionNotFoundOrNotVisible)
	}
	return &iapiserver.AssetRepresentationListResponse{Total: int64(len(items)), Items: items}, nil
}
func (s *service) RegisterRepresentation(ctx context.Context, versionID string, req *iapiserver.RegisterRepresentationRequest) (*iapiserver.AssetRepresentation, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.RegisterRepresentation(ctx, owner, versionID, req)
	if err != nil && strings.Contains(err.Error(), "conflict") {
		return nil, errors.NewStatus(code.ErrRepresentationWriteConflict, "representation write conflict")
	}
	return result, mapNotFound(err, code.ErrAssetVersionNotFoundOrNotVisible)
}
func (s *service) GetRepresentation(ctx context.Context, id string) (*iapiserver.AssetRepresentation, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, _, err := s.store.GetRepresentation(ctx, owner, id)
	return result, mapNotFound(err, code.ErrAssetRepresentationNotFoundOrNotVisible)
}
func (s *service) ReadRepresentation(ctx context.Context, id string) (*ContentRead, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	representation, content, err := s.store.GetRepresentation(ctx, owner, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAssetRepresentationNotFoundOrNotVisible)
	}
	if content == nil {
		return nil, errors.NewStatus(code.ErrAssetContentUnavailable, "representation has no readable blob")
	}
	reader, err := s.storage.Open(ctx, *content)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAssetContentUnavailable, "representation content is unavailable")
	}
	return &ContentRead{Reader: reader, MIMEType: content.MIMEType, SizeBytes: content.SizeBytes, SHA256: content.SHA256, Filename: representation.Name}, nil
}
func (s *service) RepresentationAccess(ctx context.Context, id, disposition string) (*iapiserver.RepresentationAccess, error) {
	if _, err := s.GetRepresentation(ctx, id); err != nil {
		return nil, err
	}
	expires := time.Now().Add(5 * time.Minute)
	return &iapiserver.RepresentationAccess{RepresentationID: id, AccessURL: "/api/v1/asset-representations/" + id + "/content?disposition=" + disposition, ExpiresAt: imachinery.NewTime(expires)}, nil
}
func (s *service) ListRelations(ctx context.Context, id string, paging imachinery.PagingParams) (*iapiserver.AssetRelationListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListAssetRelations(ctx, owner, id, paging)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
}
func (s *service) Lineage(ctx context.Context, id string) (*iapiserver.AssetLineage, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetAssetLineage(ctx, owner, id)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
}
func (s *service) ListReferences(ctx context.Context, id string, paging imachinery.PagingParams) (*iapiserver.AssetReferenceListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListAssetReferences(ctx, owner, id, paging)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
}
func (s *service) ListUsages(ctx context.Context, id string, paging imachinery.PagingParams) (*iapiserver.AssetUsageListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListAssetUsages(ctx, owner, id, paging)
	return result, mapNotFound(err, code.ErrAssetNotFoundOrNotVisible)
}

func currentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "", errors.NewStatus(code.ErrTokenInvalid, "authenticated user is required")
	}
	return user.ID, nil
}
func validateLabelsAndTags(labels map[string]string, tags []string) error {
	if len(labels) > 20 {
		return errors.NewStatus(code.ErrAssetLabelLimitExceeded, "asset label limit exceeded")
	}
	for key, value := range labels {
		trimmedKey, trimmedValue := strings.TrimSpace(key), strings.TrimSpace(value)
		if trimmedKey != key || trimmedValue != value || len([]rune(key)) < 1 || len([]rune(key)) > 63 || len([]rune(value)) > 63 || strings.ContainsAny(key, ",;()=!\"#@ \t\r\n") {
			return errors.NewStatus(code.ErrAssetLabelInvalid, "asset label is invalid")
		}
	}
	if len(tags) > 30 {
		return errors.NewStatus(code.ErrAssetTagLimitExceeded, "asset tag limit exceeded")
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if strings.TrimSpace(tag) != tag || len([]rune(tag)) < 1 || len([]rune(tag)) > 64 || seen[tag] {
			return errors.NewStatus(code.ErrAssetTagInvalid, "asset tag is invalid")
		}
		seen[tag] = true
	}
	return nil
}
func validateBatchLabels(req *iapiserver.BatchLabelRequest) error {
	if len(req.LabelsToUpsert) == 0 && len(req.TagsToAdd) == 0 && len(req.TagsToRemove) == 0 {
		return errors.NewStatus(code.ErrAssetBatchLabelRequestInvalid, "at least one label change is required")
	}
	if err := validateLabelsAndTags(req.LabelsToUpsert, req.TagsToAdd); err != nil {
		return err
	}
	if err := validateLabelsAndTags(nil, req.TagsToRemove); err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, item := range req.Items {
		if ids[item.ID] {
			return errors.NewStatus(code.ErrAssetBatchLabelRequestInvalid, "asset ids must be unique")
		}
		ids[item.ID] = true
	}
	added := map[string]bool{}
	for _, tag := range req.TagsToAdd {
		added[tag] = true
	}
	for _, tag := range req.TagsToRemove {
		if added[tag] {
			return errors.NewStatus(code.ErrAssetBatchLabelRequestInvalid, "tag cannot be added and removed")
		}
	}
	return nil
}
func validateContentRange(value string, size, total int64, partNumber int, chunkSize int64) error {
	match := regexp.MustCompile(`^bytes ([0-9]+)-([0-9]+)/([0-9]+)$`).FindStringSubmatch(value)
	if match == nil {
		return errors.NewStatus(code.ErrAssetUploadPartInvalid, "content range is invalid")
	}
	start, _ := strconv.ParseInt(match[1], 10, 64)
	end, _ := strconv.ParseInt(match[2], 10, 64)
	declaredTotal, _ := strconv.ParseInt(match[3], 10, 64)
	if end < start || end-start+1 != size || declaredTotal != total {
		return errors.NewStatus(code.ErrAssetUploadPartInvalid, "content range does not match uploaded part")
	}
	if partNumber < 1 || chunkSize < 1 || start != int64(partNumber-1)*chunkSize {
		return errors.NewStatus(code.ErrAssetUploadPartInvalid, "content range does not match part number")
	}
	return nil
}
func containsForbiddenArtifactData(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "credential") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "raw_response") {
				return true
			}
			if containsForbiddenArtifactData(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsForbiddenArtifactData(item) {
				return true
			}
		}
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
	}
	return false
}
func mapNotFound(err error, errorCode int) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(errorCode, "resource does not exist or is not visible")
	}
	return err
}
func mapAssetWriteError(err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(code.ErrAssetNotFoundOrNotWritable, "asset does not exist or is not writable")
	}
	if strings.Contains(err.Error(), "version conflict") {
		return errors.NewStatus(code.ErrAssetResourceVersionConflict, "asset resource version conflict")
	}
	return err
}
func mapUploadError(err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(code.ErrAssetUploadNotFoundOrNotVisible, "upload session does not exist or is not visible")
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "checksum") || strings.Contains(message, "size mismatch"):
		return errors.NewStatus(code.ErrAssetUploadChecksumMismatch, "upload checksum mismatch")
	case strings.Contains(message, "part"):
		return errors.NewStatus(code.ErrAssetUploadPartInvalid, "upload part is invalid")
	case strings.Contains(message, "state") || strings.Contains(message, "cancel"):
		return errors.NewStatus(code.ErrAssetUploadStateInvalid, "upload state does not allow this operation")
	default:
		return err
	}
}
func mapCollectionError(err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(code.ErrCollectionNotFoundOrNotVisible, "collection does not exist or is not visible")
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "name conflict"):
		return errors.NewStatus(code.ErrCollectionNameConflict, "collection name conflicts")
	case strings.Contains(message, "hierarchy") || strings.Contains(message, "depth"):
		return errors.NewStatus(code.ErrCollectionHierarchyInvalid, "collection hierarchy is invalid")
	case strings.Contains(message, "pinned"):
		return errors.NewStatus(code.ErrCollectionPinnedVersionInvalid, "pinned version is invalid")
	case strings.Contains(message, "version conflict"):
		return errors.NewStatus(code.ErrAssetResourceVersionConflict, "collection resource version conflict")
	default:
		return err
	}
}
func mapArtifactError(err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(code.ErrArtifactOwnerMismatch, "artifact does not exist or owner mismatches")
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "conflict"):
		return errors.NewStatus(code.ErrArtifactIdempotencyConflict, "artifact idempotency conflict")
	case strings.Contains(message, "content"):
		return errors.NewStatus(code.ErrArtifactContentUnavailable, "artifact content is unavailable")
	case strings.Contains(message, "state") || strings.Contains(message, "ready"):
		return errors.NewStatus(code.ErrArtifactStateInvalid, "artifact state does not allow this operation")
	default:
		return err
	}
}
func businessError(errorCode int, message string) map[string]any {
	return map[string]any{"code": errorCode, "message": message, "retryable": false}
}

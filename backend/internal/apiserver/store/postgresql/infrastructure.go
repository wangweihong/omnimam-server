package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type infrastructureStore struct{ ds *datastore }

func newInfrastructureStore(ds *datastore) *infrastructureStore { return &infrastructureStore{ds: ds} }

func (s *infrastructureStore) ReconcileInfraCatalog(ctx context.Context, profiles []*iapiserver.InfraRuntimeProfile, node *iapiserver.InfraNode) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, profile := range profiles {
			var existing iapiserver.InfraRuntimeProfile
			err := tx.Where("id = ?", profile.ID).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(profile).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				profile.CreatedAt = existing.CreatedAt
				profile.ResourceVersion = existing.ResourceVersion
				if err := tx.Save(profile).Error; err != nil {
					return err
				}
			}
		}
		var existing iapiserver.InfraNode
		err := tx.Where("id = ?", node.ID).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			return tx.Create(node).Error
		}
		if err != nil {
			return err
		}
		node.CreatedAt = existing.CreatedAt
		node.ResourceVersion = existing.ResourceVersion
		return tx.Save(node).Error
	})
}
func (s *infrastructureStore) ListInfraRuntimeProfiles(ctx context.Context, req *iapiserver.InfraBasicListRequest) ([]*iapiserver.InfraRuntimeProfile, int64, error) {
	var items []*iapiserver.InfraRuntimeProfile
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.InfraRuntimeProfile{}), nil)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *infrastructureStore) GetInfraRuntimeProfile(ctx context.Context, id string) (*iapiserver.InfraRuntimeProfile, error) {
	var item iapiserver.InfraRuntimeProfile
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraRuntimeProfileNotFound, "infra runtime profile not found")
	}
	return &item, nil
}
func (s *infrastructureStore) ListInfraNodes(ctx context.Context, req *iapiserver.InfraBasicListRequest) ([]*iapiserver.InfraNode, int64, error) {
	var items []*iapiserver.InfraNode
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.InfraNode{}), func(q *gorm.DB) *gorm.DB {
		if req.Status != "" {
			q = q.Where("status IN ?", splitCSV(req.Status))
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *infrastructureStore) GetInfraNode(ctx context.Context, id string) (*iapiserver.InfraNode, error) {
	var item iapiserver.InfraNode
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraNoEligibleNode, "infra node not found")
	}
	return &item, nil
}
func (s *infrastructureStore) CreateInfraRuntimeAggregate(ctx context.Context, runtime *iapiserver.InfraRuntime, mounts []*iapiserver.InfraRuntimeMount, bindings []*iapiserver.InfraRuntimeConfigBinding) (*iapiserver.InfraRuntime, error) {
	var result = runtime
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.InfraRuntime
		err := tx.Where("requesting_service = ? AND request_id = ?", runtime.RequestingService, runtime.RequestID).First(&existing).Error
		if err == nil {
			if existing.RequestFingerprint != runtime.RequestFingerprint {
				return errors.NewStatus(code.ErrInfraIdempotencyConflict, "infra request id fingerprint conflicts")
			}
			result = &existing
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(runtime).Error; err != nil {
			return err
		}
		if len(mounts) > 0 {
			if err := tx.Create(&mounts).Error; err != nil {
				return err
			}
		}
		if len(bindings) > 0 {
			if err := tx.Create(&bindings).Error; err != nil {
				return err
			}
		}
		return appendInfraEvent(tx, runtime, nil, "infra_runtime_status_changed")
	})
	return result, err
}
func (s *infrastructureStore) ListInfraRuntimes(ctx context.Context, req *iapiserver.InfraRuntimeListRequest) ([]*iapiserver.InfraRuntime, int64, error) {
	var items []*iapiserver.InfraRuntime
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.InfraRuntime{}), func(q *gorm.DB) *gorm.DB {
		if req.Status != "" {
			q = q.Where("status IN ?", splitCSV(req.Status))
		}
		if req.OwnerDomain != "" {
			q = q.Where("owner_domain = ?", req.OwnerDomain)
		}
		if req.OwnerReference != "" {
			q = q.Where("owner_reference = ?", req.OwnerReference)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}
func (s *infrastructureStore) GetInfraRuntime(ctx context.Context, id string) (*iapiserver.InfraRuntime, error) {
	var item iapiserver.InfraRuntime
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraRuntimeNotFound, "infra runtime not found")
	}
	return &item, nil
}
func (s *infrastructureStore) UpdateInfraRuntime(ctx context.Context, runtime *iapiserver.InfraRuntime, endpoint *iapiserver.InfraRuntimeEndpoint, outputs []*iapiserver.InfraRuntimeOutput, eventType string) (*iapiserver.InfraRuntime, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.InfraRuntime
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.ID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrInfraRuntimeNotFound, "infra runtime not found")
		}
		runtime.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(runtime).Error; err != nil {
			return err
		}
		if endpoint != nil {
			var existing iapiserver.InfraRuntimeEndpoint
			err := tx.Where("runtime_id = ?", runtime.ID).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(endpoint).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				endpoint.ID = existing.ID
				endpoint.CreatedAt = existing.CreatedAt
				endpoint.ResourceVersion = existing.ResourceVersion
				if err := tx.Save(endpoint).Error; err != nil {
					return err
				}
			}
		}
		for _, output := range outputs {
			var existing iapiserver.InfraRuntimeOutput
			err := tx.Where("runtime_id = ? AND output_key = ?", runtime.ID, output.OutputKey).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(output).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				output.ID = existing.ID
				output.CreatedAt = existing.CreatedAt
				output.ResourceVersion = existing.ResourceVersion
				if err := tx.Save(output).Error; err != nil {
					return err
				}
			}
		}
		if eventType == "" {
			eventType = "infra_runtime_status_changed"
		}
		return appendInfraEvent(tx, runtime, &previous, eventType)
	})
	return runtime, err
}
func (s *infrastructureStore) GetInfraRuntimeEndpoint(ctx context.Context, runtimeID string) (*iapiserver.InfraRuntimeEndpoint, error) {
	var item iapiserver.InfraRuntimeEndpoint
	if err := s.ds.db.WithContext(ctx).Where("runtime_id = ?", runtimeID).Order("created_at DESC").First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraEndpointAccessDenied, "infra endpoint not visible")
	}
	return &item, nil
}
func (s *infrastructureStore) GetInfraRuntimeEndpointByID(ctx context.Context, id string) (*iapiserver.InfraRuntimeEndpoint, error) {
	var item iapiserver.InfraRuntimeEndpoint
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraEndpointAccessDenied, "infra endpoint not visible")
	}
	return &item, nil
}
func (s *infrastructureStore) ListInfraRuntimeOutputs(ctx context.Context, runtimeID string) ([]*iapiserver.InfraRuntimeOutput, error) {
	var items []*iapiserver.InfraRuntimeOutput
	err := s.ds.db.WithContext(ctx).Where("runtime_id = ?", runtimeID).Order("output_key ASC").Find(&items).Error
	return items, err
}
func (s *infrastructureStore) GetInfraRuntimeOutput(ctx context.Context, id string) (*iapiserver.InfraRuntimeOutput, error) {
	var item iapiserver.InfraRuntimeOutput
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrInfraOutputContentUnavailable, "infra runtime output content is unavailable")
	}
	return &item, nil
}
func (s *infrastructureStore) AttachInfraRuntimeOutputArtifact(ctx context.Context, id, artifactID string) (*iapiserver.InfraRuntimeOutput, error) {
	var item iapiserver.InfraRuntimeOutput
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&item).Error; err != nil {
			return mapNotFound(err, code.ErrInfraOutputContentUnavailable, "infra runtime output content is unavailable")
		}
		if item.ArtifactID != "" && item.ArtifactID != artifactID {
			return errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "infra runtime output is already attached to another artifact")
		}
		if item.ArtifactID == artifactID {
			return nil
		}
		item.ArtifactID = artifactID
		return tx.Save(&item).Error
	})
	return &item, err
}
func appendInfraEvent(tx *gorm.DB, runtime, previous *iapiserver.InfraRuntime, eventType string) error {
	from := any(nil)
	if previous != nil {
		from = previous.Status
	}
	summary, _ := json.Marshal(map[string]any{"runtime_id": runtime.ID, "owner_domain": runtime.OwnerDomain, "owner_reference": runtime.OwnerReference, "from_status": from, "to_status": runtime.Status, "failure_code": agentNullableString(runtime.FailureCode)})
	event := &iapiserver.InfraRuntimeEvent{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtime.ID, EventType: eventType, ProviderStatus: runtime.Status, SafeSummary: summary, IdempotencyKey: fmt.Sprintf("%s:%d:%s", runtime.ID, runtime.ResourceVersion, eventType)}
	if err := tx.Create(event).Error; err != nil {
		return err
	}
	_ = time.Now()
	return nil
}

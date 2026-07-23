package assetlibrary

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
)

const (
	RepresentationBackfillRef = "asset-library.representation-backfill"
	defaultBackfillActions    = 100
	maxRepresentationAttempts = 3
)

// RepresentationBackfillHandler 按稳定 AssetVersion ID 扫描 expected set，并返回幂等修复动作。
type RepresentationBackfillHandler struct {
	store  store.AssetV1Store
	policy RepresentationPolicy
}

func NewRepresentationBackfillHandler(factory store.Factory, policy RepresentationPolicy) *RepresentationBackfillHandler {
	return &RepresentationBackfillHandler{store: factory.AssetsV1(), policy: policy}
}

func (*RepresentationBackfillHandler) Ref() string         { return RepresentationBackfillRef }
func (*RepresentationBackfillHandler) DisplayName() string { return "素材 Representation 补全" }
func (*RepresentationBackfillHandler) ValidateConfig(config map[string]any) error {
	for key := range config {
		if key != "max_actions_per_run" {
			return fmt.Errorf("representation backfill config contains unsupported field %s", key)
		}
	}
	value := intArgument(config["max_actions_per_run"], defaultBackfillActions)
	if value < 1 || value > 1000 {
		return fmt.Errorf("representation backfill action limit is invalid")
	}
	return nil
}

func (h *RepresentationBackfillHandler) Reconcile(ctx context.Context, req taskcenter.ReconcileRequest) (taskcenter.ReconcileResult, error) {
	cursor, _ := req.Checkpoint["asset_version_id"].(string)
	items, err := h.store.ListRepresentationBackfillCandidatesAfter(ctx, cursor, req.MaxItemsPerRun+1)
	if err != nil {
		return taskcenter.ReconcileResult{}, err
	}
	cycleCompleted := len(items) <= req.MaxItemsPerRun
	if len(items) > req.MaxItemsPerRun {
		items = items[:req.MaxItemsPerRun]
	}
	maxActions := intArgument(req.Config["max_actions_per_run"], defaultBackfillActions)
	result := taskcenter.ReconcileResult{NextCheckpoint: map[string]any{"asset_version_id": cursor}, CycleCompleted: cycleCompleted, Summary: map[string]any{"missing": 0, "irreparable": 0, "repaired": 0}}
	missing, irreparable := 0, 0
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			result.CycleCompleted = false
			return result, err
		}
		result.Scanned++
		result.NextCheckpoint = map[string]any{"asset_version_id": item.AssetVersionID}
		plan := h.policy.Plan(item.MediaType, item.ProfileVersion)
		if len(plan.Requested) == 0 || (item.RepresentationStatus == "ready" && item.RepresentationBlobOK) {
			continue
		}
		result.Findings++
		missing++
		if item.RepresentationStatus == "irreparable" {
			irreparable++
			continue
		}
		if !item.SourceAvailable || item.RetryCount >= maxRepresentationAttempts {
			_, markErr := h.store.CompleteRepresentationGeneration(ctx, item.OwnerUserID, item.AssetVersionID, store.RepresentationGenerationMutation{
				Type: "thumbnail", Profile: thumbnailListProfile, ProfileVersion: item.ProfileVersion, Status: "irreparable", Required: false,
				RetryCount: item.RetryCount, ErrorCode: "representation_source_irrecoverable", ErrorDetail: "thumbnail source cannot be rebuilt automatically",
			})
			if markErr != nil {
				result.CycleCompleted = false
				return result, markErr
			}
			irreparable++
			continue
		}
		if item.RetryAfter != nil && item.RetryAfter.After(time.Now()) {
			result.Deferred++
			continue
		}
		if len(result.Actions) >= maxActions {
			result.Deferred++
			continue
		}
		if err := h.store.PrepareRepresentationBackfill(ctx, item.OwnerUserID, item.AssetVersionID, plan.ExpectedCount); err != nil {
			result.CycleCompleted = false
			return result, err
		}
		result.Actions = append(result.Actions, &iapiserver.AtomicTaskCreateRequest{
			Key: "thumbnail:" + thumbnailListProfile, Name: "Backfill AssetVersion thumbnail",
			SystemName:  iapiserver.SystemNameSpec{Key: taskname.BackfillThumbnail},
			FunctionRef: FunctionRepresentationGenerate, RequiredCapabilities: FunctionRepresentationGenerate,
			Arguments:        map[string]any{"asset_id": item.AssetID, "asset_version_id": item.AssetVersionID, "owner_user_id": item.OwnerUserID, "media_type": item.MediaType, "representation_type": "thumbnail", "profile": thumbnailListProfile, "profile_version": item.ProfileVersion, "required": false, "max_attempts": maxRepresentationAttempts},
			RetryPolicy:      iapiserver.RetryPolicy{MaxAttempts: maxRepresentationAttempts, RetryDelaySeconds: 5, BackoffType: "EXPONENTIAL_BACKOFF", MaxRetryDelaySeconds: 30},
			IdempotencyScope: "asset-representation", IdempotencyKey: "asset-representation:" + item.AssetVersionID + ":thumbnail:" + item.ProfileVersion,
		})
	}
	if cycleCompleted {
		result.NextCheckpoint = map[string]any{"asset_version_id": ""}
	}
	result.Summary["missing"], result.Summary["irreparable"], result.Summary["repaired"] = missing, irreparable, 0
	return result, nil
}

var _ taskcenter.ReconcileHandler = (*RepresentationBackfillHandler)(nil)

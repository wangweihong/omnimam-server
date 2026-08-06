package consumer

import (
	"context"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
)

type representationTaskCreator interface {
	CreateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error)
}

type representationRequestedEvent struct {
	AssetID                  string                    `json:"asset_id"`
	AssetVersionID           string                    `json:"asset_version_id"`
	OwnerUserID              string                    `json:"owner_user_id"`
	ProjectID                string                    `json:"project_id"`
	Namespace                string                    `json:"namespace"`
	MediaType                string                    `json:"media_type"`
	ProfileVersion           string                    `json:"profile_version"`
	RequestedRepresentations []requestedRepresentation `json:"requested_representations"`
	IdempotencyKey           string                    `json:"idempotency_key"`
}

type requestedRepresentation struct {
	RepresentationType string `json:"representation_type"`
	Profile            string `json:"profile"`
	Required           bool   `json:"required"`
}

// StartAssetLibrary 启动 Artifact 投影、处理任务和 representation DAG 编排消费。
func StartAssetLibrary(
	ctx context.Context,
	tasks taskcentersvc.TaskCenterSrv,
	projector *appplatformsvc.ApplicationArtifactProjector,
) error {
	for _, topic := range []string{
		postgresql.OutboxTopicArtifactCreated,
		postgresql.OutboxTopicArtifactProcessingChanged,
		postgresql.OutboxTopicArtifactRegistrationChanged,
	} {
		messages, err := postgresql.SubscribeOutbox(ctx, topic, iapiserver.TaskWorkerConsumerGroupArtifactProjection)
		if err != nil {
			return err
		}
		go func(topic string, messages <-chan *message.Message) {
			for msg := range messages {
				if err := projector.Project(ctx, msg.Payload); err != nil {
					log.Errorf("application artifact projection failed: topic=%s message_id=%s error=%v", topic, msg.UUID, err)
					msg.Nack()
				} else {
					msg.Ack()
				}
			}
		}(topic, messages)
	}
	artifactMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicArtifactContentCompleted, iapiserver.TaskWorkerConsumerGroupArtifactProcess)
	if err != nil {
		return err
	}
	go func() {
		for msg := range artifactMessages {
			var event struct {
				ArtifactID               string `json:"artifact_id"`
				OwnerUserID              string `json:"owner_user_id"`
				ProcessingProfileVersion string `json:"processing_profile_version"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err != nil || event.ArtifactID == "" || event.OwnerUserID == "" {
				msg.Nack()
				continue
			}
			_, err := tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{
				Key: iapiserver.TaskWorkerTaskKeyArtifactProcess, Name: "Process uploaded Artifact", FunctionRef: assetlibrarysvc.FunctionArtifactProcess, SystemName: iapiserver.SystemNameSpec{Key: taskname.ArtifactProcess},
				Arguments:            map[string]any{iapiserver.TaskWorkerKeyArtifactID: event.ArtifactID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID},
				RequiredCapabilities: assetlibrarysvc.FunctionArtifactProcess,
				ProjectID:            iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: event.OwnerUserID,
				IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeArtifactProcess, IdempotencyKey: iapiserver.TaskWorkerIdempotencyScopeArtifactProcess + iapiserver.TaskWorkerCompositeKeySeparator + event.ArtifactID + iapiserver.TaskWorkerCompositeKeySeparator + event.ProcessingProfileVersion,
			})
			if err != nil {
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()

	representationMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetVersionRepresentationRequested, iapiserver.TaskWorkerConsumerGroupRepresentationOrchestrator)
	if err != nil {
		return err
	}
	go func() {
		for msg := range representationMessages {
			if err := HandleRepresentationRequested(ctx, tasks, msg.Payload); err != nil {
				log.Errorf("asset representation orchestration failed: consumer_group=%s message_id=%s error=%v", iapiserver.TaskWorkerConsumerGroupRepresentationOrchestrator, msg.UUID, err)
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()
	return nil
}

// StartThumbnail 启动 Asset 上传事件消费，并为旧版 Asset 创建缩略图任务。
func StartThumbnail(
	ctx context.Context,
	tasks taskcentersvc.TaskCenterSrv,
	thumbnails store.AssetThumbnailStore,
) error {
	messages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetUploaded, iapiserver.TaskWorkerConsumerGroupThumbnail)
	if err != nil {
		return err
	}
	go func() {
		for msg := range messages {
			var event struct {
				AssetID        string `json:"asset_id"`
				AssetVersionID string `json:"asset_version_id"`
				OwnerUserID    string `json:"owner_user_id"`
				ProjectID      string `json:"project_id"`
				Namespace      string `json:"namespace"`
				MediaType      string `json:"media_type"`
				ProfileVersion string `json:"profile_version"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err != nil {
				msg.Nack()
				continue
			}
			if event.AssetVersionID != "" {
				msg.Ack()
				continue
			}
			thumbnail, err := thumbnails.GetByAsset(ctx, event.AssetID)
			if err == nil {
				_, err = tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{Key: iapiserver.TaskWorkerTaskKeyThumbnail, Name: "Generate asset thumbnail", FunctionRef: appplatformsvc.FunctionAssetThumbnailGenerate, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyThumbnailID: thumbnail.ID}, RequiredCapabilities: appplatformsvc.CapabilityAssetThumbnail, ProjectID: event.ProjectID, Namespace: event.Namespace, IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeThumbnail, IdempotencyKey: iapiserver.TaskWorkerIdempotencyPrefixThumbnail + event.AssetID + iapiserver.TaskWorkerCompositeKeySeparator + event.ProfileVersion, SystemName: iapiserver.SystemNameSpec{Key: taskname.AssetThumbnail}})
			}
			if err != nil {
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()
	return nil
}

// HandleRepresentationRequested 将 representation 请求事件创建为 DAG TaskGroup。
func HandleRepresentationRequested(ctx context.Context, tasks representationTaskCreator, payload []byte) error {
	request, err := RepresentationDAGRequest(payload)
	if err != nil {
		return err
	}
	_, err = tasks.CreateDAGTaskGroup(ctx, request)
	return errors.Wrap(err, "create representation DAG task group")
}

// RepresentationDAGRequest 将已校验的 representation 请求事件转换为 DAG 创建请求。
func RepresentationDAGRequest(payload []byte) (*iapiserver.DAGTaskGroupCreateRequest, error) {
	var event representationRequestedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, errors.Wrap(err, "decode representation requested event")
	}
	if event.AssetID == "" || event.AssetVersionID == "" || event.OwnerUserID == "" || event.ProjectID == "" || event.Namespace == "" || event.MediaType == "" || event.ProfileVersion == "" || event.IdempotencyKey == "" || event.RequestedRepresentations == nil {
		return nil, errors.Errorf("representation requested event is incomplete")
	}
	nodes := []iapiserver.DAGNode{
		{Key: iapiserver.TaskWorkerTaskKeyRepresentationInspect, Task: iapiserver.AtomicTaskTemplate{Key: iapiserver.TaskWorkerTaskKeyRepresentationInspect, Name: "Inspect AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationInspect}, FunctionRef: assetlibrarysvc.FunctionRepresentationInspect, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationInspect, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID, iapiserver.TaskWorkerKeyMediaType: event.MediaType, iapiserver.TaskWorkerKeyProfileVersion: event.ProfileVersion}}},
	}
	for _, requested := range event.RequestedRepresentations {
		if requested.RepresentationType == "" || requested.Profile == "" {
			return nil, errors.Errorf("representation requested event contains an invalid representation")
		}
		childKey := requested.RepresentationType + iapiserver.TaskWorkerCompositeKeySeparator + requested.Profile
		nodes = append(nodes, iapiserver.DAGNode{Key: childKey, Task: iapiserver.AtomicTaskTemplate{Key: childKey, Name: "Generate " + requested.RepresentationType, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationGenerate, Params: map[string]string{iapiserver.TaskWorkerKeyRepresentationType: requested.RepresentationType}}, FunctionRef: assetlibrarysvc.FunctionRepresentationGenerate, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationGenerate, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID, iapiserver.TaskWorkerKeyMediaType: event.MediaType, iapiserver.TaskWorkerKeyRepresentationType: requested.RepresentationType, iapiserver.TaskWorkerKeyProfile: requested.Profile, iapiserver.TaskWorkerKeyProfileVersion: event.ProfileVersion, iapiserver.TaskWorkerKeyRequired: requested.Required, iapiserver.TaskWorkerKeyMaxAttempts: 3}, RetryPolicy: iapiserver.RetryPolicy{MaxAttempts: 3, RetryDelaySeconds: 5, BackoffType: iapiserver.TaskWorkerRetryBackoffExponential, MaxRetryDelaySeconds: 30}}})
	}
	nodes = append(nodes, iapiserver.DAGNode{Key: iapiserver.TaskWorkerTaskKeyRepresentationFinalize, Task: iapiserver.AtomicTaskTemplate{Key: iapiserver.TaskWorkerTaskKeyRepresentationFinalize, Name: "Finalize AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationFinalize}, FunctionRef: assetlibrarysvc.FunctionRepresentationFinalize, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationFinalize, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID}}})
	edges := make([]iapiserver.DAGEdge, 0, max(1, 2*len(event.RequestedRepresentations)))
	for _, node := range nodes[1 : len(nodes)-1] {
		edges = append(edges, iapiserver.DAGEdge{FromNode: iapiserver.TaskWorkerTaskKeyRepresentationInspect, ToNode: node.Key}, iapiserver.DAGEdge{FromNode: node.Key, ToNode: iapiserver.TaskWorkerTaskKeyRepresentationFinalize})
	}
	if len(nodes) == 2 {
		edges = append(edges, iapiserver.DAGEdge{FromNode: iapiserver.TaskWorkerTaskKeyRepresentationInspect, ToNode: iapiserver.TaskWorkerTaskKeyRepresentationFinalize})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		Name: "Build AssetVersion representations", Nodes: nodes, Edges: edges, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationBuild},
		Input: map[string]any{iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID}, ProjectID: event.ProjectID,
		Namespace: event.Namespace, CreatedBy: event.OwnerUserID, IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeRepresentations, IdempotencyKey: event.IdempotencyKey,
		TriggerType: iapiserver.DAGTriggerDomainEvent, TriggerSourceID: event.AssetVersionID, TriggerSourceName: iapiserver.TaskWorkerRepresentationTriggerSourceName,
	}, nil
}

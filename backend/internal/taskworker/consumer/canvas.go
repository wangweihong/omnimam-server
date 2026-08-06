package consumer

import (
	"context"
	stderrors "errors"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
)

type reliablePayloadProjector interface {
	Project(context.Context, []byte) error
}

type publishedCanvasApplicationLister interface {
	ListPublishedCanvasApplicationVersions(context.Context) ([]*appsvc.CanvasApplicationVersion, error)
}

// ReconcilePublishedApplicationCatalog 将已发布的应用版本补投影到 Workflow Canvas 目录。
func ReconcilePublishedApplicationCatalog(
	ctx context.Context,
	applications publishedCanvasApplicationLister,
	projector *workflowcanvassvc.ApplicationCatalogProjector,
) error {
	versions, err := applications.ListPublishedCanvasApplicationVersions(ctx)
	if err != nil {
		return err
	}
	for _, item := range versions {
		if item == nil || item.Application == nil || item.Version == nil {
			continue
		}
		err := projector.ProjectPublication(ctx, workflowcanvassvc.ApplicationVersionPublication{
			ApplicationID:                item.Application.ID,
			ApplicationVersionID:         item.Version.ID,
			ApplicationTemplateVersionID: item.Version.ApplicationTemplateVersionID,
			SemanticVersion:              item.Version.SemanticVersion,
			ApplicationName:              item.Application.Name,
			OwnerUserID:                  item.Application.OwnerUserID,
			Visibility:                   item.Application.Visibility,
			CanvasEnabled:                item.Application.CanvasEnabled,
			RunEnabled:                   item.Application.RunEnabled,
			InputSchema:                  item.Version.InputSchema,
			OutputSchema:                 item.Version.OutputSchema,
		})
		var diagnostic *workflowcanvassvc.ApplicationCatalogDiagnosticError
		if stderrors.As(err, &diagnostic) {
			log.Warnf(
				"application version omitted from canvas catalog: application_version_id=%s error=%v",
				item.Version.ID,
				err,
			)
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// StartCanvasApplications 启动应用目录与运行产物引用的可靠 outbox 消费。
func StartCanvasApplications(
	ctx context.Context,
	catalog *workflowcanvassvc.ApplicationCatalogProjector,
	artifacts reliablePayloadProjector,
) error {
	catalogMessages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicApplicationVersionPublished,
		iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
	)
	if err != nil {
		return err
	}
	go ConsumeApplicationCatalog(ctx, catalogMessages, catalog)
	artifactMessages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicApplicationRunArtifactRefChanged,
		iapiserver.TaskWorkerConsumerGroupApplicationArtifactProjection,
	)
	if err != nil {
		return err
	}
	go ConsumeReliablePayloads(
		ctx,
		artifactMessages,
		artifacts,
		iapiserver.TaskWorkerConsumerGroupApplicationArtifactProjection,
	)
	return nil
}

// ConsumeApplicationCatalog 投影应用目录消息，并按可重试性确认或拒绝消息。
func ConsumeApplicationCatalog(
	ctx context.Context,
	messages <-chan *message.Message,
	projector *workflowcanvassvc.ApplicationCatalogProjector,
) {
	for msg := range messages {
		err := projector.Project(ctx, msg.Payload)
		var diagnostic *workflowcanvassvc.ApplicationCatalogDiagnosticError
		if stderrors.As(err, &diagnostic) {
			log.Warnf(
				"application version omitted from canvas catalog: consumer_group=%s message_id=%s error=%v",
				iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
				msg.UUID,
				err,
			)
			msg.Ack()
			continue
		}
		if err != nil {
			log.Errorf(
				"application catalog projection failed: consumer_group=%s message_id=%s error=%v",
				iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// ConsumeReliablePayloads 对通用可靠投影执行成功确认和失败重试。
func ConsumeReliablePayloads(
	ctx context.Context,
	messages <-chan *message.Message,
	projector reliablePayloadProjector,
	consumerGroup string,
) {
	for msg := range messages {
		if err := projector.Project(ctx, msg.Payload); err != nil {
			log.Errorf(
				"reliable projection failed: consumer_group=%s message_id=%s error=%v",
				consumerGroup,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

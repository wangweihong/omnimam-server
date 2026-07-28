package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	OutboxTopicAssetUploaded                       = "asset_uploaded"
	OutboxTopicArtifactRegistered                  = "artifact_registered"
	OutboxTopicEngineHealthChanged                 = "engine_instance_health_changed"
	OutboxTopicScheduleExecutionRecorded           = "task_schedule_execution_recorded"
	OutboxTopicAtomicTaskCreated                   = "atomic_task_created"
	OutboxTopicAtomicTaskStatusChanged             = "atomic_task_status_changed"
	OutboxTopicTaskAttemptStatusChanged            = "task_attempt_status_changed"
	OutboxTopicTaskGroupStatusChanged              = "task_group_status_changed"
	OutboxTopicArtifactCreated                     = "artifact_created"
	OutboxTopicArtifactProcessingChanged           = "artifact_processing_changed"
	OutboxTopicArtifactRegistrationChanged         = "artifact_registration_changed"
	OutboxTopicAssetVersionProcessingChanged       = "asset_version_processing_changed"
	OutboxTopicArtifactContentCompleted            = "artifact_content_completed"
	OutboxTopicAssetVersionRepresentationRequested = "asset_version_representation_requested"
	OutboxTopicApplicationRunArtifactRefChanged    = "application_run_artifact_ref_changed"
	OutboxTopicCanvasVersionPublished              = "canvas_version_published"
	OutboxTopicCanvasRunCreated                    = "canvas_run_created"
	OutboxTopicCanvasRunTaskGroupBound             = "canvas_run_task_group_bound"
	OutboxTopicCanvasRunStatusChanged              = "canvas_run_status_changed"
	OutboxTopicCanvasNodeRunStatusChanged          = "canvas_node_run_status_changed"
	OutboxTopicCanvasNodeOutputAvailable           = "canvas_node_output_available"
	OutboxTopicCanvasRunCancelRequested            = "canvas_run_cancel_requested"
	OutboxTopicCanvasRunRetryCreated               = "canvas_run_retry_created"
)

var outboxSchema = &wmsql.DefaultPostgreSQLSchema{}

func (ds *datastore) ensureOutboxScheme() error {
	for _, topic := range []string{
		OutboxTopicAssetUploaded, OutboxTopicArtifactRegistered, OutboxTopicEngineHealthChanged,
		OutboxTopicScheduleExecutionRecorded, OutboxTopicAtomicTaskCreated, OutboxTopicAtomicTaskStatusChanged,
		OutboxTopicTaskAttemptStatusChanged, OutboxTopicTaskGroupStatusChanged, OutboxTopicArtifactCreated,
		OutboxTopicArtifactProcessingChanged, OutboxTopicArtifactRegistrationChanged, OutboxTopicAssetVersionProcessingChanged,
		OutboxTopicArtifactContentCompleted, OutboxTopicAssetVersionRepresentationRequested,
		OutboxTopicApplicationRunArtifactRefChanged,
		OutboxTopicCanvasVersionPublished, OutboxTopicCanvasRunCreated, OutboxTopicCanvasRunTaskGroupBound,
		OutboxTopicCanvasRunStatusChanged, OutboxTopicCanvasNodeRunStatusChanged, OutboxTopicCanvasNodeOutputAvailable,
		OutboxTopicCanvasRunCancelRequested, OutboxTopicCanvasRunRetryCreated,
	} {
		queries, err := outboxSchema.SchemaInitializingQueries(wmsql.SchemaInitializingQueriesParams{Topic: topic})
		if err != nil {
			return err
		}
		for _, query := range queries {
			if err := ds.db.Exec(query.Query, query.Args...).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

const outboxIdempotencyKeyMetadata = "idempotency_key"

func newOutboxMessage(idempotencyKey string, payload []byte) *message.Message {
	msg := message.NewMessage(uuid.NewString(), payload)
	msg.Metadata.Set(outboxIdempotencyKeyMetadata, idempotencyKey)
	return msg
}

func publishOutbox(tx *gorm.DB, topic, idempotencyKey string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := newOutboxMessage(idempotencyKey, raw)
	msg.SetContext(context.Background())
	query, err := outboxSchema.InsertQuery(wmsql.InsertQueryParams{Topic: topic, Msgs: message.Messages{msg}})
	if err != nil {
		return err
	}
	return tx.Exec(query.Query, query.Args...).Error
}

// SubscribeOutbox opens a durable Watermill PostgreSQL subscription for a worker consumer group.
func SubscribeOutbox(ctx context.Context, topic, consumerGroup string) (<-chan *message.Message, error) {
	factory, ok := postgresqlFactory.(*datastore)
	if !ok || factory == nil {
		return nil, fmt.Errorf("postgresql store is not initialized")
	}
	db, err := factory.db.DB()
	if err != nil {
		return nil, err
	}
	subscriber, err := wmsql.NewSubscriber(
		wmsql.StdSQLBeginner{SQLBeginner: db},
		wmsql.SubscriberConfig{
			ConsumerGroup:    consumerGroup,
			PollInterval:     time.Second,
			ResendInterval:   time.Second,
			SchemaAdapter:    outboxSchema,
			OffsetsAdapter:   wmsql.DefaultPostgreSQLOffsetsAdapter{},
			InitializeSchema: true,
		},
		watermill.NopLogger{},
	)
	if err != nil {
		return nil, err
	}
	messages, err := subscriber.Subscribe(ctx, topic)
	if err != nil {
		return nil, err
	}
	go func() { <-ctx.Done(); _ = subscriber.Close() }()
	return messages, nil
}

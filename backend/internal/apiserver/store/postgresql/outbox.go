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
	OutboxTopicAssetUploaded      = "asset_uploaded"
	OutboxTopicArtifactRegistered = "artifact_registered"
)

var outboxSchema = &wmsql.DefaultPostgreSQLSchema{}

func (ds *datastore) ensureOutboxScheme() error {
	for _, topic := range []string{OutboxTopicAssetUploaded, OutboxTopicArtifactRegistered} {
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
	subscriber, err := wmsql.NewSubscriber(wmsql.StdSQLBeginner{SQLBeginner: db}, wmsql.SubscriberConfig{ConsumerGroup: consumerGroup, PollInterval: time.Second, ResendInterval: time.Second, SchemaAdapter: outboxSchema, OffsetsAdapter: wmsql.DefaultPostgreSQLOffsetsAdapter{}, InitializeSchema: true}, watermill.NopLogger{})
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

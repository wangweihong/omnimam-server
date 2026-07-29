package postgresql

import (
	"errors"
	"fmt"
	"reflect"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const notificationCenterConstraintsSQL = `
CREATE INDEX IF NOT EXISTS idx_notification_topics_activation ON notification_topics(activation_status, enabled, category);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_events_source_topic ON notification_events(source_domain, source_event_id, notification_topic);
CREATE INDEX IF NOT EXISTS idx_notification_events_processing ON notification_events(processing_status, next_attempt_at, created_at);
CREATE INDEX IF NOT EXISTS idx_notification_events_source_aggregate ON notification_events(source_domain, source_aggregate_type, source_aggregate_id, source_aggregate_version);
CREATE INDEX IF NOT EXISTS idx_notification_events_expiry ON notification_events(expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_recipient_dedup ON notifications(recipient_user_id, deduplication_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_aggregate_window ON notifications(recipient_user_id, aggregate_key, aggregation_window, aggregation_bucket) WHERE aggregate_key <> '' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_inbox ON notifications(recipient_user_id, inbox_status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_attention ON notifications(recipient_user_id, attention_status, severity, last_occurred_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_category ON notifications(recipient_user_id, category, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_source ON notifications(source_type, source_id, source_aggregate_version);
CREATE INDEX IF NOT EXISTS idx_notifications_expiry ON notifications(expires_at) WHERE expires_at IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_event_links_pair ON notification_event_links(notification_id, notification_event_id);
CREATE INDEX IF NOT EXISTS idx_notification_event_links_event ON notification_event_links(notification_event_id, notification_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_preferences_scope ON notification_preferences(user_id, category, notification_topic);
CREATE INDEX IF NOT EXISTS idx_notification_preferences_user ON notification_preferences(user_id, category, notification_topic);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_deliveries_destination ON notification_deliveries(notification_id, channel, destination);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_dispatch ON notification_deliveries(channel, delivery_status, next_attempt_at, scheduled_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_outbox_version_event ON notification_outbox(aggregate_type, aggregate_id, aggregate_version, event_name);
CREATE INDEX IF NOT EXISTS idx_notification_outbox_delivery ON notification_outbox(delivery_status, next_attempt_at, created_at);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_notification_events_topic') THEN ALTER TABLE notification_events ADD CONSTRAINT fk_notification_events_topic FOREIGN KEY (notification_topic) REFERENCES notification_topics(topic); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_notifications_topic') THEN ALTER TABLE notifications ADD CONSTRAINT fk_notifications_topic FOREIGN KEY (notification_topic) REFERENCES notification_topics(topic); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_notification_event_links_notification') THEN ALTER TABLE notification_event_links ADD CONSTRAINT fk_notification_event_links_notification FOREIGN KEY (notification_id) REFERENCES notifications(id) ON DELETE CASCADE; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_notification_event_links_event') THEN ALTER TABLE notification_event_links ADD CONSTRAINT fk_notification_event_links_event FOREIGN KEY (notification_event_id) REFERENCES notification_events(id) ON DELETE RESTRICT; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_notification_deliveries_notification') THEN ALTER TABLE notification_deliveries ADD CONSTRAINT fk_notification_deliveries_notification FOREIGN KEY (notification_id) REFERENCES notifications(id) ON DELETE CASCADE; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_topics_values') THEN ALTER TABLE notification_topics ADD CONSTRAINT ck_notification_topics_values CHECK (category IN ('system','task','asset','application','provider','storage','security','agent','canvas') AND activation_status IN ('ACTIVE','CONTRACT_GAP','FUTURE') AND default_severity IN ('info','success','warning','error','critical') AND default_attention_status IN ('informational','action_required','resolved') AND aggregation_mode IN ('immediate','per_source','1_minute','5_minutes','1_hour','daily_digest') AND rule_version >= 1 AND (enabled=FALSE OR activation_status='ACTIVE') AND topic ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$' AND jsonb_typeof(source_mapping_json)='array' AND jsonb_typeof(rule_config_json)='object' AND jsonb_typeof(navigation_config_json)='object'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_events_source_type') THEN ALTER TABLE notification_events ADD CONSTRAINT ck_notification_events_source_type CHECK (resource_version >= 0 AND source_type IN ('atomic_task','task_group','dag_task_group','application_run','artifact','asset','asset_version','application_engine_instance','provider_model','storage_backend','canvas_run','scan_run','asset_batch','user_action','agent_run','system')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notifications_source_type') THEN ALTER TABLE notifications ADD CONSTRAINT ck_notifications_source_type CHECK (resource_version >= 0 AND source_type IN ('atomic_task','task_group','dag_task_group','application_run','artifact','asset','asset_version','application_engine_instance','provider_model','storage_backend','canvas_run','scan_run','asset_batch','user_action','agent_run','system')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_events_values') THEN ALTER TABLE notification_events ADD CONSTRAINT ck_notification_events_values CHECK (resource_version >= 0 AND source_type IN ('atomic_task','task_group','dag_task_group','application_run','artifact','asset','asset_version','application_engine_instance','provider_model','storage_backend','canvas_run','scan_run','asset_batch','user_action','agent_run','system') AND source_aggregate_version >= 0 AND processing_status IN ('PENDING','PROCESSED','IGNORED','FAILED','DEAD_LETTER') AND processing_attempt_count >= 0 AND rule_version >= 1 AND expires_at > occurred_at AND jsonb_typeof(recipient_basis_json)='object' AND jsonb_typeof(payload_snapshot_json)='object'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notifications_values') THEN ALTER TABLE notifications ADD CONSTRAINT ck_notifications_values CHECK (resource_version >= 0 AND category IN ('system','task','asset','application','provider','storage','security','agent','canvas') AND severity IN ('info','success','warning','error','critical') AND inbox_status IN ('unread','read','archived') AND attention_status IN ('informational','action_required','resolved') AND source_type IN ('atomic_task','task_group','dag_task_group','application_run','artifact','asset','asset_version','application_engine_instance','provider_model','storage_backend','canvas_run','scan_run','asset_batch','user_action','agent_run','system') AND aggregation_window IN ('immediate','per_source','1_minute','5_minutes','1_hour','daily_digest') AND source_aggregate_version >= 0 AND occurrence_count >= 1 AND last_occurred_at >= first_occurred_at AND (expires_at IS NULL OR expires_at > first_occurred_at) AND ((inbox_status='unread' AND read_at IS NULL) OR (inbox_status='read' AND read_at IS NOT NULL) OR inbox_status='archived') AND ((inbox_status='archived' AND archived_at IS NOT NULL) OR (inbox_status<>'archived' AND archived_at IS NULL)) AND ((attention_status='resolved' AND resolved_at IS NOT NULL) OR (attention_status<>'resolved' AND resolved_at IS NULL)) AND ((aggregate_key='' AND aggregation_bucket IS NULL) OR (aggregate_key<>'' AND aggregation_bucket IS NOT NULL)) AND (navigation_target_json IS NULL OR jsonb_typeof(navigation_target_json)='object')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_event_links_values') THEN ALTER TABLE notification_event_links ADD CONSTRAINT ck_notification_event_links_values CHECK (source_aggregate_version >= 0 AND occurrence_delta >= 1); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_counters_values') THEN ALTER TABLE notification_recipient_counters ADD CONSTRAINT ck_notification_counters_values CHECK (unread_count >= 0 AND critical_count >= 0 AND action_required_count >= 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_preferences_values') THEN ALTER TABLE notification_preferences ADD CONSTRAINT ck_notification_preferences_values CHECK (category IN ('system','task','asset','application','provider','storage','security','agent','canvas') AND minimum_severity IN ('info','success','warning','error','critical') AND digest_mode IN ('none','daily','weekly') AND (notification_topic='' OR notification_topic ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_deliveries_values') THEN ALTER TABLE notification_deliveries ADD CONSTRAINT ck_notification_deliveries_values CHECK (channel IN ('in_app','email','webhook','mobile_push') AND delivery_status IN ('PENDING','SENDING','SENT','FAILED','DEAD_LETTER','CANCELED') AND attempt_count >= 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_notification_outbox_values') THEN ALTER TABLE notification_outbox ADD CONSTRAINT ck_notification_outbox_values CHECK (event_name IN ('notification_created','notification_updated','notification_deleted','notification_unread_count_changed') AND aggregate_type IN ('notification','notification_recipient_counter') AND aggregate_version >= 0 AND delivery_status IN ('PENDING','PUBLISHED','FAILED') AND attempt_count >= 0 AND jsonb_typeof(payload_json)='object'); END IF;
END $$;
`

type notificationTopicSeed struct {
	topic, category, activation, severity, attention, aggregation string
	recipient, resolver, view                                     string
	enabled, mandatory                                            bool
	sources                                                       []iapiserver.NotificationSourceMapping
}

func (ds *datastore) ensureNotificationCenterScheme() error {
	if err := ds.db.Exec(notificationCenterConstraintsSQL).Error; err != nil {
		return fmt.Errorf("ensure notification center constraints: %w", err)
	}
	return ds.seedNotificationTopics()
}

func (ds *datastore) seedNotificationTopics() error {
	for _, seed := range notificationTopicCatalog() {
		item := &iapiserver.NotificationTopic{
			ObjectMeta:             imachinery.ObjectMeta{ID: "notification-topic:" + seed.topic, Name: seed.topic, Description: "spec-v1.8.0 preset notification topic"},
			Topic:                  seed.topic,
			Category:               seed.category,
			ActivationStatus:       seed.activation,
			Enabled:                seed.enabled,
			MandatoryInApp:         seed.mandatory,
			DefaultSeverity:        seed.severity,
			DefaultAttentionStatus: seed.attention,
			AggregationMode:        seed.aggregation,
			RuleVersion:            1,
			SourceMappings:         seed.sources,
			RuleConfig:             iapiserver.NotificationRuleConfig{Recipient: seed.recipient},
			NavigationConfig:       iapiserver.NotificationNavigationConfig{Resolver: seed.resolver, View: seed.view},
		}
		var current iapiserver.NotificationTopic
		err := ds.db.Where("topic = ?", seed.topic).First(&current).Error
		if err == nil {
			if notificationTopicContentEqual(&current, item) {
				continue
			}
			current.Name = item.Name
			current.Description = item.Description
			current.Category = item.Category
			current.ActivationStatus = item.ActivationStatus
			current.Enabled = item.Enabled
			current.MandatoryInApp = item.MandatoryInApp
			current.DefaultSeverity = item.DefaultSeverity
			current.DefaultAttentionStatus = item.DefaultAttentionStatus
			current.AggregationMode = item.AggregationMode
			current.RuleVersion = item.RuleVersion
			current.SourceMappings = item.SourceMappings
			current.RuleConfig = item.RuleConfig
			current.NavigationConfig = item.NavigationConfig
			if err := ds.db.Save(&current).Error; err != nil {
				return fmt.Errorf("update notification topic %s: %w", seed.topic, err)
			}
			continue
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load notification topic %s: %w", seed.topic, err)
		}
		if err := ds.db.Create(item).Error; err != nil {
			return fmt.Errorf("seed notification topic %s: %w", seed.topic, err)
		}
	}
	return nil
}

func notificationTopicContentEqual(current, expected *iapiserver.NotificationTopic) bool {
	return current.Name == expected.Name &&
		current.Description == expected.Description &&
		current.Category == expected.Category &&
		current.ActivationStatus == expected.ActivationStatus &&
		current.Enabled == expected.Enabled &&
		current.MandatoryInApp == expected.MandatoryInApp &&
		current.DefaultSeverity == expected.DefaultSeverity &&
		current.DefaultAttentionStatus == expected.DefaultAttentionStatus &&
		current.AggregationMode == expected.AggregationMode &&
		current.RuleVersion == expected.RuleVersion &&
		reflect.DeepEqual(current.SourceMappings, expected.SourceMappings) &&
		reflect.DeepEqual(current.RuleConfig, expected.RuleConfig) &&
		reflect.DeepEqual(current.NavigationConfig, expected.NavigationConfig)
}

func notificationTopicCatalog() []notificationTopicSeed {
	source := func(domain, event string) []iapiserver.NotificationSourceMapping {
		return []iapiserver.NotificationSourceMapping{{SourceDomain: domain, SourceEventType: event}}
	}
	active := func(topic, category, severity, attention, recipient, aggregation, sourceDomain, sourceEvent, resolver, view string) notificationTopicSeed {
		return notificationTopicSeed{topic: topic, category: category, activation: iapiserver.NotificationTopicActive, severity: severity, attention: attention, recipient: recipient, aggregation: aggregation, enabled: true, sources: source(sourceDomain, sourceEvent), resolver: resolver, view: view}
	}
	gap := func(topic, category, severity, attention, recipient, aggregation string, sources []iapiserver.NotificationSourceMapping, mandatory bool) notificationTopicSeed {
		return notificationTopicSeed{topic: topic, category: category, activation: iapiserver.NotificationTopicContractGap, severity: severity, attention: attention, recipient: recipient, aggregation: aggregation, sources: sources, mandatory: mandatory}
	}
	future := func(topic, category, severity, attention, recipient, aggregation string, mandatory bool) notificationTopicSeed {
		return notificationTopicSeed{topic: topic, category: category, activation: iapiserver.NotificationTopicFuture, severity: severity, attention: attention, recipient: recipient, aggregation: aggregation, mandatory: mandatory, sources: []iapiserver.NotificationSourceMapping{}}
	}
	return []notificationTopicSeed{
		active("task.atomic_task.succeeded", "task", "success", "informational", "owner_or_initiator", "immediate", "task-center", "atomic_task_status_changed", "atomic_task", "detail"),
		active("task.atomic_task.failed", "task", "error", "action_required", "owner_or_initiator", "per_source", "task-center", "atomic_task_status_changed", "atomic_task", "detail"),
		active("task.atomic_task.timed_out", "task", "error", "action_required", "owner_or_initiator", "per_source", "task-center", "atomic_task_status_changed", "atomic_task", "detail"),
		active("task.atomic_task.action_required", "task", "warning", "action_required", "owner_or_initiator", "per_source", "task-center", "atomic_task_status_changed", "atomic_task", "detail"),
		active("canvas.run.succeeded", "canvas", "success", "informational", "initiator", "immediate", "workflow-canvas", "canvas_run_status_changed", "canvas_run", "detail"),
		active("canvas.run.partially_succeeded", "canvas", "warning", "action_required", "initiator", "per_source", "workflow-canvas", "canvas_run_status_changed", "canvas_run", "detail"),
		active("canvas.run.failed", "canvas", "error", "action_required", "initiator", "per_source", "workflow-canvas", "canvas_run_status_changed", "canvas_run", "detail"),
		gap("task.group.succeeded", "task", "success", "informational", "owner_or_initiator", "per_source", source("task-center", "task_group_status_changed"), false),
		gap("task.group.failed", "task", "error", "action_required", "owner_or_initiator", "per_source", source("task-center", "task_group_status_changed"), false),
		gap("task.group.action_required", "task", "warning", "action_required", "owner_or_initiator", "per_source", source("task-center", "task_group_status_changed"), false),
		gap("asset.representation.completed", "asset", "success", "informational", "owner", "per_source", source("asset-library", "asset_version_processing_changed"), false),
		gap("asset.representation.failed", "asset", "error", "action_required", "owner", "per_source", source("asset-library", "asset_version_processing_changed"), false),
		gap("asset.representation.batch_completed", "asset", "info", "informational", "owner", "5_minutes", source("asset-library", "asset_version_processing_changed"), false),
		gap("asset.representation.batch_failed", "asset", "error", "action_required", "owner", "5_minutes", source("asset-library", "asset_version_processing_changed"), false),
		gap("application.run.completed", "application", "success", "informational", "initiator", "immediate", source("application-platform", "application_run_projection_changed"), false),
		gap("application.run.failed", "application", "error", "action_required", "initiator", "per_source", source("application-platform", "application_run_projection_changed"), false),
		gap("application.artifact.ready", "application", "success", "informational", "initiator", "per_source", []iapiserver.NotificationSourceMapping{{SourceDomain: "asset-library", SourceEventType: "artifact_processing_changed"}, {SourceDomain: "asset-library", SourceEventType: "artifact_registration_changed"}}, false),
		gap("application.engine_instance.unavailable", "provider", "critical", "action_required", "administrators", "per_source", source("application-platform", "engine_instance_health_changed"), true),
		gap("application.engine_instance.recovered", "provider", "info", "resolved", "administrators", "per_source", source("application-platform", "engine_instance_health_changed"), true),
		gap("model.provider_model.unavailable", "provider", "warning", "action_required", "owner", "per_source", source("model-management", "model_health_status_changed"), false),
		gap("model.provider_model.recovered", "provider", "info", "resolved", "owner", "per_source", source("model-management", "model_health_status_changed"), false),
		future("asset.scan.completed", "asset", "info", "informational", "owner", "per_source", false),
		future("asset.scan.partially_failed", "asset", "warning", "action_required", "owner", "per_source", false),
		future("asset.scan.failed", "asset", "error", "action_required", "owner", "per_source", false),
		future("asset.import.completed", "asset", "success", "informational", "initiator", "per_source", false),
		future("asset.import.partially_failed", "asset", "warning", "action_required", "initiator", "per_source", false),
		future("asset.metadata.failed", "asset", "warning", "action_required", "owner", "5_minutes", false),
		future("asset.batch.completed", "asset", "info", "informational", "initiator", "per_source", false),
		future("application.engine_instance.credential_expiring", "provider", "warning", "action_required", "administrators", "per_source", true),
		future("model.provider.credential_expiring", "provider", "warning", "action_required", "owner", "per_source", false),
		future("storage.backend.unavailable", "storage", "critical", "action_required", "administrators", "per_source", true),
		future("storage.capacity.warning", "storage", "warning", "action_required", "administrators", "1_hour", true),
		future("storage.scan.completed", "storage", "info", "informational", "administrators", "per_source", false),
		future("system.update.available", "system", "info", "informational", "system_broadcast", "per_source", false),
		future("system.background_job.failed", "system", "critical", "action_required", "administrators", "1_hour", true),
		future("agent.run.succeeded", "agent", "success", "informational", "initiator", "immediate", false),
		future("agent.run.failed", "agent", "error", "action_required", "initiator", "per_source", false),
		future("agent.approval.requested", "agent", "warning", "action_required", "approver", "immediate", true),
	}
}

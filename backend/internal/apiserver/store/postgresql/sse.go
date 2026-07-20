package postgresql

import (
	"context"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type userEventStore struct{ ds *datastore }

func newUserEventStore(ds *datastore) *userEventStore { return &userEventStore{ds: ds} }

func (s *userEventStore) AddIdempotent(ctx context.Context, data *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error) {
	var existing iapiserver.UserEvent
	err := s.ds.db.WithContext(ctx).Where("recipient_user_id = ? AND source_domain = ? AND source_event_id = ? AND event_type = ?", data.RecipientUserID, data.SourceDomain, data.SourceEventID, data.EventType).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, errors.WithStack(err)
	}
	insert := s.ds.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{
		{Name: "recipient_user_id"}, {Name: "source_domain"}, {Name: "source_event_id"}, {Name: "event_type"},
	}, DoNothing: true}).Create(data)
	if insert.Error != nil {
		return nil, false, errors.WithStack(insert.Error)
	}
	if insert.RowsAffected == 0 {
		if err := s.ds.db.WithContext(ctx).Where("recipient_user_id = ? AND source_domain = ? AND source_event_id = ? AND event_type = ?", data.RecipientUserID, data.SourceDomain, data.SourceEventID, data.EventType).First(&existing).Error; err != nil {
			return nil, false, errors.WithStack(err)
		}
		return &existing, false, nil
	}
	return data, true, nil
}

func (s *userEventStore) List(ctx context.Context, req *iapiserver.UserEventListRequest, now time.Time) ([]*iapiserver.UserEvent, int64, error) {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.UserEvent{}).
		Where("recipient_user_id = ? AND expires_at > ?", req.RecipientUserID, now)
	if req.AfterEventID > 0 {
		query = query.Where("event_sequence > ?", req.AfterEventID)
	}
	if eventTypes := splitCSV(req.EventTypes); len(eventTypes) > 0 {
		query = query.Where("event_type IN ?", eventTypes)
	}
	if req.AggregateType != "" {
		query = query.Where("aggregate_type = ?", req.AggregateType)
	}
	if req.AggregateID != "" {
		query = query.Where("aggregate_id = ?", req.AggregateID)
	}
	if req.Keyword != "" {
		fields := splitCSV(strings.Join(req.SearchFields, ","))
		if len(fields) == 0 {
			fields = []string{"event_type", "aggregate_type", "aggregate_id"}
		}
		parts := make([]string, 0, len(fields))
		args := make([]any, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, field+" ILIKE ?")
			args = append(args, "%"+req.Keyword+"%")
		}
		query = query.Where("("+strings.Join(parts, " OR ")+")", args...)
	}
	if req.CreatedAfterRFC3339 != "" {
		value, _ := time.Parse(time.RFC3339, req.CreatedAfterRFC3339)
		query = query.Where("occurred_at >= ?", value)
	}
	if req.CreatedBeforeRFC3339 != "" {
		value, _ := time.Parse(time.RFC3339, req.CreatedBeforeRFC3339)
		query = query.Where("occurred_at <= ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	orderColumn := "event_sequence"
	if req.SortField == "occurred_at" {
		orderColumn = "occurred_at"
	}
	query = query.Order(orderColumn + " " + strings.ToUpper(req.SortOrder))
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, 0, err
	}
	var items []*iapiserver.UserEvent
	if err := query.Offset(window.Offset).Limit(window.Limit).Find(&items).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *userEventStore) ListAfter(ctx context.Context, recipient string, after int64, limit int, now time.Time) ([]*iapiserver.UserEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var items []*iapiserver.UserEvent
	err := s.ds.db.WithContext(ctx).
		Where("recipient_user_id = ? AND event_sequence > ? AND expires_at > ?", recipient, after, now).
		Order("event_sequence ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *userEventStore) CursorVisible(ctx context.Context, recipient string, eventID int64) (bool, bool, error) {
	var item iapiserver.UserEvent
	err := s.ds.db.WithContext(ctx).
		Select("event_sequence", "expires_at").
		Where("recipient_user_id = ? AND event_sequence = ?", recipient, eventID).
		First(&item).Error
	if err == nil {
		return true, !item.ExpiresAt.After(time.Now()), nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, false, nil
	}
	return false, false, errors.WithStack(err)
}

func (s *userEventStore) SyncState(ctx context.Context, recipient string, now time.Time) (int64, int64, error) {
	var result struct {
		Earliest int64
		Latest   int64
	}
	err := s.ds.db.WithContext(ctx).Model(&iapiserver.UserEvent{}).
		Select("COALESCE(MIN(event_sequence), 0) AS earliest, COALESCE(MAX(event_sequence), 0) AS latest").
		Where("recipient_user_id = ? AND expires_at > ?", recipient, now).
		Scan(&result).Error
	return result.Earliest, result.Latest, errors.WithStack(err)
}

func (s *userEventStore) PruneExpired(ctx context.Context, now time.Time) (int64, error) {
	result := s.ds.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&iapiserver.UserEvent{})
	return result.RowsAffected, errors.WithStack(result.Error)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

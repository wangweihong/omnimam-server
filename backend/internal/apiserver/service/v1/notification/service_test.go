package notification

import (
	"context"
	"errors"
	"testing"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type fakeNotificationStore struct {
	items              map[string]*iapiserver.Notification
	topics             []*iapiserver.NotificationTopic
	preferences        []*iapiserver.NotificationPreference
	listRecipient      string
	mutationRecipients []string
	counter            iapiserver.NotificationRecipientCounter
	mutationError      error
}

func (s *fakeNotificationStore) ListNotifications(_ context.Context, req *iapiserver.NotificationListRequest) ([]*iapiserver.Notification, int64, error) {
	s.listRecipient = req.RecipientUserID
	result := make([]*iapiserver.Notification, 0)
	for _, item := range s.items {
		if item.RecipientUserID == req.RecipientUserID {
			result = append(result, item)
		}
	}
	return result, int64(len(result)), nil
}
func (s *fakeNotificationStore) GetNotificationCounter(_ context.Context, userID string) (*iapiserver.NotificationRecipientCounter, error) {
	value := s.counter
	value.RecipientUserID = userID
	return &value, nil
}
func (s *fakeNotificationStore) MutateNotificationInbox(_ context.Context, userID, id, action string) (*iapiserver.Notification, *iapiserver.NotificationRecipientCounter, error) {
	s.mutationRecipients = append(s.mutationRecipients, userID)
	if s.mutationError != nil {
		return nil, nil, s.mutationError
	}
	item, ok := s.items[id]
	if !ok || item.RecipientUserID != userID {
		return nil, nil, store.ErrNotificationNotVisible
	}
	if item.InboxStatus == iapiserver.NotificationInboxArchived && action == "read" {
		return nil, nil, store.ErrNotificationStateConflict
	}
	item.InboxStatus = iapiserver.NotificationInboxRead
	return item, &s.counter, nil
}
func (s *fakeNotificationStore) ReadAllNotifications(context.Context, string, string) (int64, *iapiserver.NotificationRecipientCounter, error) {
	return 0, &s.counter, nil
}
func (s *fakeNotificationStore) ListNotificationTopics(context.Context, bool) ([]*iapiserver.NotificationTopic, error) {
	return s.topics, nil
}
func (s *fakeNotificationStore) ListNotificationPreferences(context.Context, string) ([]*iapiserver.NotificationPreference, error) {
	return s.preferences, nil
}
func (s *fakeNotificationStore) ReplaceNotificationPreferences(_ context.Context, userID string, items []*iapiserver.NotificationPreference) ([]*iapiserver.NotificationPreference, error) {
	for _, item := range items {
		item.UserID = userID
	}
	s.preferences = items
	return items, nil
}

func notificationUserContext(id string) context.Context {
	user := &iapiserver.User{}
	user.ID = id
	return context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
}

func TestListForcesCurrentRecipient(t *testing.T) {
	fake := &fakeNotificationStore{items: map[string]*iapiserver.Notification{"visible": {RecipientUserID: "user-1"}, "hidden": {RecipientUserID: "user-2"}}}
	result, err := New(fake).List(notificationUserContext("user-1"), &iapiserver.NotificationListRequest{RecipientUserID: "user-2"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.listRecipient != "user-1" || result.Total != 1 {
		t.Fatalf("recipient=%q result=%+v", fake.listRecipient, result)
	}
}

func TestBatchReadKeepsOrderAndPartialFailures(t *testing.T) {
	fake := &fakeNotificationStore{items: map[string]*iapiserver.Notification{"first": {RecipientUserID: "user-1", InboxStatus: "unread"}, "archived": {RecipientUserID: "user-1", InboxStatus: "archived"}}, counter: iapiserver.NotificationRecipientCounter{UnreadCount: 4}}
	result, err := New(fake).BatchRead(notificationUserContext("user-1"), &iapiserver.BatchNotificationRequest{Items: []iapiserver.BatchNotificationItem{{ID: "first"}, {ID: "missing"}, {ID: "archived"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 || result.Fail != 2 || result.UnreadCount != 4 {
		t.Fatalf("result=%+v", result)
	}
	if result.Results[0].ID != "first" || !result.Results[0].Success || result.Results[1].Error.Code != "ERR_NOTIFICATION_NOT_VISIBLE" || result.Results[2].Error.Code != "ERR_NOTIFICATION_STATE_CONFLICT" {
		t.Fatalf("ordered results=%+v", result.Results)
	}
	for _, recipient := range fake.mutationRecipients {
		if recipient != "user-1" {
			t.Fatalf("unexpected recipient %q", recipient)
		}
	}
}

func TestPreferencesRejectUnknownAndMismatchedTopics(t *testing.T) {
	fake := &fakeNotificationStore{topics: []*iapiserver.NotificationTopic{{Topic: "task.atomic_task.failed", Category: "task", Enabled: true, ActivationStatus: "ACTIVE"}}}
	topic := "task.atomic_task.failed"
	enabled, mergeRepeated := true, true
	_, err := New(fake).PutPreferences(notificationUserContext("user-1"), &iapiserver.NotificationPreferenceSetRequest{Items: []iapiserver.NotificationPreferenceItemRequest{{Category: "canvas", NotificationTopic: &topic, InAppEnabled: &enabled, MinimumSeverity: "info", MergeRepeated: &mergeRepeated}}})
	if toolerrors.ToStatus(err).Code != code.ErrNotificationPreferenceInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestPreferencesRejectDisablingMandatoryTopic(t *testing.T) {
	fake := &fakeNotificationStore{topics: []*iapiserver.NotificationTopic{{
		Topic: "system.background_job.failed", Category: "system", Enabled: true,
		ActivationStatus: iapiserver.NotificationTopicActive, MandatoryInApp: true,
	}}}
	topic := "system.background_job.failed"
	enabled, mergeRepeated := false, true
	_, err := New(fake).PutPreferences(notificationUserContext("user-1"), &iapiserver.NotificationPreferenceSetRequest{Items: []iapiserver.NotificationPreferenceItemRequest{{
		Category: "system", NotificationTopic: &topic, InAppEnabled: &enabled,
		MinimumSeverity: iapiserver.NotificationSeverityInfo, MergeRepeated: &mergeRepeated,
	}}})
	if toolerrors.ToStatus(err).Code != code.ErrNotificationMandatoryTopicDisabled {
		t.Fatalf("error=%v", err)
	}
}

func TestBatchReadPropagatesUnexpectedStoreError(t *testing.T) {
	expected := errors.New("database unavailable")
	fake := &fakeNotificationStore{mutationError: expected}
	_, err := New(fake).BatchRead(notificationUserContext("user-1"), &iapiserver.BatchNotificationRequest{
		Items: []iapiserver.BatchNotificationItem{{ID: "first"}},
	})
	if !errors.Is(err, expected) {
		t.Fatalf("error=%v", err)
	}
}

func TestUnauthenticatedAccessIsDenied(t *testing.T) {
	_, err := New(&fakeNotificationStore{}).UnreadCount(context.Background())
	if toolerrors.ToStatus(err).Code != code.ErrNotificationPermissionDenied {
		t.Fatalf("error=%v", err)
	}
}

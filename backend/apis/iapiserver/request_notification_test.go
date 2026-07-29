package iapiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func notificationJSONContext(body string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/notification-preferences", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func TestNotificationPreferenceRequestPresenceAndStrictJSON(t *testing.T) {
	t.Run("explicit false values are preserved", func(t *testing.T) {
		req := &NotificationPreferenceSetRequest{}
		err := req.Decode(notificationJSONContext(`{"items":[{"category":"task","in_app_enabled":false,"minimum_severity":"info","merge_repeated":false}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if err := req.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(req.Items) != 1 || req.Items[0].InAppEnabled == nil || *req.Items[0].InAppEnabled ||
			req.Items[0].MergeRepeated == nil || *req.Items[0].MergeRepeated {
			t.Fatalf("request=%+v", req)
		}
	})
	t.Run("missing items is rejected", func(t *testing.T) {
		req := &NotificationPreferenceSetRequest{}
		if err := req.Decode(notificationJSONContext(`{}`)); err == nil {
			t.Fatal("missing items was accepted")
		}
	})
	t.Run("unknown outer field is rejected", func(t *testing.T) {
		req := &NotificationPreferenceSetRequest{}
		if err := req.Decode(notificationJSONContext(`{"items":[],"email":true}`)); err == nil {
			t.Fatal("unknown outer field was accepted")
		}
	})
	t.Run("unknown item field is rejected", func(t *testing.T) {
		req := &NotificationPreferenceSetRequest{}
		if err := req.Decode(notificationJSONContext(`{"items":[{"category":"task","in_app_enabled":true,"minimum_severity":"info","merge_repeated":true,"digest":"daily"}]}`)); err == nil {
			t.Fatal("unknown item field was accepted")
		}
	})
	t.Run("missing boolean field differs from false", func(t *testing.T) {
		req := &NotificationPreferenceSetRequest{}
		if err := req.Decode(notificationJSONContext(`{"items":[{"category":"task","minimum_severity":"info","merge_repeated":false}]}`)); err != nil {
			t.Fatal(err)
		}
		if err := req.Validate(); err == nil {
			t.Fatal("missing in_app_enabled was accepted")
		}
	})
}

func TestBatchNotificationRequestRejectsUnknownFieldsAndDuplicates(t *testing.T) {
	req := &BatchNotificationRequest{}
	if err := req.Decode(notificationJSONContext(`{"items":[{"id":"one","extra":true}]}`)); err == nil {
		t.Fatal("unknown batch item field was accepted")
	}
	req = &BatchNotificationRequest{}
	if err := req.Decode(notificationJSONContext(`{"items":[{"id":"one"},{"id":"one"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err == nil {
		t.Fatal("duplicate notification ids were accepted")
	}
}

func TestReadAllNotificationRequestAcceptsOptionalEmptyBody(t *testing.T) {
	for _, body := range []string{"", `{}`} {
		t.Run("body="+body, func(t *testing.T) {
			req := &ReadAllNotificationsRequest{}
			if err := req.Decode(notificationJSONContext(body)); err != nil {
				t.Fatal(err)
			}
			if err := req.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}

	req := &ReadAllNotificationsRequest{}
	if err := req.Decode(notificationJSONContext(`{"unknown":true}`)); err == nil {
		t.Fatal("unknown read-all field was accepted")
	}
}

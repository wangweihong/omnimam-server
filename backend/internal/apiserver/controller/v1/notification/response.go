package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/notificationsanitize"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type notificationErrorDefinition struct {
	name      string
	retryable bool
}

var notificationErrors = map[int]notificationErrorDefinition{
	code.ErrNotificationNotVisible:             {name: "ERR_NOTIFICATION_NOT_VISIBLE"},
	code.ErrNotificationStateConflict:          {name: "ERR_NOTIFICATION_STATE_CONFLICT"},
	code.ErrNotificationBatchInvalid:           {name: "ERR_NOTIFICATION_BATCH_INVALID"},
	code.ErrNotificationQueryInvalid:           {name: "ERR_NOTIFICATION_QUERY_INVALID"},
	code.ErrNotificationPreferenceInvalid:      {name: "ERR_NOTIFICATION_PREFERENCE_INVALID"},
	code.ErrNotificationMandatoryTopicDisabled: {name: "ERR_NOTIFICATION_MANDATORY_TOPIC_DISABLED"},
	code.ErrNotificationChannelUnsupported:     {name: "ERR_NOTIFICATION_CHANNEL_UNSUPPORTED"},
	code.ErrNotificationSourceEventInvalid:     {name: "ERR_NOTIFICATION_SOURCE_EVENT_INVALID", retryable: true},
	code.ErrNotificationSourceEventUnsupported: {name: "ERR_NOTIFICATION_SOURCE_EVENT_UNSUPPORTED"},
	code.ErrNotificationRecipientUnresolved:    {name: "ERR_NOTIFICATION_RECIPIENT_UNRESOLVED", retryable: true},
	code.ErrNotificationRuleProcessingFailed:   {name: "ERR_NOTIFICATION_RULE_PROCESSING_FAILED", retryable: true},
	code.ErrNotificationPermissionDenied:       {name: "ERR_NOTIFICATION_PERMISSION_DENIED"},
	code.ErrNotificationAdminScopeRequired:     {name: "ERR_NOTIFICATION_ADMIN_SCOPE_REQUIRED"},
}

type notificationErrorResponse struct {
	Code      string            `json:"code"`
	Value     int               `json:"value"`
	Message   string            `json:"message"`
	Messages  map[string]string `json:"messages"`
	Detail    string            `json:"detail,omitempty"`
	Retryable bool              `json:"retryable"`
	Data      any               `json:"data,omitempty"`
}

func writeResponse(ctx *gin.Context, err error, data any) {
	if err == nil {
		ctx.JSON(http.StatusOK, data)
		return
	}
	status := toolerrors.ToStatus(err)
	definition, ok := notificationErrors[status.Code]
	if !ok {
		core.WriteResponse(ctx, err, data)
		return
	}
	log.Errorf(
		"notification business request rejected: code=%s detail=%s",
		definition.name,
		notificationsanitize.Text(status.Desc, 500),
	)
	ctx.JSON(http.StatusOK, notificationErrorResponse{
		Code: definition.name, Value: status.Code,
		Message: status.Message[toolerrors.MessageLangENKey],
		Messages: map[string]string{
			"zh-CN": status.Message[toolerrors.MessageLangCNKey],
			"en-US": status.Message[toolerrors.MessageLangENKey],
		},
		Detail: status.Desc, Retryable: definition.retryable, Data: data,
	})
}

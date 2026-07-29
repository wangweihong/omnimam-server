package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	notificationsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/notification"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service notificationsvc.Service }

func NewController(service notificationsvc.Service) *Controller { return &Controller{service: service} }

// List 返回当前用户通知列表，只包含持久化摘要和受控导航。
func (c *Controller) List(ctx *gin.Context) {
	req := &iapiserver.NotificationListRequest{}
	if !decode(ctx, req, code.ErrNotificationQueryInvalid) {
		return
	}
	result, err := c.service.List(ctx, req)
	writeResponse(ctx, err, result)
}

// UnreadCount 返回当前用户三个通知计数。
func (c *Controller) UnreadCount(ctx *gin.Context) {
	result, err := c.service.UnreadCount(ctx)
	writeResponse(ctx, err, result)
}

// BatchRead 按请求顺序逐项标记已读，单项失败不回滚成功项。
func (c *Controller) BatchRead(ctx *gin.Context) {
	req := &iapiserver.BatchNotificationRequest{}
	if !decode(ctx, req, code.ErrNotificationBatchInvalid) {
		return
	}
	result, err := c.service.BatchRead(ctx, req)
	writeResponse(ctx, err, result)
}

// ReadAll 将事务开始时锁定的当前范围通知标记已读。
func (c *Controller) ReadAll(ctx *gin.Context) {
	req := &iapiserver.ReadAllNotificationsRequest{}
	if !decode(ctx, req, code.ErrNotificationQueryInvalid) {
		return
	}
	result, err := c.service.ReadAll(ctx, req)
	writeResponse(ctx, err, result)
}

// Read 标记当前用户单条通知已读。
func (c *Controller) Read(ctx *gin.Context) {
	if !validNotificationID(ctx.Param("notification_id")) {
		writeResponse(ctx, errors.NewStatus(code.ErrNotificationNotVisible, "notification is not visible"), nil)
		return
	}
	result, err := c.service.Read(ctx, ctx.Param("notification_id"))
	writeResponse(ctx, err, result)
}

// Unread 标记当前用户单条通知未读。
func (c *Controller) Unread(ctx *gin.Context) {
	if !validNotificationID(ctx.Param("notification_id")) {
		writeResponse(ctx, errors.NewStatus(code.ErrNotificationNotVisible, "notification is not visible"), nil)
		return
	}
	result, err := c.service.Unread(ctx, ctx.Param("notification_id"))
	writeResponse(ctx, err, result)
}

// Archive 归档当前用户单条通知，不改变 attention 状态。
func (c *Controller) Archive(ctx *gin.Context) {
	if !validNotificationID(ctx.Param("notification_id")) {
		writeResponse(ctx, errors.NewStatus(code.ErrNotificationNotVisible, "notification is not visible"), nil)
		return
	}
	result, err := c.service.Archive(ctx, ctx.Param("notification_id"))
	writeResponse(ctx, err, result)
}

// Unarchive 将归档通知固定恢复为已读。
func (c *Controller) Unarchive(ctx *gin.Context) {
	if !validNotificationID(ctx.Param("notification_id")) {
		writeResponse(ctx, errors.NewStatus(code.ErrNotificationNotVisible, "notification is not visible"), nil)
		return
	}
	result, err := c.service.Unarchive(ctx, ctx.Param("notification_id"))
	writeResponse(ctx, err, result)
}

// GetPreferences 返回显式偏好、默认值、mandatory topic 和首期渠道能力。
func (c *Controller) GetPreferences(ctx *gin.Context) {
	result, err := c.service.GetPreferences(ctx)
	writeResponse(ctx, err, result)
}

// PutPreferences 整体替换当前用户站内通知显式偏好。
func (c *Controller) PutPreferences(ctx *gin.Context) {
	req := &iapiserver.NotificationPreferenceSetRequest{}
	if !decode(ctx, req, code.ErrNotificationPreferenceInvalid) {
		return
	}
	result, err := c.service.PutPreferences(ctx, req)
	writeResponse(ctx, err, result)
}

func decode(ctx *gin.Context, req any, errorCode int) bool {
	if err := core.DecodeParameter(ctx, req); err != nil {
		writeResponse(ctx, errors.NewStatus(errorCode, err.Error()), nil)
		return false
	}
	return true
}

func validNotificationID(id string) bool { return id != "" && len(id) <= 128 }

var _ = http.MethodGet

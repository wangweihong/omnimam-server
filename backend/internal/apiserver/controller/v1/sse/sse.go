package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct {
	service           ssesvc.Service
	pollInterval      time.Duration
	heartbeatInterval time.Duration
	connections       *connectionLimiter
}

var (
	draining  = make(chan struct{})
	drainOnce sync.Once
)

// BeginDraining 通知当前实例上的活动 SSE 连接迁移并随后退出。
func BeginDraining() { drainOnce.Do(func() { close(draining) }) }

func NewController(factory store.Factory, config *options.SSEOptions) *Controller {
	if config == nil {
		config = options.NewSSEOptions()
	}
	return &Controller{
		service: ssesvc.New(factory, config.Retention), pollInterval: config.PollInterval,
		heartbeatInterval: config.HeartbeatInterval, connections: newConnectionLimiter(config.MaxConnectionsPerUser),
	}
}

// ListEvents 返回当前认证用户的短期事件历史；payload 不包含其他用户或大型业务正文。
func (c *Controller) ListEvents(ctx *gin.Context) {
	req := &iapiserver.UserEventListRequest{}
	core.Run(ctx, req, func(value *iapiserver.UserEventListRequest) (any, error) {
		return c.service.List(ctx, value)
	})
}

// GetSyncState 返回当前用户可重放事件的最早和最新游标。
func (c *Controller) GetSyncState(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.SyncState(ctx) })
}

// StreamEvents 建立当前认证用户唯一范围的 SSE 流，支持游标重放、心跳与请求取消。
func (c *Controller) StreamEvents(ctx *gin.Context) {
	req := &iapiserver.EventStreamRequest{}
	if err := core.DecodeParameter(ctx, req); err != nil {
		core.WriteResponse(ctx, err, nil)
		return
	}
	headerCursor, err := parseLastEventID(ctx.GetHeader("Last-Event-ID"))
	if err != nil {
		core.WriteResponse(ctx, toolerrors.WrapCode(err, code.ErrValidation), nil)
		return
	}
	if headerCursor > 0 && req.AfterEventID > 0 && headerCursor != req.AfterEventID {
		core.WriteResponse(ctx, toolerrors.NewStatus(code.ErrSSECursorConflict, "Last-Event-ID and after_event_id differ"), nil)
		return
	}
	cursor := req.AfterEventID
	if headerCursor > 0 {
		cursor = headerCursor
	}
	userID, err := c.service.CurrentUserID(ctx)
	if err != nil {
		core.WriteResponse(ctx, err, nil)
		return
	}
	if !c.connections.acquire(userID, req.ClientInstanceID) {
		core.WriteResponse(ctx, toolerrors.NewStatus(code.ErrSSEConnectionLimitReached, "SSE connection limit reached"), nil)
		return
	}
	defer c.connections.release(userID, req.ClientInstanceID)

	cursorState, syncState, err := c.service.ResolveCursor(ctx, cursor)
	if err != nil {
		core.WriteResponse(ctx, toolerrors.WrapCode(err, code.ErrSSEStreamUnavailable), nil)
		return
	}
	prepareSSEHeaders(ctx)
	if cursorState != ssesvc.CursorValid {
		reason := "cursor_not_visible"
		if cursorState == ssesvc.CursorExpired {
			reason = "event_retention_expired"
		}
		if err := writeControlEvent(ctx, "connection.resync_required", map[string]any{
			"reason": reason, "requested_after_event_id": cursor, "earliest_available_event_id": syncState.EarliestAvailableEventID,
		}); err != nil {
			return
		}
		return
	}
	if cursor == 0 {
		cursor = syncState.LatestEventID
	}
	if err := writeControlEvent(ctx, "connection.ready", map[string]any{
		"connection_id": fmt.Sprintf("%d", time.Now().UnixNano()), "server_time": time.Now().UTC().Format(time.RFC3339Nano), "resume_from_event_id": cursor,
	}); err != nil {
		return
	}

	poll := time.NewTicker(c.pollInterval)
	heartbeat := time.NewTicker(c.heartbeatInterval)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-draining:
			if err := writeControlEvent(ctx, "connection.server_draining", map[string]any{"retry_after_seconds": 3}); err != nil {
				return
			}
			return
		case <-poll.C:
			events, listErr := c.service.ListAfter(ctx, cursor, 200)
			if listErr != nil {
				return
			}
			for _, event := range events {
				if err := writeBusinessEvent(ctx, event); err != nil {
					return
				}
				cursor = event.EventSequence
			}
		case <-heartbeat.C:
			if err := writeSSEBytes(ctx, []byte(": heartbeat\n\n")); err != nil {
				return
			}
		}
	}
}

func prepareSSEHeaders(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)
	ctx.Writer.Flush()
}

func writeControlEvent(ctx *gin.Context, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeSSEBytes(ctx, []byte("event: "+eventType+"\ndata: "+string(raw)+"\n\n"))
}

func writeBusinessEvent(ctx *gin.Context, event *iapiserver.UserEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("id: %d\nevent: %s\nretry: 1000\ndata: %s\n\n", event.EventSequence, event.EventType, raw)
	return writeSSEBytes(ctx, []byte(frame))
}

func writeSSEBytes(ctx *gin.Context, data []byte) error {
	controller := http.NewResponseController(ctx.Writer)
	if err := controller.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := ctx.Writer.Write(data); err != nil {
		return err
	}
	ctx.Writer.Flush()
	return nil
}

func parseLastEventID(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("Last-Event-ID must be a positive integer")
	}
	return parsed, nil
}

type connectionLimiter struct {
	mu      sync.Mutex
	counts  map[string]int
	clients map[string]int
	maxUser int
}

func newConnectionLimiter(maxUser int) *connectionLimiter {
	if maxUser <= 0 {
		maxUser = 8
	}
	return &connectionLimiter{counts: make(map[string]int), clients: make(map[string]int), maxUser: maxUser}
}

func (l *connectionLimiter) acquire(userID, clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	clientKey := userID + ":" + clientID
	if l.counts[userID] >= l.maxUser || (clientID != "" && l.clients[clientKey] >= 1) {
		return false
	}
	l.counts[userID]++
	if clientID != "" {
		l.clients[clientKey]++
	}
	return true
}

func (l *connectionLimiter) release(userID, clientID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[userID] <= 1 {
		delete(l.counts, userID)
	} else {
		l.counts[userID]--
	}
	if clientID != "" {
		delete(l.clients, userID+":"+clientID)
	}
}

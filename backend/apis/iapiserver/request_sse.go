package iapiserver

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// UserEventListRequest 查询当前认证用户的短期事件历史；不允许指定其他用户。
type UserEventListRequest struct {
	imachinery.BasicQueryParam
	AfterEventID         int64  `form:"after_event_id" binding:"omitempty,min=1"`
	EventTypes           string `form:"event_types" binding:"omitempty,max=4096"`
	AggregateType        string `form:"aggregate_type" binding:"omitempty,max=128"`
	AggregateID          string `form:"aggregate_id" binding:"omitempty,max=256"`
	CreatedAfterRFC3339  string `form:"-" json:"-"`
	CreatedBeforeRFC3339 string `form:"-" json:"-"`
	RecipientUserID      string `form:"-" json:"-"`
}

// Decode 显式解析 SSE 列表的 RFC3339 时间和逗号分隔查询字段，避免通用 Unix 时间参数改变契约。
func (r *UserEventListRequest) Decode(c *gin.Context) error {
	query := c.Request.URL.Query()
	parseInt := func(name string, target *int64) error {
		if value := query.Get(name); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("%s must be an integer", name)
			}
			*target = parsed
		}
		return nil
	}
	parsePage := func(name string, target *int) error {
		if value := query.Get(name); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s must be an integer", name)
			}
			*target = parsed
		}
		return nil
	}
	if err := parsePage("page_num", &r.PageNum); err != nil {
		return err
	}
	if err := parsePage("page_size", &r.PageSize); err != nil {
		return err
	}
	if err := parseInt("after_event_id", &r.AfterEventID); err != nil {
		return err
	}
	r.Keyword = query.Get("keyword")
	r.SearchFields = splitQueryCSV(query.Get("search_fields"))
	r.SortField = query.Get("sort_field")
	r.SortOrder = strings.ToLower(query.Get("sort_order"))
	r.EventTypes = query.Get("event_types")
	r.AggregateType = query.Get("aggregate_type")
	r.AggregateID = query.Get("aggregate_id")
	r.CreatedAfterRFC3339 = query.Get("created_after")
	r.CreatedBeforeRFC3339 = query.Get("created_before")
	return nil
}

func splitQueryCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (r *UserEventListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 200
	}
	if r.SortField == "" {
		r.SortField = "event_id"
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

func (r *UserEventListRequest) Validate() error {
	if r.PageNum < 0 || r.PageSize < 1 || r.PageSize > 500 {
		return fmt.Errorf("page_num or page_size is invalid")
	}
	if r.SortField != "event_id" && r.SortField != "occurred_at" {
		return fmt.Errorf("sort_field is invalid")
	}
	if r.SortOrder != "asc" && r.SortOrder != "desc" {
		return fmt.Errorf("sort_order is invalid")
	}
	allowedSearch := map[string]struct{}{"event_type": {}, "aggregate_type": {}, "aggregate_id": {}}
	for _, field := range r.SearchFields {
		for _, item := range strings.Split(field, ",") {
			if _, ok := allowedSearch[strings.TrimSpace(item)]; !ok {
				return fmt.Errorf("search_fields contains an unsupported field")
			}
		}
	}
	for _, value := range []string{r.CreatedAfterRFC3339, r.CreatedBeforeRFC3339} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("created time must use RFC3339")
			}
		}
	}
	return nil
}

// UserEventListResponse 返回当前用户可见的短期事件信封。
type UserEventListResponse struct {
	Total int64        `json:"total"`
	Items []*UserEvent `json:"items"`
}

// EventSyncState 描述当前用户事件流的可恢复上下界。
type EventSyncState struct {
	LatestEventID            int64           `json:"latest_event_id"`
	EarliestAvailableEventID int64           `json:"earliest_available_event_id"`
	RetentionSeconds         int64           `json:"retention_seconds"`
	ServerTime               imachinery.Time `json:"server_time"`
}

// EventStreamRequest 控制当前用户 SSE 恢复游标和标签页诊断标识。
type EventStreamRequest struct {
	AfterEventID     int64  `form:"after_event_id" binding:"omitempty,min=1"`
	ClientInstanceID string `form:"client_instance_id" binding:"omitempty,min=1,max=128"`
}

func (r *EventStreamRequest) Validate() error {
	if r.AfterEventID < 0 {
		return fmt.Errorf("after_event_id must be positive")
	}
	if len(r.ClientInstanceID) > 128 {
		return fmt.Errorf("client_instance_id is too long")
	}
	return nil
}

func (r *EventStreamRequest) Decode(c *gin.Context) error {
	if value := c.Query("after_event_id"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			return fmt.Errorf("after_event_id must be a positive integer")
		}
		r.AfterEventID = parsed
	}
	r.ClientInstanceID = c.Query("client_instance_id")
	return nil
}

package iapiserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// NotificationListRequest 查询当前用户通知，recipient 只由认证上下文注入。
// +k8s:deepcopy-gen=true
type NotificationListRequest struct {
	imachinery.BasicQueryParam
	InboxStatuses        string `json:"-" form:"inbox_statuses"`
	Categories           string `json:"-" form:"categories"`
	Severities           string `json:"-" form:"severities"`
	AttentionStatuses    string `json:"-" form:"attention_statuses"`
	NotificationTopics   string `json:"-" form:"notification_topics"`
	CreatedAfterRFC3339  string `json:"-" form:"-"`
	CreatedBeforeRFC3339 string `json:"-" form:"-"`
	RecipientUserID      string `json:"-" form:"-"`
}

// Decode 按 OpenAPI 显式解析 RFC3339 时间和逗号分隔过滤器。
func (r *NotificationListRequest) Decode(c *gin.Context) error {
	query := c.Request.URL.Query()
	if value := query.Get("page_num"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("page_num must be an integer")
		}
		r.PageNum = parsed
	}
	if value := query.Get("page_size"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("page_size must be an integer")
		}
		r.PageSize = parsed
	}
	r.Keyword = query.Get("keyword")
	r.SearchFields = splitQueryCSV(query.Get("search_fields"))
	r.SortField, r.SortOrder = query.Get("sort_field"), strings.ToLower(query.Get("sort_order"))
	r.InboxStatuses, r.Categories, r.Severities = query.Get("inbox_statuses"), query.Get("categories"), query.Get("severities")
	r.AttentionStatuses, r.NotificationTopics = query.Get("attention_statuses"), query.Get("notification_topics")
	r.CreatedAfterRFC3339, r.CreatedBeforeRFC3339 = query.Get("created_after"), query.Get("created_before")
	return nil
}

func (r *NotificationListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 20
	}
	if len(r.SearchFields) == 0 {
		r.SearchFields = []string{"title", "content"}
	}
	if r.SortField == "" {
		r.SortField = "created_at"
	}
	if r.SortOrder == "" {
		r.SortOrder = "desc"
	}
}

func (r *NotificationListRequest) Validate() error {
	if r.PageNum < 0 || r.PageSize < 1 || r.PageSize > 100 || len(r.Keyword) > 200 {
		return fmt.Errorf("notification query pagination or keyword is invalid")
	}
	if !isOneOf(r.SortField, "created_at", "last_occurred_at", "severity") || !isOneOf(r.SortOrder, "asc", "desc") {
		return fmt.Errorf("notification sort is invalid")
	}
	if !allOneOf(r.SearchFields, "title", "content") {
		return fmt.Errorf("notification search_fields is invalid")
	}
	if !csvOneOf(r.InboxStatuses, "unread", "read", "archived") || !csvOneOf(r.Categories, "system", "task", "asset", "application", "provider", "storage", "security", "agent", "canvas") || !csvOneOf(r.Severities, "info", "success", "warning", "error", "critical") || !csvOneOf(r.AttentionStatuses, "informational", "action_required", "resolved") {
		return fmt.Errorf("notification filter is invalid")
	}
	for _, value := range []string{r.CreatedAfterRFC3339, r.CreatedBeforeRFC3339} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("notification created time must use RFC3339")
			}
		}
	}
	return nil
}

// BatchNotificationItem 是保持请求顺序的通知批量操作项。
// +k8s:deepcopy-gen=true
type BatchNotificationItem struct {
	ID string `json:"id" binding:"required,min=1,max=128"`
}

// BatchNotificationRequest 批量标记 1..200 个唯一通知为已读。
// +k8s:deepcopy-gen=true
type BatchNotificationRequest struct {
	Items []BatchNotificationItem `json:"items" binding:"required,min=1,max=200,dive"`
}

// Decode 严格拒绝 OpenAPI 未声明字段，避免客户端误以为未知批量参数已经生效。
func (r *BatchNotificationRequest) Decode(c *gin.Context) error {
	return decodeNotificationJSON(c, r)
}

func (r *BatchNotificationRequest) Validate() error {
	if len(r.Items) < 1 || len(r.Items) > 200 {
		return fmt.Errorf("batch notification request must contain 1 to 200 items")
	}
	seen := make(map[string]struct{}, len(r.Items))
	for _, item := range r.Items {
		if item.ID == "" || len(item.ID) > 128 {
			return fmt.Errorf("notification id length is invalid")
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("notification ids must be unique")
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

// ReadAllNotificationsRequest 可选地将当前用户指定分类的未读通知全部标记已读。
// +k8s:deepcopy-gen=true
type ReadAllNotificationsRequest struct {
	Category string `json:"category,omitempty" binding:"omitempty,oneof=system task asset application provider storage security agent canvas"`
}

// Decode 严格解析可选 read-all 分类范围。
func (r *ReadAllNotificationsRequest) Decode(c *gin.Context) error {
	err := decodeNotificationJSON(c, r)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (r *ReadAllNotificationsRequest) Validate() error {
	if r.Category != "" && !isOneOf(r.Category, "system", "task", "asset", "application", "provider", "storage", "security", "agent", "canvas") {
		return fmt.Errorf("notification category is invalid")
	}
	return nil
}

// NotificationPreferenceItemRequest 是站内渠道显式偏好覆盖。
// +k8s:deepcopy-gen=true
type NotificationPreferenceItemRequest struct {
	Category          string  `json:"category" binding:"required,oneof=system task asset application provider storage security agent canvas"`
	NotificationTopic *string `json:"notification_topic" binding:"omitempty,max=200"`
	InAppEnabled      *bool   `json:"in_app_enabled" binding:"required"`
	MinimumSeverity   string  `json:"minimum_severity" binding:"required,oneof=info success warning error critical"`
	MergeRepeated     *bool   `json:"merge_repeated" binding:"required"`
}

// NotificationPreferenceSetRequest 整体替换当前用户显式偏好；空 items 恢复默认。
// +k8s:deepcopy-gen=true
type NotificationPreferenceSetRequest struct {
	Items        []NotificationPreferenceItemRequest `json:"items" binding:"required,max=100,dive"`
	ItemsPresent bool                                `json:"-"`
}

// Decode 严格拒绝首期未开放的渠道、摘要和静默字段。
func (r *NotificationPreferenceSetRequest) Decode(c *gin.Context) error {
	var wire struct {
		Items json.RawMessage `json:"items"`
	}
	if err := decodeNotificationJSON(c, &wire); err != nil {
		return err
	}
	if wire.Items == nil || string(wire.Items) == "null" {
		return fmt.Errorf("notification preference items are required")
	}
	itemsDecoder := json.NewDecoder(bytes.NewReader(wire.Items))
	itemsDecoder.DisallowUnknownFields()
	if err := itemsDecoder.Decode(&r.Items); err != nil {
		return err
	}
	r.ItemsPresent = true
	return nil
}

func (r *NotificationPreferenceSetRequest) Validate() error {
	if !r.ItemsPresent || r.Items == nil {
		return fmt.Errorf("notification preference items are required")
	}
	if len(r.Items) > 100 {
		return fmt.Errorf("notification preference items exceed 100")
	}
	seen := make(map[string]struct{}, len(r.Items))
	topicPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)
	for _, item := range r.Items {
		if !isOneOf(item.Category, "system", "task", "asset", "application", "provider", "storage", "security", "agent", "canvas") ||
			!isOneOf(item.MinimumSeverity, "info", "success", "warning", "error", "critical") ||
			item.InAppEnabled == nil || item.MergeRepeated == nil {
			return fmt.Errorf("notification preference item is incomplete")
		}
		topic := ""
		if item.NotificationTopic != nil {
			topic = *item.NotificationTopic
			if !topicPattern.MatchString(topic) {
				return fmt.Errorf("notification topic is invalid")
			}
		}
		key := item.Category + "\x00" + topic
		if _, ok := seen[key]; ok {
			return fmt.Errorf("notification preference scopes must be unique")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func decodeNotificationJSON(c *gin.Context, target any) error {
	const maxNotificationRequestBytes = 1 << 20
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxNotificationRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func isOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
func allOneOf(values []string, allowed ...string) bool {
	for _, value := range values {
		if !isOneOf(value, allowed...) {
			return false
		}
	}
	return true
}
func csvOneOf(value string, allowed ...string) bool {
	return allOneOf(splitQueryCSV(value), allowed...)
}

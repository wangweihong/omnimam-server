package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

// NotificationResponse 是当前用户可见的通知历史摘要，不展开源资源当前事实。
// +k8s:deepcopy-gen=true
type NotificationResponse struct {
	ID                string                        `json:"id"`
	Category          string                        `json:"category"`
	NotificationTopic string                        `json:"notification_topic"`
	Severity          string                        `json:"severity"`
	Title             string                        `json:"title"`
	Content           string                        `json:"content"`
	InboxStatus       string                        `json:"inbox_status"`
	AttentionStatus   string                        `json:"attention_status"`
	SourceType        string                        `json:"source_type"`
	SourceID          string                        `json:"source_id"`
	NavigationTarget  *NotificationNavigationTarget `json:"navigation_target"`
	ActionPath        *string                       `json:"action_path"`
	OccurrenceCount   int                           `json:"occurrence_count"`
	FirstOccurredAt   imachinery.Time               `json:"first_occurred_at"`
	LastOccurredAt    imachinery.Time               `json:"last_occurred_at"`
	ReadAt            *imachinery.Time              `json:"read_at"`
	ArchivedAt        *imachinery.Time              `json:"archived_at"`
	ResolvedAt        *imachinery.Time              `json:"resolved_at"`
	ExpiresAt         *imachinery.Time              `json:"expires_at"`
	ResourceVersion   int64                         `json:"resource_version"`
	CreatedAt         imachinery.Time               `json:"created_at"`
	UpdatedAt         imachinery.Time               `json:"updated_at"`
}

// NotificationListResponse 返回当前用户通知总数和当前页。
// +k8s:deepcopy-gen=true
type NotificationListResponse struct {
	Total int64                   `json:"total"`
	Items []*NotificationResponse `json:"items"`
}

// NotificationUnreadCount 返回当前用户三个独立计数投影。
// +k8s:deepcopy-gen=true
type NotificationUnreadCount struct {
	UnreadCount         int             `json:"unread_count"`
	CriticalCount       int             `json:"critical_count"`
	ActionRequiredCount int             `json:"action_required_count"`
	ResourceVersion     int64           `json:"resource_version"`
	UpdatedAt           imachinery.Time `json:"updated_at"`
}

// NotificationAPIError 是批量结果中的标准业务错误摘要。
// +k8s:deepcopy-gen=true
type NotificationAPIError struct {
	Code      string            `json:"code"`
	Value     int               `json:"value"`
	Message   string            `json:"message"`
	Messages  map[string]string `json:"messages"`
	Retryable bool              `json:"retryable"`
}

// BatchNotificationItemResult 返回单项成功数据或业务错误。
// +k8s:deepcopy-gen=true
type BatchNotificationItemResult struct {
	ID      string                `json:"id"`
	Success bool                  `json:"success"`
	Data    *NotificationResponse `json:"data"`
	Error   *NotificationAPIError `json:"error"`
}

// BatchNotificationResponse 保持请求顺序，并附操作后的未读数。
// +k8s:deepcopy-gen=true
type BatchNotificationResponse struct {
	Total       int                           `json:"total"`
	Success     int                           `json:"success"`
	Fail        int                           `json:"fail"`
	Results     []BatchNotificationItemResult `json:"results"`
	UnreadCount int                           `json:"unread_count"`
}

// NotificationBulkActionSummary 返回 read-all 的影响条数与最新计数。
// +k8s:deepcopy-gen=true
type NotificationBulkActionSummary struct {
	AffectedCount       int64           `json:"affected_count"`
	UnreadCount         int             `json:"unread_count"`
	CriticalCount       int             `json:"critical_count"`
	ActionRequiredCount int             `json:"action_required_count"`
	ResourceVersion     int64           `json:"resource_version"`
	UpdatedAt           imachinery.Time `json:"updated_at"`
}

// NotificationPreferenceItem 是当前用户的一条显式偏好。
// +k8s:deepcopy-gen=true
type NotificationPreferenceItem struct {
	ID                string          `json:"id"`
	Category          string          `json:"category"`
	NotificationTopic *string         `json:"notification_topic"`
	InAppEnabled      bool            `json:"in_app_enabled"`
	MinimumSeverity   string          `json:"minimum_severity"`
	MergeRepeated     bool            `json:"merge_repeated"`
	Mandatory         bool            `json:"mandatory"`
	ResourceVersion   int64           `json:"resource_version"`
	CreatedAt         imachinery.Time `json:"created_at"`
	UpdatedAt         imachinery.Time `json:"updated_at"`
}

// NotificationPreferenceDefaults 是未覆盖主题的默认站内策略。
// +k8s:deepcopy-gen=true
type NotificationPreferenceDefaults struct {
	InAppEnabled    bool   `json:"in_app_enabled"`
	MinimumSeverity string `json:"minimum_severity"`
	MergeRepeated   bool   `json:"merge_repeated"`
}

// NotificationChannelCapabilities 明确首期只开放站内投递。
// +k8s:deepcopy-gen=true
type NotificationChannelCapabilities struct {
	InApp      bool `json:"in_app"`
	Email      bool `json:"email"`
	Webhook    bool `json:"webhook"`
	MobilePush bool `json:"mobile_push"`
	Digest     bool `json:"digest"`
	QuietHours bool `json:"quiet_hours"`
}

// NotificationPreferenceSet 返回显式偏好、默认值、mandatory topic 与渠道能力。
// +k8s:deepcopy-gen=true
type NotificationPreferenceSet struct {
	Defaults        NotificationPreferenceDefaults  `json:"defaults"`
	Capabilities    NotificationChannelCapabilities `json:"capabilities"`
	MandatoryTopics []string                        `json:"mandatory_topics"`
	Total           int                             `json:"total"`
	Items           []NotificationPreferenceItem    `json:"items"`
}

package taskcenter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type taskLogCursor struct {
	Version  int    `json:"v"`
	Sort     string `json:"sort"`
	Sequence int    `json:"sequence"`
}

type taskLogFilter struct {
	Keyword string
	Levels  string
	Sources string
	Sort    string
}

// ListAttemptLogs 在父任务授权和 Attempt 归属校验后返回稳定 cursor 分页的运行时日志。
func (s *taskCenterService) ListAttemptLogs(ctx context.Context, req *iapiserver.TaskAttemptLogListRequest) (*iapiserver.TaskAttemptLogListResponse, error) {
	logs, err := s.filteredAttemptLogs(ctx, req.AtomicTaskID, req.TaskAttemptID, taskLogFilter{Keyword: req.Keyword, Levels: req.Levels, Sources: req.Sources, Sort: req.SortOrder})
	if err != nil {
		return nil, err
	}
	start, end, err := taskLogPage(logs, req)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, err.Error())
	}
	items := projectTaskLogs(logs[start:end])
	var previous, next *string
	if start > 0 && len(items) > 0 {
		value := encodeTaskLogCursor(req.SortOrder, items[0].Sequence)
		previous = &value
	}
	if end < len(logs) && len(items) > 0 {
		value := encodeTaskLogCursor(req.SortOrder, items[len(items)-1].Sequence)
		next = &value
	}
	return &iapiserver.TaskAttemptLogListResponse{Total: int64(len(logs)), Items: items, NextCursor: next, PreviousCursor: previous}, nil
}

// DownloadAttemptLogs 生成与在线查询相同过滤、排序和脱敏语义的纯文本附件正文。
func (s *taskCenterService) DownloadAttemptLogs(ctx context.Context, req *iapiserver.TaskAttemptLogDownloadRequest) ([]byte, error) {
	logs, err := s.filteredAttemptLogs(ctx, req.AtomicTaskID, req.TaskAttemptID, taskLogFilter{Keyword: req.Keyword, Levels: req.Levels, Sources: req.Sources, Sort: req.SortOrder})
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	for _, entry := range logs {
		fmt.Fprintf(&builder, "%s\t%s\t%s\t%s\n", entry.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"), entry.Level, entry.Source, entry.Message)
	}
	return []byte(builder.String()), nil
}

func (s *taskCenterService) filteredAttemptLogs(ctx context.Context, atomicTaskID, attemptID string, filter taskLogFilter) ([]workflowruntime.TaskLogEntry, error) {
	if _, err := s.GetAtomicTask(ctx, atomicTaskID); err != nil {
		return nil, err
	}
	attempt, err := s.store.GetAttempt(ctx, atomicTaskID, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.RuntimeTaskID == "" {
		return []workflowruntime.TaskLogEntry{}, nil
	}
	logs, err := s.runtime.ListTaskLogs(ctx, attempt.RuntimeTaskID)
	if err != nil {
		if stderrors.Is(err, workflowruntime.ErrTaskLogNotFound) {
			return nil, errors.NewStatus(code.ErrTaskAttemptLogUnavailable, "task attempt runtime log history is unavailable")
		}
		return nil, taskLogRuntimeError(err)
	}
	levels, err := taskLogValues(filter.Levels, workflowruntime.TaskLogLevelInfo, workflowruntime.TaskLogLevelWarn, workflowruntime.TaskLogLevelError)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, "invalid task log levels")
	}
	sources, err := taskLogValues(filter.Sources, workflowruntime.TaskLogSourceLifecycle, workflowruntime.TaskLogSourceWorker)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, "invalid task log sources")
	}
	keyword := strings.ToLower(filter.Keyword)
	filtered := make([]workflowruntime.TaskLogEntry, 0, len(logs))
	for _, entry := range logs {
		if len(levels) > 0 && !levels[entry.Level] || len(sources) > 0 && !sources[entry.Source] {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(entry.Message), keyword) {
			continue
		}
		filtered = append(filtered, entry)
	}
	if filter.Sort == "desc" {
		slices.Reverse(filtered)
	}
	return filtered, nil
}

func taskLogValues(raw string, allowed ...string) (map[string]bool, error) {
	result := make(map[string]bool)
	if raw == "" {
		return result, nil
	}
	valid := make(map[string]bool, len(allowed))
	for _, value := range allowed {
		valid[value] = true
	}
	for _, value := range strings.Split(raw, ",") {
		value = strings.ToUpper(strings.TrimSpace(value))
		if !valid[value] {
			return nil, fmt.Errorf("unsupported filter value")
		}
		result[value] = true
	}
	return result, nil
}

func taskLogPage(logs []workflowruntime.TaskLogEntry, req *iapiserver.TaskAttemptLogListRequest) (int, int, error) {
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return 0, 0, err
	}
	start := min(window.Offset, len(logs))
	if req.Cursor != "" {
		cursor, err := decodeTaskLogCursor(req.Cursor, req.SortOrder)
		if err != nil {
			return 0, 0, err
		}
		index := slices.IndexFunc(logs, func(entry workflowruntime.TaskLogEntry) bool { return entry.Sequence == cursor.Sequence })
		if index < 0 {
			return 0, 0, fmt.Errorf("task log cursor is no longer available")
		}
		if req.Direction == "backward" {
			end := index
			start = max(0, end-window.Limit)
			return start, end, nil
		}
		start = index + 1
	}
	return start, min(start+window.Limit, len(logs)), nil
}

func projectTaskLogs(logs []workflowruntime.TaskLogEntry) []*iapiserver.TaskAttemptLog {
	items := make([]*iapiserver.TaskAttemptLog, 0, len(logs))
	for _, item := range logs {
		items = append(items, &iapiserver.TaskAttemptLog{Sequence: item.Sequence, Source: item.Source, Level: item.Level, Message: item.Message, OccurredAt: imachinery.NewTime(item.OccurredAt)})
	}
	return items
}

func encodeTaskLogCursor(sortOrder string, sequence int) string {
	payload, _ := json.Marshal(taskLogCursor{Version: 1, Sort: sortOrder, Sequence: sequence})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeTaskLogCursor(value, sortOrder string) (taskLogCursor, error) {
	var cursor taskLogCursor
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(payload, &cursor) != nil || cursor.Version != 1 || cursor.Sort != sortOrder || cursor.Sequence < 1 {
		return taskLogCursor{}, fmt.Errorf("task log cursor is invalid")
	}
	return cursor, nil
}

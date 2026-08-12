package workflowruntime

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

const (
	TaskLogSourceLifecycle = "LIFECYCLE"
	TaskLogSourceWorker    = "WORKER"
	TaskLogLevelInfo       = "INFO"
	TaskLogLevelWarn       = "WARN"
	TaskLogLevelError      = "ERROR"

	maxTaskLogMessageBytes = 4096
	taskLogWriteTimeout    = 2 * time.Second
)

var (
	taskLogAuthorizationPattern = regexp.MustCompile(`(?i)\bauthorization\b["']?\s*[:=]\s*["']?(?:(?:bearer|basic)\s+)?[^\s,;}"']+["']?`)
	taskLogSecretPattern        = regexp.MustCompile(`(?i)\b(api[\s_-]?key|access[\s_-]?key|secret[\s_-]?key|token|secret|password|credential)\b["']?\s*[:=]\s*["']?[^\s,;}"']+["']?`)
	taskLogBearerPattern        = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	taskLogURLPattern           = regexp.MustCompile(`(?i)https?://[^\s]+`)
	taskLogSensitiveRefPattern  = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*(?:access|credential|grant|secret)[a-z0-9+.-]*://[^\s,;}"']+`)
	taskLogEventPattern         = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)

	taskLogEntriesWritten = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "task_log_entries_written_total",
		Help: "Task execution log entries written by bounded source and level.",
	}, []string{"backend", "source", "level"})
	taskLogWriteFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "task_log_write_failures_total",
		Help: "Task execution log write failures by runtime backend.",
	}, []string{"backend"})
	taskLogReadFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "task_log_read_failures_total",
		Help: "Task execution log read failures by runtime backend and bounded reason.",
	}, []string{"backend", "reason"})
	taskLogMetricsOnce sync.Once
)

// TaskLogEntry 是 WorkflowRuntime 与 Task Center 之间的结构化日志投影。
type TaskLogEntry struct {
	Sequence   int
	Source     string
	Level      string
	EventKey   string
	Message    string
	OccurredAt time.Time
}

// TaskLogger 为 Worker handler 提供不改变任务结果的 best-effort 日志能力。
type TaskLogger interface {
	Log(context.Context, TaskLogEntry)
}

type taskLogEnvelope struct {
	Version  int    `json:"v"`
	Source   string `json:"source"`
	Level    string `json:"level"`
	EventKey string `json:"event_key,omitempty"`
	Message  string `json:"message"`
}

type boundTaskLogger struct {
	backend       string
	runtimeTaskID string
	manager       TaskLogManager
}

func registerTaskLogMetrics() {
	taskLogMetricsOnce.Do(func() {
		prometheus.MustRegister(taskLogEntriesWritten, taskLogWriteFailures, taskLogReadFailures)
	})
}

func newBoundTaskLogger(backend, runtimeTaskID string, manager TaskLogManager) TaskLogger {
	return &boundTaskLogger{backend: backend, runtimeTaskID: runtimeTaskID, manager: manager}
}

func (l *boundTaskLogger) Log(ctx context.Context, entry TaskLogEntry) {
	if l == nil || l.manager == nil || l.runtimeTaskID == "" {
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskLogWriteTimeout)
	defer cancel()
	if err := l.manager.AppendTaskLog(writeCtx, l.runtimeTaskID, entry); err != nil {
		log.Warnf("append task execution log failed: backend=%s error=%v", l.backend, err)
	}
}

// LifecycleLog 创建可按 event key 去重的框架生命周期日志。
func LifecycleLog(eventKey, level, message string) TaskLogEntry {
	return TaskLogEntry{Source: TaskLogSourceLifecycle, Level: level, EventKey: eventKey, Message: message}
}

// WorkerLog 创建受控业务执行器进度日志。
func WorkerLog(eventKey, level, message string) TaskLogEntry {
	return TaskLogEntry{Source: TaskLogSourceWorker, Level: level, EventKey: eventKey, Message: message}
}

func failedAttemptLog(err error) TaskLogEntry {
	message := "Execution attempt failed."
	if err != nil {
		message += " " + err.Error()
	}
	return LifecycleLog("attempt.failed", TaskLogLevelError, message)
}

func encodeTaskLog(entry TaskLogEntry) (string, TaskLogEntry, error) {
	entry = normalizeTaskLogEntry(entry)
	payload, err := json.Marshal(taskLogEnvelope{Version: 1, Source: entry.Source, Level: entry.Level, EventKey: entry.EventKey, Message: entry.Message})
	return string(payload), entry, err
}

func decodeTaskLog(raw string, occurredAt time.Time) TaskLogEntry {
	entry := TaskLogEntry{Source: TaskLogSourceWorker, Level: TaskLogLevelInfo, Message: raw, OccurredAt: occurredAt}
	var envelope taskLogEnvelope
	if json.Unmarshal([]byte(raw), &envelope) == nil && envelope.Version == 1 && envelope.Message != "" {
		entry.Source, entry.Level, entry.EventKey, entry.Message = envelope.Source, envelope.Level, envelope.EventKey, envelope.Message
	}
	return normalizeTaskLogEntry(entry)
}

func normalizeTaskLogs(entries []TaskLogEntry) []TaskLogEntry {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].OccurredAt.Before(entries[j].OccurredAt) })
	seenEvents := make(map[string]struct{})
	result := make([]TaskLogEntry, 0, len(entries))
	for _, entry := range entries {
		entry = normalizeTaskLogEntry(entry)
		if entry.EventKey != "" {
			dedupeKey := entry.Source + ":" + entry.EventKey
			if _, exists := seenEvents[dedupeKey]; exists {
				continue
			}
			seenEvents[dedupeKey] = struct{}{}
		}
		entry.Sequence = len(result) + 1
		result = append(result, entry)
	}
	return result
}

func normalizeTaskLogEntry(entry TaskLogEntry) TaskLogEntry {
	if entry.Source != TaskLogSourceLifecycle && entry.Source != TaskLogSourceWorker {
		entry.Source = TaskLogSourceWorker
	}
	switch entry.Level {
	case TaskLogLevelInfo, TaskLogLevelWarn, TaskLogLevelError:
	default:
		entry.Level = TaskLogLevelInfo
	}
	if !taskLogEventPattern.MatchString(entry.EventKey) {
		entry.EventKey = ""
	}
	entry.Message = sanitizeTaskLogMessage(entry.Message)
	return entry
}

func sanitizeTaskLogMessage(message string) string {
	message = strings.ToValidUTF8(message, "\uFFFD")
	message = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(message)
	message = taskLogAuthorizationPattern.ReplaceAllString(message, "Authorization=[REDACTED]")
	message = taskLogBearerPattern.ReplaceAllString(message, "Bearer [REDACTED]")
	message = taskLogSecretPattern.ReplaceAllStringFunc(message, func(match string) string {
		if index := strings.IndexAny(match, ":="); index >= 0 {
			return strings.TrimSpace(match[:index]) + "=[REDACTED]"
		}
		return "[REDACTED]"
	})
	message = taskLogURLPattern.ReplaceAllString(message, "[REDACTED-URL]")
	message = taskLogSensitiveRefPattern.ReplaceAllString(message, "[REDACTED-REFERENCE]")
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= maxTaskLogMessageBytes {
		return message
	}
	end := maxTaskLogMessageBytes
	for end > 0 && !utf8.ValidString(message[:end]) {
		end--
	}
	return message[:end]
}

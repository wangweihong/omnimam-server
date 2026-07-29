// Package notificationsanitize 清理通知候选、持久化摘要和错误日志中的敏感内容。
package notificationsanitize

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	authorizationPattern = regexp.MustCompile(`(?i)\bauthorization\b\s*["']?\s*[:=]?\s*["']?\s*(?:bearer\s+|basic\s+)?[^\s,;"']+`)
	secretPattern        = regexp.MustCompile(`(?i)\b(bearer|access[_-]?token|refresh[_-]?token|token|api[\s_-]?key|secret|signature)\b\s*["']?\s*[:=]?\s*["']?\s*[^\s,;"']+`)
	urlPattern           = regexp.MustCompile(`(?i)\bhttps?://[^\s]+`)
	privateIP            = regexp.MustCompile(`(?i)\b(localhost|127(?:\.\d{1,3}){3}|10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2}|::1|f[cd][0-9a-f:]+|fe80:[0-9a-f:]+)\b`)
	stackLine            = regexp.MustCompile(`(?i)(^\s*(goroutine\s+\d+|panic:|runtime\.|at\s+)|\.go:\d+|stack trace|traceback)`)
)

// Text 删除凭证、完整 URL、私网地址和内部栈，并按 Unicode 字符安全截断。
func Text(value string, limit int) string {
	lines := strings.Split(value, "\n")
	safeLines := lines[:0]
	for _, line := range lines {
		if !stackLine.MatchString(line) {
			safeLines = append(safeLines, line)
		}
	}
	value = strings.Join(safeLines, " ")
	value = authorizationPattern.ReplaceAllString(value, "Authorization=[REDACTED]")
	value = secretPattern.ReplaceAllString(value, "$1=[REDACTED]")
	value = urlPattern.ReplaceAllString(value, "[URL_REMOVED]")
	value = privateIP.ReplaceAllString(value, "[PRIVATE_ADDRESS_REMOVED]")
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

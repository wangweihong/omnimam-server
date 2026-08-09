// Package invocationsse implements the wire-level contract shared by Platform
// Agent and AppStudio Coding Agent Invocation event streams.
package invocationsse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

var errInvalidLastEventID = errors.New("last-event-id must be a canonical non-negative decimal integer")

// ParseLastEventID accepts only the canonical decimal syntax released for the
// Invocation SSE resume cursor. An absent header starts at sequence zero.
func ParseLastEventID(value string) (int, error) {
	if value == "" || value == "0" {
		return 0, nil
	}
	if value[0] < '1' || value[0] > '9' {
		return 0, errInvalidLastEventID
	}
	for index := 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return 0, errInvalidLastEventID
		}
	}
	parsed, err := strconv.ParseUint(value, 10, strconv.IntSize)
	if err != nil {
		return 0, errInvalidLastEventID
	}
	return int(parsed), nil
}

// WriteEvent writes one persisted event as the exact SSE id/event/data triple.
func WriteEvent(writer io.Writer, event *iapiserver.AgentOperationEvent) error {
	if event == nil || event.InvocationID == "" || event.SequenceNo < 1 || !isEventType(event.EventType) {
		return fmt.Errorf("invalid agent invocation event")
	}
	if strings.ContainsAny(event.EventType, "\r\n") {
		return fmt.Errorf("invalid agent invocation event type")
	}
	payload := []byte(event.Payload)
	if len(payload) == 0 || !json.Valid(payload) || payload[0] != '{' {
		return fmt.Errorf("invalid agent invocation event payload")
	}
	envelope, err := json.Marshal(&iapiserver.AgentInvocationEventEnvelope{
		InvocationID: event.InvocationID,
		SequenceNo:   int64(event.SequenceNo),
		OccurredAt:   event.CreatedAt,
		Event:        event.EventType,
		Payload:      event.Payload,
	})
	if err != nil {
		return fmt.Errorf("marshal agent invocation event envelope: %w", err)
	}
	if _, err := fmt.Fprintf(writer, "id: %d\nevent: %s\ndata: %s\n\n", event.SequenceNo, event.EventType, envelope); err != nil {
		return fmt.Errorf("write agent invocation event: %w", err)
	}
	return nil
}

// IsTerminalEventType reports whether an event is the unique stream-closing event.
func IsTerminalEventType(eventType string) bool {
	return eventType == iapiserver.AgentOperationEventTypeInvocationCompleted ||
		eventType == iapiserver.AgentOperationEventTypeInvocationFailed ||
		eventType == iapiserver.AgentOperationEventTypeInvocationCanceled
}

// IsTerminalInvocationStatus reports whether no more business events may be appended.
func IsTerminalInvocationStatus(status string) bool {
	return status == iapiserver.AgentInvocationStatusSucceeded ||
		status == iapiserver.AgentInvocationStatusFailed ||
		status == iapiserver.AgentInvocationStatusCanceled
}

func isEventType(eventType string) bool {
	switch eventType {
	case iapiserver.AgentOperationEventTypeInvocationStarted,
		iapiserver.AgentOperationEventTypeMessageDelta,
		iapiserver.AgentOperationEventTypeMessageCompleted,
		iapiserver.AgentOperationEventTypeToolRequested,
		iapiserver.AgentOperationEventTypeToolStarted,
		iapiserver.AgentOperationEventTypeToolProgress,
		iapiserver.AgentOperationEventTypeToolCompleted,
		iapiserver.AgentOperationEventTypeToolFailed,
		iapiserver.AgentOperationEventTypeUserInputRequired,
		iapiserver.AgentOperationEventTypeInvocationCompleted,
		iapiserver.AgentOperationEventTypeInvocationFailed,
		iapiserver.AgentOperationEventTypeInvocationCanceled:
		return true
	default:
		return false
	}
}

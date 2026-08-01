package mcp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type cursorPayload struct {
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Scope  string `json:"scope"`
}

func decodeCursor(raw string, limit int, scope string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, err
	}
	var payload cursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return 0, err
	}
	if payload.Offset < 0 || payload.Limit != limit || payload.Scope != cursorScope(scope) || payload.Offset%limit != 0 {
		return 0, fmt.Errorf("cursor does not match the current query")
	}
	return payload.Offset / limit, nil
}

func nextCursor(offset, returned, total, limit int, scope string) string {
	next := offset + returned
	if returned == 0 || next >= total {
		return ""
	}
	raw, _ := json.Marshal(cursorPayload{Offset: next, Limit: limit, Scope: cursorScope(scope)})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func cursorScope(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}

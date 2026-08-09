package agentmcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

var sensitiveConfigurationKeyParts = [...]string{
	"apikey",
	"auth",
	"command",
	"cookie",
	"credential",
	"environment",
	"header",
	"password",
	"privatekey",
	"secret",
	"token",
	"url",
}

var reservedConfigurationKeys = map[string]struct{}{
	"cmd":     {},
	"enabled": {},
	"env":     {},
	"type":    {},
}

// ParseConfiguration decodes an MCP Binding's non-sensitive JSON object and
// rejects fields that could carry credentials or override resolver-owned data.
func ParseConfiguration(raw json.RawMessage) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var configuration map[string]any
	if err := decoder.Decode(&configuration); err != nil || configuration == nil {
		return nil, fmt.Errorf("MCP configuration must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("MCP configuration must contain one JSON object")
	}
	if err := ValidateConfiguration(configuration); err != nil {
		return nil, err
	}
	return configuration, nil
}

// ValidateConfiguration recursively rejects secret-bearing and resolver-owned
// fields without including attacker-controlled keys or values in the error.
func ValidateConfiguration(configuration map[string]any) error {
	if configurationContainsReservedField(configuration) {
		return fmt.Errorf("MCP configuration contains sensitive or reserved fields")
	}
	return nil
}

func configurationContainsReservedField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := normalizeConfigurationKey(key)
			if _, reserved := reservedConfigurationKeys[normalized]; reserved {
				return true
			}
			for _, part := range sensitiveConfigurationKeyParts {
				if strings.Contains(normalized, part) {
					return true
				}
			}
			if configurationContainsReservedField(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if configurationContainsReservedField(nested) {
				return true
			}
		}
	}
	return false
}

func normalizeConfigurationKey(key string) string {
	var normalized strings.Builder
	normalized.Grow(len(key))
	for _, char := range strings.ToLower(key) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

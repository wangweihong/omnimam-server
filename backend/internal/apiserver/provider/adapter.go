package provider

import "context"

type Capability string

type InvokeRequest struct {
	Capability Capability     `json:"capability"`
	Model      string         `json:"model"`
	Messages   []ChatMessage  `json:"messages,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InvokeResult struct {
	Content string         `json:"content,omitempty"`
	Raw     map[string]any `json:"raw,omitempty"`
}

type Adapter interface {
	ID() string
	Capabilities() []Capability
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
}

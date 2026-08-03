package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(baseURL, token string) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("infrastructure base url is required")
	}
	if len(token) < 32 {
		return nil, fmt.Errorf("infrastructure service token must be at least 32 bytes")
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: 10 * time.Minute}}, nil
}
func (c *Client) Execute(ctx context.Context, request *CommandRequest) (*CommandResponse, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/execute", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result CommandResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK || result.Error != "" {
		return nil, fmt.Errorf("infrastructure command failed: %s", result.Error)
	}
	return &result, nil
}

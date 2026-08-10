package infrastructureclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// Client is the API-server side read-only Infrastructure diagnostics client.
type Client struct {
	baseURL, token string
	http           *http.Client
}

const (
	infrastructureLogPageSize   = 200
	infrastructureLogMaxEntries = 5000
)

func New(baseURL, token string) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("infrastructure base url is required")
	}
	if len(token) < 32 {
		return nil, fmt.Errorf("infrastructure service token must be at least 32 bytes")
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) Logs(ctx context.Context, runtimeID, ownerReference string, limit int) ([]*iapiserver.InfraRuntimeLogEntry, error) {
	if limit <= 0 || limit > infrastructureLogMaxEntries {
		limit = infrastructureLogMaxEntries
	}
	entries := make([]*iapiserver.InfraRuntimeLogEntry, 0, limit)
	for pageNum := 0; len(entries) < limit; pageNum++ {
		path := "/api/v1/infra/runtimes/" + url.PathEscape(runtimeID) + "/logs?owner_reference=" + url.QueryEscape(ownerReference) + "&page_num=" + strconv.Itoa(pageNum) + "&page_size=" + strconv.Itoa(infrastructureLogPageSize)
		request, err := c.request(ctx, path)
		if err != nil {
			return nil, err
		}
		response, err := c.http.Do(request)
		if err != nil {
			return nil, err
		}
		var result iapiserver.InfraRuntimeLogListResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("infrastructure logs returned status %d", response.StatusCode)
		}
		entries = append(entries, result.Items...)
		if len(result.Items) < infrastructureLogPageSize || len(entries) >= int(result.Total) {
			break
		}
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (c *Client) Health(ctx context.Context, runtimeID, ownerReference string) (*iapiserver.InfraRuntimeHealthResult, error) {
	request, err := c.request(ctx, "/api/v1/infra/runtimes/"+url.PathEscape(runtimeID)+"/health?owner_reference="+url.QueryEscape(ownerReference))
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result iapiserver.InfraRuntimeHealthResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("infrastructure health returned status %d", response.StatusCode)
	}
	return &result, nil
}

func (c *Client) request(ctx context.Context, path string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	return request, nil
}

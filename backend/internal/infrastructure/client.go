package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
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
	transportRequest := *request
	if request.Create != nil {
		transportRequest.CreateContext = &CommandCreateContext{
			AuthorizationRef:   request.Create.AuthorizationRef,
			EndpointVisibility: request.Create.EndpointVisibility,
			FunctionRef:        request.Create.FunctionRef,
			FunctionArguments:  request.Create.FunctionArguments,
		}
	}
	raw, err := json.Marshal(&transportRequest)
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

// OutputContent 是 Infrastructure 已认证输出流及其可信 descriptor。
type OutputContent struct {
	Body          io.ReadCloser
	MediaType     string
	SizeBytes     int64
	ContentDigest string
}

// ReadOutputContent 按稳定 output ID 打开已收集内容；调用方必须关闭 Body。
func (c *Client) ReadOutputContent(ctx context.Context, outputID string) (*OutputContent, error) {
	request, err := c.request(ctx, http.MethodGet, "/api/v1/infra/runtime-outputs/"+url.PathEscape(outputID)+"/content", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Content-Digest") == "" {
		defer response.Body.Close()
		return nil, decodeInfraError(response)
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil || size < 0 {
		response.Body.Close()
		return nil, fmt.Errorf("infrastructure output content length is invalid")
	}
	digest := response.Header.Get("X-Content-Digest")
	if !validSHA256(digest) {
		response.Body.Close()
		return nil, fmt.Errorf("infrastructure output content digest is invalid")
	}
	mediaType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if mediaType == "" {
		response.Body.Close()
		return nil, fmt.Errorf("infrastructure output content type is invalid")
	}
	return &OutputContent{Body: response.Body, MediaType: mediaType, SizeBytes: size, ContentDigest: digest}, nil
}

// AttachOutputArtifact 幂等回链已完成且摘要一致的 Asset Library Artifact。
func (c *Client) AttachOutputArtifact(ctx context.Context, outputID string, input *iapiserver.InfraAttachArtifactRequest) (*iapiserver.InfraRuntimeOutput, error) {
	request, err := c.request(ctx, http.MethodPost, "/api/v1/infra/runtime-outputs/"+url.PathEscape(outputID)+"/attach-artifact", input)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var output iapiserver.InfraRuntimeOutput
	if err := decodeInfraJSON(response, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) request(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func decodeInfraJSON(response *http.Response, target any) error {
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxRequestBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxRequestBytes {
		return fmt.Errorf("infrastructure response is too large")
	}
	var status infraErrorResponse
	if err := json.Unmarshal(raw, &status); err == nil && status.Value != 0 {
		return toolerrors.NewStatus(status.Value, status.Message)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("infrastructure request failed with status %d", response.StatusCode)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode infrastructure response: %w", err)
	}
	return nil
}

func decodeInfraError(response *http.Response) error {
	var status infraErrorResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, maxRequestBytes)).Decode(&status); err != nil {
		return fmt.Errorf("infrastructure request failed with status %d", response.StatusCode)
	}
	if status.Value == 0 {
		return fmt.Errorf("infrastructure request failed with status %d", response.StatusCode)
	}
	return toolerrors.NewStatus(status.Value, status.Message)
}

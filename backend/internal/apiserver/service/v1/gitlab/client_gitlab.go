package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/httpcli/httpconfig"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

const gitLabMaxResponseBytes = 4 << 20

type HTTPClientFactory struct {
	timeout time.Duration
}

func NewHTTPClientFactory() *HTTPClientFactory { return &HTTPClientFactory{timeout: 30 * time.Second} }

func (f *HTTPClientFactory) NewClient(server *iapiserver.GitLabServer) (Client, error) {
	if server == nil || server.APIURL == "" || server.Credential == "" {
		return nil, fmt.Errorf("gitlab server connection is incomplete")
	}
	baseURL, err := url.Parse(strings.TrimRight(server.APIURL, "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("gitlab api_url is invalid")
	}
	client, err := httpcli.NewClient(&httpconfig.HttpConfig{Timeout: f.timeout})
	if err != nil {
		return nil, fmt.Errorf("construct gitlab http client: %w", err)
	}
	return &httpClient{baseURL: baseURL.String(), token: server.Credential, client: client}, nil
}

type httpClient struct {
	baseURL string
	token   string
	client  *httpcli.Client
}

func (c *httpClient) GetVersion(ctx context.Context) (*Version, error) {
	result := &Version{}
	return result, c.do(ctx, http.MethodGet, "/version", nil, result)
}

func (c *httpClient) GetCurrentUser(ctx context.Context) (*User, error) {
	result := &User{}
	return result, c.do(ctx, http.MethodGet, "/user", nil, result)
}

func (c *httpClient) ResolveNamespace(ctx context.Context, path string) (*Namespace, error) {
	result := &Namespace{}
	return result, c.do(ctx, http.MethodGet, "/groups/"+url.PathEscape(path), nil, result)
}

func (c *httpClient) CreateProject(ctx context.Context, input CreateProjectRequest) (*RemoteProject, error) {
	body := struct {
		Name                 string `json:"name"`
		Path                 string `json:"path"`
		Description          string `json:"description,omitempty"`
		NamespaceID          int64  `json:"namespace_id"`
		Visibility           string `json:"visibility"`
		InitializeWithReadme bool   `json:"initialize_with_readme"`
		DefaultBranch        string `json:"default_branch"`
	}{input.Name, input.Path, input.Description, input.NamespaceID, "private", true, "main"}
	result := &RemoteProject{}
	return result, c.do(ctx, http.MethodPost, "/projects", body, result)
}

func (c *httpClient) GetProject(ctx context.Context, projectID int64) (*RemoteProject, error) {
	result := &RemoteProject{}
	return result, c.do(ctx, http.MethodGet, projectPath(projectID), nil, result)
}

func (c *httpClient) DeleteProject(ctx context.Context, projectID int64) error {
	return c.do(ctx, http.MethodDelete, projectPath(projectID), nil, nil)
}

func (c *httpClient) CreatePipeline(ctx context.Context, projectID int64, input CreatePipelineRequest) (*Pipeline, error) {
	type variable struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	variables := make([]variable, 0, len(input.Variables))
	for key, value := range input.Variables {
		variables = append(variables, variable{Key: key, Value: value})
	}
	body := struct {
		Ref       string     `json:"ref"`
		Variables []variable `json:"variables,omitempty"`
	}{Ref: input.Ref, Variables: variables}
	result := &Pipeline{}
	return result, c.do(ctx, http.MethodPost, projectPath(projectID)+"/pipeline", body, result)
}

func (c *httpClient) GetPipeline(ctx context.Context, projectID, pipelineID int64) (*Pipeline, error) {
	result := &Pipeline{}
	return result, c.do(ctx, http.MethodGet, pipelinePath(projectID, pipelineID), nil, result)
}

func (c *httpClient) RetryPipeline(ctx context.Context, projectID, pipelineID int64) (*Pipeline, error) {
	result := &Pipeline{}
	return result, c.do(ctx, http.MethodPost, pipelinePath(projectID, pipelineID)+"/retry", struct{}{}, result)
}

func (c *httpClient) CancelPipeline(ctx context.Context, projectID, pipelineID int64) (*Pipeline, error) {
	result := &Pipeline{}
	return result, c.do(ctx, http.MethodPost, pipelinePath(projectID, pipelineID)+"/cancel", struct{}{}, result)
}

func (c *httpClient) ListPipelineJobs(ctx context.Context, projectID, pipelineID int64) ([]Job, error) {
	result := make([]Job, 0)
	if err := c.do(ctx, http.MethodGet, pipelinePath(projectID, pipelineID)+"/jobs", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *httpClient) do(ctx context.Context, method, path string, body, output any) error {
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(c.baseURL).WithPath(path).WithMethod(method).
		AddHeaderParam("PRIVATE-TOKEN", c.token).AddHeaderParam("Accept", "application/json")
	if body != nil {
		builder.AddHeaderParam("Content-Type", "application/json").WithBody("application/json", body)
	}
	response, err := c.client.Invoke(ctx, builder.Build(), nil, nil)
	if err != nil {
		return fmt.Errorf("gitlab %s request failed: %w", operationName(method, path), err)
	}
	if response == nil || response.Response == nil {
		return fmt.Errorf("gitlab %s returned no response", operationName(method, path))
	}
	defer response.Response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Response.Body, gitLabMaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read gitlab %s response: %w", operationName(method, path), err)
	}
	if len(data) > gitLabMaxResponseBytes {
		return fmt.Errorf("gitlab %s response exceeds size limit", operationName(method, path))
	}
	status := response.GetStatusCode()
	if status < 200 || status >= 300 {
		return &RemoteError{StatusCode: status, Operation: operationName(method, path), Message: gitLabErrorMessage(data)}
	}
	if output == nil || status == http.StatusNoContent {
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("decode gitlab %s response: body is empty", operationName(method, path))
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode gitlab %s response: %w", operationName(method, path), err)
	}
	return nil
}

func gitLabErrorMessage(data []byte) string {
	var payload struct {
		Message json.RawMessage `json:"message"`
	}
	if len(data) == 0 || json.Unmarshal(data, &payload) != nil || len(payload.Message) == 0 {
		return ""
	}
	var message string
	if json.Unmarshal(payload.Message, &message) == nil {
		return strings.Join(strings.Fields(message), " ")
	}
	var fields map[string][]string
	if json.Unmarshal(payload.Message, &fields) != nil {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+strings.Join(fields[key], ", "))
	}
	return strings.Join(strings.Fields(strings.Join(parts, "; ")), " ")
}

func projectPath(projectID int64) string { return "/projects/" + strconv.FormatInt(projectID, 10) }
func pipelinePath(projectID, pipelineID int64) string {
	return projectPath(projectID) + "/pipelines/" + strconv.FormatInt(pipelineID, 10)
}
func operationName(method, path string) string { return strings.ToLower(method) + " " + path }

package gitlab

import (
	"context"
	"encoding/base64"
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
	initializeWithReadme := input.InitializeWithReadme
	if !input.InitializeWithReadme && input.DefaultBranch == "" {
		initializeWithReadme = false
	}
	defaultBranch := input.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	body := struct {
		Name                 string `json:"name"`
		Path                 string `json:"path"`
		Description          string `json:"description,omitempty"`
		NamespaceID          int64  `json:"namespace_id"`
		Visibility           string `json:"visibility"`
		InitializeWithReadme bool   `json:"initialize_with_readme"`
		DefaultBranch        string `json:"default_branch"`
	}{input.Name, input.Path, input.Description, input.NamespaceID, "private", initializeWithReadme, defaultBranch}
	result := &RemoteProject{}
	return result, c.do(ctx, http.MethodPost, "/projects", body, result)
}

func (c *httpClient) GetProject(ctx context.Context, projectID int64) (*RemoteProject, error) {
	result := &RemoteProject{}
	return result, c.do(ctx, http.MethodGet, projectPath(projectID), nil, result)
}

func (c *httpClient) GetProjectByPath(ctx context.Context, path string) (*RemoteProject, error) {
	result := &RemoteProject{}
	return result, c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(path), nil, result)
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

func (c *httpClient) ListRepositoryTree(ctx context.Context, projectID int64, ref, path string) ([]RepositoryTreeEntry, error) {
	result := make([]RepositoryTreeEntry, 0)
	query := url.Values{"ref": []string{ref}, "per_page": []string{"100"}}
	if path != "" {
		query.Set("path", path)
	}
	if err := c.do(ctx, http.MethodGet, projectPath(projectID)+"/repository/tree?"+query.Encode(), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *httpClient) GetRepositoryFile(ctx context.Context, projectID int64, ref, path string) (*RepositoryFile, error) {
	result := &RepositoryFile{}
	query := url.Values{"ref": []string{ref}}
	return result, c.do(ctx, http.MethodGet, projectPath(projectID)+"/repository/files/"+url.PathEscape(path)+"?"+query.Encode(), nil, result)
}

func (c *httpClient) GetRepositoryArchive(ctx context.Context, projectID int64, ref string) (io.ReadCloser, error) {
	query := url.Values{"sha": []string{ref}}
	return c.stream(ctx, http.MethodGet, projectPath(projectID)+"/repository/archive.tar.gz?"+query.Encode())
}

func (c *httpClient) GetBranchHead(ctx context.Context, projectID int64, branch string) (*BranchHead, error) {
	result := &BranchHead{}
	return result, c.do(ctx, http.MethodGet, projectPath(projectID)+"/repository/branches/"+url.PathEscape(branch), nil, result)
}

func (c *httpClient) CompareCommits(ctx context.Context, projectID int64, from, to string) (*CommitComparison, error) {
	result := &CommitComparison{}
	query := url.Values{"from": []string{from}, "to": []string{to}}
	return result, c.do(ctx, http.MethodGet, projectPath(projectID)+"/repository/compare?"+query.Encode(), nil, result)
}

func (c *httpClient) CreateCommit(ctx context.Context, projectID int64, input CreateCommitRequest) (*Commit, error) {
	body := struct {
		Branch        string         `json:"branch"`
		CommitMessage string         `json:"commit_message"`
		StartBranch   string         `json:"start_branch,omitempty"`
		Actions       []CommitAction `json:"actions"`
	}{Branch: input.Branch, CommitMessage: input.CommitMessage, StartBranch: input.StartBranch, Actions: input.Actions}
	result := &Commit{}
	return result, c.do(ctx, http.MethodPost, projectPath(projectID)+"/repository/commits", body, result)
}

func (c *httpClient) CreateProjectAccessToken(ctx context.Context, projectID int64, input CreateProjectAccessTokenRequest) (*ProjectAccessToken, error) {
	body := struct {
		Name        string   `json:"name"`
		Scopes      []string `json:"scopes"`
		AccessLevel int      `json:"access_level"`
		ExpiresAt   string   `json:"expires_at"`
	}{input.Name, input.Scopes, input.AccessLevel, input.ExpiresAt}
	result := &ProjectAccessToken{}
	return result, c.do(ctx, http.MethodPost, projectPath(projectID)+"/access_tokens", body, result)
}

func (c *httpClient) RevokeProjectAccessToken(ctx context.Context, projectID, tokenID int64) error {
	return c.do(ctx, http.MethodDelete, projectPath(projectID)+"/access_tokens/"+strconv.FormatInt(tokenID, 10), nil, nil)
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

func (c *httpClient) stream(ctx context.Context, method, path string) (io.ReadCloser, error) {
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(c.baseURL).WithPath(path).WithMethod(method).
		AddHeaderParam("PRIVATE-TOKEN", c.token).AddHeaderParam("Accept", "application/gzip")
	response, err := c.client.Invoke(ctx, builder.Build(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab %s request failed: %w", operationName(method, path), err)
	}
	if response == nil || response.Response == nil {
		return nil, fmt.Errorf("gitlab %s returned no response", operationName(method, path))
	}
	if status := response.GetStatusCode(); status < 200 || status >= 300 {
		defer response.Response.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(response.Response.Body, gitLabMaxResponseBytes+1))
		if readErr != nil {
			return nil, fmt.Errorf("gitlab %s response failed", operationName(method, path))
		}
		return nil, &RemoteError{StatusCode: status, Operation: operationName(method, path), Message: gitLabErrorMessage(data)}
	}
	return response.Response.Body, nil
}

func decodeRepositoryFileContent(file *RepositoryFile) ([]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("repository file is nil")
	}
	if file.Encoding == "" || file.Encoding == "base64" {
		return base64.StdEncoding.DecodeString(strings.Join(strings.Fields(file.Content), ""))
	}
	return []byte(file.Content), nil
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

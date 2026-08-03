package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type ProfileImageResolver interface {
	Image(string, string) (string, bool)
}
type MapProfileImages map[string]string

func (m MapProfileImages) Image(id, revision string) (string, bool) {
	value, ok := m[id+"@"+revision]
	return value, ok
}

type DockerProvider struct {
	client     *http.Client
	images     ProfileImageResolver
	apiVersion string
}

func NewDockerProvider(socketPath, apiVersion string, images ProfileImageResolver) (*DockerProvider, error) {
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if apiVersion == "" {
		apiVersion = "v1.44"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
	}}
	provider := &DockerProvider{client: &http.Client{Transport: transport, Timeout: 2 * time.Minute}, images: images, apiVersion: apiVersion}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.request(ctx, http.MethodGet, "/_ping", nil, nil); err != nil {
		return nil, err
	}
	return provider, nil
}
func (d *DockerProvider) Info(ctx context.Context) (*iapiserver.InfraNode, error) {
	var info struct {
		NCPU          int    `json:"NCPU"`
		MemTotal      int64  `json:"MemTotal"`
		DockerRootDir string `json:"DockerRootDir"`
	}
	if err := d.request(ctx, http.MethodGet, "/info", nil, &info); err != nil {
		return nil, err
	}
	return &iapiserver.InfraNode{ObjectMeta: imachinery.ObjectMeta{ID: "docker-local", Name: "Docker Local"}, ProviderType: "docker", Status: "ONLINE", CPUCores: float64(info.NCPU), MemoryMB: info.MemTotal / (1024 * 1024), LastHeartbeatAt: imachinery.Now()}, nil
}
func (d *DockerProvider) Ensure(ctx context.Context, input ProviderRequest) (*ProviderResult, error) {
	if d.images == nil {
		return nil, fmt.Errorf("docker profile image catalog is unavailable")
	}
	image, ok := d.images.Image(input.Profile.ID, input.Profile.Revision)
	if !ok || image == "" {
		return nil, fmt.Errorf("docker image is not configured for profile %s@%s", input.Profile.ID, input.Profile.Revision)
	}
	if len(input.Request.Mounts) > 0 {
		return nil, fmt.Errorf("docker source resolver is not configured for requested mounts")
	}
	env := make([]string, 0, len(input.Request.ConfigurationBindings))
	for _, binding := range input.Request.ConfigurationBindings {
		if binding.BindingType != "PLAIN_CONFIG" {
			return nil, fmt.Errorf("binding %s requires a configured secret/model resolver", binding.Name)
		}
		env = append(env, normalizedEnvName(binding.Name)+"="+binding.Reference)
	}
	body := map[string]any{"Image": image, "Env": env, "Labels": map[string]string{"io.omnimam.runtime_id": input.RuntimeID, "io.omnimam.profile": input.Profile.ID}, "HostConfig": map[string]any{"ReadonlyRootfs": true, "Privileged": false, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "NetworkMode": "bridge", "AutoRemove": false, "PidsLimit": 256}}
	var created struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	path := "/containers/create?name=" + url.QueryEscape("omnimam-"+input.RuntimeID)
	if err := d.request(ctx, http.MethodPost, path, body, &created); err != nil {
		return nil, err
	}
	if created.ID == "" {
		return nil, fmt.Errorf("docker returned no container id")
	}
	if err := d.request(ctx, http.MethodPost, "/containers/"+created.ID+"/start", nil, nil); err != nil {
		_ = d.Delete(context.Background(), created.ID)
		return nil, err
	}
	if input.Request.RuntimeMode == "JOB" {
		status, err := d.wait(ctx, created.ID)
		if err != nil {
			return nil, err
		}
		if status != 0 {
			return nil, fmt.Errorf("docker job exited with code %d", status)
		}
		return &ProviderResult{ProviderRuntimeRef: created.ID, Status: "SUCCEEDED"}, nil
	}
	return &ProviderResult{ProviderRuntimeRef: created.ID, Status: "RUNNING", EndpointDisplayRef: "infra-runtime://" + input.RuntimeID}, nil
}
func (d *DockerProvider) Start(ctx context.Context, ref string) (*ProviderResult, error) {
	if err := d.request(ctx, http.MethodPost, "/containers/"+ref+"/start", nil, nil); err != nil && !strings.Contains(err.Error(), "304") {
		return nil, err
	}
	return &ProviderResult{ProviderRuntimeRef: ref, Status: "RUNNING"}, nil
}
func (d *DockerProvider) Stop(ctx context.Context, ref string, deleteRuntime bool) (*ProviderResult, error) {
	if ref == "" {
		return nil, fmt.Errorf("docker runtime reference is empty")
	}
	_ = d.request(ctx, http.MethodPost, "/containers/"+ref+"/stop?t=10", nil, nil)
	if deleteRuntime {
		if err := d.Delete(ctx, ref); err != nil {
			return nil, err
		}
		return &ProviderResult{ProviderRuntimeRef: ref, Status: "DELETED"}, nil
	}
	return &ProviderResult{ProviderRuntimeRef: ref, Status: "STOPPED"}, nil
}
func (d *DockerProvider) Delete(ctx context.Context, ref string) error {
	return d.request(ctx, http.MethodDelete, "/containers/"+ref+"?force=true&v=true", nil, nil)
}
func (d *DockerProvider) Inspect(ctx context.Context, ref string) (*ProviderResult, error) {
	var data struct {
		State struct {
			Status   string `json:"Status"`
			Running  bool   `json:"Running"`
			ExitCode int    `json:"ExitCode"`
		} `json:"State"`
	}
	if err := d.request(ctx, http.MethodGet, "/containers/"+ref+"/json", nil, &data); err != nil {
		return nil, err
	}
	status := "STOPPED"
	if data.State.Running {
		status = "RUNNING"
	} else if data.State.ExitCode == 0 {
		status = "SUCCEEDED"
	} else {
		status = "FAILED"
	}
	return &ProviderResult{ProviderRuntimeRef: ref, Status: status}, nil
}
func (d *DockerProvider) Logs(ctx context.Context, ref string, limit int) ([]*iapiserver.InfraRuntimeLogEntry, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	var raw bytes.Buffer
	if err := d.requestRaw(ctx, http.MethodGet, "/containers/"+ref+"/logs?stdout=true&stderr=true&timestamps=true&tail="+strconv.Itoa(limit), nil, &raw); err != nil {
		return nil, err
	}
	items := make([]*iapiserver.InfraRuntimeLogEntry, 0)
	scanner := bufio.NewScanner(&raw)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 256*1024)
	for scanner.Scan() {
		line := sanitizeLogLine(scanner.Text())
		occurred := imachinery.Now()
		message := line
		if index := strings.IndexByte(line, ' '); index > 0 {
			if parsed, err := time.Parse(time.RFC3339Nano, line[:index]); err == nil {
				occurred = imachinery.Time{Time: parsed}
				message = line[index+1:]
			}
		}
		items = append(items, &iapiserver.InfraRuntimeLogEntry{OccurredAt: occurred, Level: "INFO", Message: message, Source: "docker"})
	}
	return items, scanner.Err()
}
func (d *DockerProvider) wait(ctx context.Context, ref string) (int, error) {
	var response struct {
		StatusCode int `json:"StatusCode"`
		Error      *struct {
			Message string `json:"Message"`
		} `json:"Error"`
	}
	if err := d.request(ctx, http.MethodPost, "/containers/"+ref+"/wait?condition=not-running", nil, &response); err != nil {
		return 0, err
	}
	if response.Error != nil && response.Error.Message != "" {
		return 0, fmt.Errorf("docker wait: %s", response.Error.Message)
	}
	return response.StatusCode, nil
}
func (d *DockerProvider) request(ctx context.Context, method, path string, body, out any) error {
	var target bytes.Buffer
	if err := d.requestRaw(ctx, method, path, body, &target); err != nil {
		return err
	}
	if out == nil || target.Len() == 0 {
		return nil
	}
	return json.Unmarshal(target.Bytes(), out)
}
func (d *DockerProvider) requestRaw(ctx context.Context, method, path string, body any, out io.Writer) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://docker/"+d.apiVersion+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		return fmt.Errorf("docker api %s: %s", response.Status, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		_, err = io.Copy(out, io.LimitReader(response.Body, 8<<20))
		return err
	}
	return nil
}
func normalizedEnvName(value string) string {
	value = strings.ToUpper(value)
	var builder strings.Builder
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return "OMNIMAM_BINDING_" + builder.String()
}
func sanitizeLogLine(value string) string {
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization:", "bearer ", "api_key", "apikey", "secret", "password"} {
		if strings.Contains(lower, marker) {
			return "[REDACTED]"
		}
	}
	if len(value) > 16*1024 {
		return value[:16*1024]
	}
	return value
}

var _ RuntimeProvider = (*DockerProvider)(nil)

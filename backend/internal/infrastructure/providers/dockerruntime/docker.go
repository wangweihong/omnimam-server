package dockerruntime

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
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure/providers"
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
	client      *http.Client
	runtimeHTTP *http.Client
	images      ProfileImageResolver
	apiVersion  string
	networkMode string
}

type dockerContainerInspect struct {
	Name   string `json:"Name"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status   string `json:"Status"`
		Running  bool   `json:"Running"`
		ExitCode int    `json:"ExitCode"`
	} `json:"State"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type agentServiceProfile struct {
	healthPath string
	port       int
	command    string
	tmpfs      map[string]string
}

func NewDockerProvider(socketPath, apiVersion string, images ProfileImageResolver) (providers.RuntimeProvider, error) {
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if apiVersion == "" {
		apiVersion = "v1.44"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
	}}
	provider := &DockerProvider{
		client:      &http.Client{Transport: transport, Timeout: 2 * time.Minute},
		runtimeHTTP: &http.Client{Timeout: 5 * time.Second},
		images:      images,
		apiVersion:  apiVersion,
		networkMode: "bridge",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.request(ctx, http.MethodGet, "/_ping", nil, nil); err != nil {
		return nil, err
	}
	provider.networkMode = provider.detectNetwork(ctx)
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
	return &iapiserver.InfraNode{ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraNodeIDDockerLocal, Name: iapiserver.InfraNodeNameDockerLocal}, ProviderType: iapiserver.InfraProviderTypeDocker, Status: iapiserver.InfraNodeStatusOnline, CPUCores: float64(info.NCPU), MemoryMB: info.MemTotal / (1024 * 1024), LastHeartbeatAt: imachinery.Now()}, nil
}
func (d *DockerProvider) Ensure(ctx context.Context, input providers.ProviderRequest) (*providers.ProviderResult, error) {
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
		if binding.BindingType != iapiserver.InfraConfigBindingTypePlainConfig {
			return nil, fmt.Errorf("binding %s requires a configured secret/model resolver", binding.Name)
		}
		env = append(env, normalizedEnvName(binding.Name)+"="+binding.Reference)
	}
	hostConfig := map[string]any{"ReadonlyRootfs": true, "Privileged": false, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "NetworkMode": d.networkMode, "AutoRemove": false, "PidsLimit": 256}
	body := map[string]any{"Image": image, "Env": env, "Labels": map[string]string{"io.omnimam.runtime_id": input.RuntimeID, "io.omnimam.profile": input.Profile.ID}, "HostConfig": hostConfig}
	serviceProfile, agentService := agentRuntimeServiceProfile(input.Profile.ID)
	if agentService {
		body["Entrypoint"] = []string{"/bin/sh", "-c"}
		body["Cmd"] = []string{serviceProfile.command}
		hostConfig["Tmpfs"] = serviceProfile.tmpfs
	}
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
	if input.Request.RuntimeMode == iapiserver.InfraRuntimeModeJob {
		status, err := d.wait(ctx, created.ID)
		if err != nil {
			return nil, err
		}
		if status != 0 {
			return nil, fmt.Errorf("docker job exited with code %d", status)
		}
		return &providers.ProviderResult{ProviderRuntimeRef: created.ID, Status: iapiserver.InfraRuntimeStatusSucceeded}, nil
	}
	if agentService {
		result, err := d.waitForAgentService(ctx, created.ID, serviceProfile)
		if err != nil {
			_ = d.Delete(context.Background(), created.ID)
			return nil, err
		}
		return result, nil
	}
	state, err := d.Inspect(ctx, created.ID)
	if err != nil {
		_ = d.Delete(context.Background(), created.ID)
		return nil, fmt.Errorf("inspect docker runtime after start: %w", err)
	}
	if state.Status != iapiserver.InfraRuntimeStatusRunning {
		_ = d.Delete(context.Background(), created.ID)
		return nil, fmt.Errorf("docker runtime exited after start with status %s", state.Status)
	}
	return &providers.ProviderResult{ProviderRuntimeRef: created.ID, Status: iapiserver.InfraRuntimeStatusRunning, EndpointDisplayRef: iapiserver.InfraEndpointDisplayRefPrefix + input.RuntimeID}, nil
}
func (d *DockerProvider) Start(ctx context.Context, ref string) (*providers.ProviderResult, error) {
	if err := d.request(ctx, http.MethodPost, "/containers/"+ref+"/start", nil, nil); err != nil && !strings.Contains(err.Error(), "304") {
		return nil, err
	}
	data, err := d.inspectContainer(ctx, ref)
	if err != nil {
		return nil, err
	}
	if profile, ok := agentRuntimeServiceProfile(data.Config.Labels["io.omnimam.profile"]); ok {
		return d.waitForAgentService(ctx, ref, profile)
	}
	return &providers.ProviderResult{ProviderRuntimeRef: ref, Status: iapiserver.InfraRuntimeStatusRunning}, nil
}
func (d *DockerProvider) Stop(ctx context.Context, ref string, deleteRuntime bool) (*providers.ProviderResult, error) {
	if ref == "" {
		return nil, fmt.Errorf("docker runtime reference is empty")
	}
	_ = d.request(ctx, http.MethodPost, "/containers/"+ref+"/stop?t=10", nil, nil)
	if deleteRuntime {
		if err := d.Delete(ctx, ref); err != nil {
			return nil, err
		}
		return &providers.ProviderResult{ProviderRuntimeRef: ref, Status: iapiserver.InfraRuntimeStatusDeleted}, nil
	}
	return &providers.ProviderResult{ProviderRuntimeRef: ref, Status: iapiserver.InfraRuntimeStatusStopped}, nil
}
func (d *DockerProvider) Delete(ctx context.Context, ref string) error {
	return d.request(ctx, http.MethodDelete, "/containers/"+ref+"?force=true&v=true", nil, nil)
}
func (d *DockerProvider) Inspect(ctx context.Context, ref string) (*providers.ProviderResult, error) {
	data, err := d.inspectContainer(ctx, ref)
	if err != nil {
		return nil, err
	}
	status := iapiserver.InfraRuntimeStatusStopped
	if data.State.Running {
		status = iapiserver.InfraRuntimeStatusRunning
	} else if data.State.ExitCode == 0 {
		status = iapiserver.InfraRuntimeStatusSucceeded
	} else {
		status = iapiserver.InfraRuntimeStatusFailed
	}
	result := &providers.ProviderResult{ProviderRuntimeRef: ref, Status: status}
	if profile, ok := agentRuntimeServiceProfile(data.Config.Labels["io.omnimam.profile"]); ok && data.State.Running {
		endpoint, endpointErr := d.agentServiceEndpoint(data, profile)
		if endpointErr != nil {
			return nil, endpointErr
		}
		result.EndpointDisplayRef = iapiserver.InfraEndpointDisplayRefPrefix + data.Config.Labels["io.omnimam.runtime_id"]
		result.Endpoint = endpoint
	}
	return result, nil
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
		items = append(items, &iapiserver.InfraRuntimeLogEntry{OccurredAt: occurred, Level: iapiserver.InfraRuntimeLogLevelInfo, Message: message, Source: iapiserver.InfraRuntimeLogSourceDocker})
	}
	return items, scanner.Err()
}

func (d *DockerProvider) inspectContainer(ctx context.Context, ref string) (*dockerContainerInspect, error) {
	var data dockerContainerInspect
	if err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(ref)+"/json", nil, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (d *DockerProvider) detectNetwork(ctx context.Context) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "bridge"
	}
	data, err := d.inspectContainer(ctx, hostname)
	if err != nil || len(data.NetworkSettings.Networks) == 0 {
		return "bridge"
	}
	names := make([]string, 0, len(data.NetworkSettings.Networks))
	for name := range data.NetworkSettings.Networks {
		if name != "bridge" && name != "host" && name != "none" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "bridge"
	}
	sort.Strings(names)
	return names[0]
}

func agentRuntimeServiceProfile(profileID string) (agentServiceProfile, bool) {
	switch profileID {
	case iapiserver.InfraRuntimeProfileIDAgentCoding:
		return agentServiceProfile{
			healthPath: "/global/health",
			port:       14096,
			command: `set -eu
printf '#!/bin/sh\nexec nc 127.0.0.1 4096\n' > /run/omnimam/forward
chmod 500 /run/omnimam/forward
nc -lk -p 14096 -e /run/omnimam/forward &
exec opencode serve --hostname 127.0.0.1 --port 4096`,
			tmpfs: map[string]string{
				"/tmp":                        "rw,nosuid,nodev,noexec,size=64m",
				"/run/omnimam":                "rw,nosuid,nodev,exec,size=1m",
				"/root/.cache/opencode":       "rw,nosuid,nodev,noexec,size=64m",
				"/root/.config/opencode":      "rw,nosuid,nodev,noexec,size=32m",
				"/root/.local/share/opencode": "rw,nosuid,nodev,noexec,size=128m",
			},
		}, true
	case iapiserver.InfraRuntimeProfileIDAgentHermes:
		return agentServiceProfile{
			healthPath: "/openapi.json",
			port:       19119,
			command: `set -eu
python3 -c 'import asyncio
async def copy(reader, writer):
    try:
        while True:
            data = await reader.read(65536)
            if not data:
                break
            writer.write(data)
            await writer.drain()
    finally:
        writer.close()
async def handle(reader, writer):
    upstream_reader, upstream_writer = await asyncio.open_connection("127.0.0.1", 9119)
    await asyncio.gather(copy(reader, upstream_writer), copy(upstream_reader, writer))
async def main():
    server = await asyncio.start_server(handle, "0.0.0.0", 19119)
    async with server:
        await server.serve_forever()
asyncio.run(main())' &
exec hermes serve --host 127.0.0.1 --port 9119 --skip-build`,
			tmpfs: map[string]string{
				"/tmp":          "rw,nosuid,nodev,noexec,size=64m",
				"/root/.cache":  "rw,nosuid,nodev,noexec,size=64m",
				"/root/.config": "rw,nosuid,nodev,noexec,size=32m",
				"/root/.hermes": "rw,nosuid,nodev,noexec,size=128m",
			},
		}, true
	default:
		return agentServiceProfile{}, false
	}
}

func (d *DockerProvider) agentServiceEndpoint(data *dockerContainerInspect, profile agentServiceProfile) (*providers.ProviderEndpoint, error) {
	host := strings.TrimPrefix(data.Name, "/")
	if d.networkMode == "bridge" {
		host = ""
		if network, ok := data.NetworkSettings.Networks[d.networkMode]; ok {
			host = network.IPAddress
		}
	}
	if host == "" {
		return nil, fmt.Errorf("docker agent runtime has no reachable network address")
	}
	return &providers.ProviderEndpoint{Protocol: iapiserver.InfraProtocolHTTP, BaseURL: "http://" + net.JoinHostPort(host, strconv.Itoa(profile.port))}, nil
}

func (d *DockerProvider) waitForAgentService(ctx context.Context, ref string, profile agentServiceProfile) (*providers.ProviderResult, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var lastErr error
	for {
		result, err := d.Inspect(waitCtx, ref)
		if err != nil {
			lastErr = err
		} else if result.Status != iapiserver.InfraRuntimeStatusRunning {
			return nil, fmt.Errorf("docker agent runtime exited during startup with status %s", result.Status)
		} else if result.Endpoint != nil {
			request, requestErr := http.NewRequestWithContext(waitCtx, http.MethodGet, result.Endpoint.BaseURL+profile.healthPath, nil)
			if requestErr != nil {
				return nil, requestErr
			}
			response, requestErr := d.runtimeHTTP.Do(request)
			if requestErr == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
				closeErr := response.Body.Close()
				if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices && closeErr == nil {
					return result, nil
				}
				lastErr = fmt.Errorf("agent runtime health check returned status %d", response.StatusCode)
			} else {
				lastErr = requestErr
			}
		}
		select {
		case <-waitCtx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("wait for docker agent runtime health: %w", lastErr)
			}
			return nil, fmt.Errorf("wait for docker agent runtime health: %w", waitCtx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
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

var _ providers.RuntimeProvider = (*DockerProvider)(nil)

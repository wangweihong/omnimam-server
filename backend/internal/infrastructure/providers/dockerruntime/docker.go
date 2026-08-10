package dockerruntime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
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

	"github.com/wangweihong/gotoolbox/pkg/wait"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure/providers"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentmcp"
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
	client       *http.Client
	runtimeHTTP  *http.Client
	images       ProfileImageResolver
	apiVersion   string
	networkMode  string
	runtimeCA    []byte
	sourceVolume string
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

const (
	openCodeConfigInstallCommand = "umask 077; cat > /root/.config/opencode/opencode.json && chmod 600 /root/.config/opencode/opencode.json && touch /run/omnimam/start"
	openCodeCAInstallCommand     = "umask 077; cat > /run/omnimam/mcp-ca.crt && chmod 400 /run/omnimam/mcp-ca.crt"
	openCodeRuntimeCAPath        = "/run/omnimam/mcp-ca.crt"
	previewContentRoot           = "/usr/share/nginx/html"
	previewRuntimeCommand        = `set -eu
cp -R /mnt/omnimam/source/. /usr/share/nginx/html/
chmod -R a+rX,a-w /usr/share/nginx/html
exec /docker-entrypoint.sh nginx -g 'daemon off;'`
	maxRuntimeCABundleBytes   = 1 << 20
	dockerExecInspectInterval = 50 * time.Millisecond
	dockerExecInspectTimeout  = 5 * time.Second
)

func NewDockerProvider(socketPath, apiVersion string, images ProfileImageResolver, runtimeCAFile, sourceVolume string) (providers.RuntimeProvider, error) {
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if apiVersion == "" {
		apiVersion = "v1.45"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
	}}
	provider := &DockerProvider{
		client:       &http.Client{Transport: transport, Timeout: 2 * time.Minute},
		runtimeHTTP:  &http.Client{Timeout: 5 * time.Second},
		images:       images,
		apiVersion:   apiVersion,
		networkMode:  "bridge",
		sourceVolume: strings.TrimSpace(sourceVolume),
	}
	runtimeCA, err := readRuntimeCA(runtimeCAFile)
	if err != nil {
		return nil, err
	}
	provider.runtimeCA = runtimeCA
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.request(ctx, http.MethodGet, "/_ping", nil, nil); err != nil {
		return nil, err
	}
	provider.networkMode = provider.detectNetwork(ctx)
	return provider, nil
}

func readRuntimeCA(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	runtimeCA, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read coding runtime MCP CA bundle: %w", err)
	}
	if len(runtimeCA) == 0 || len(runtimeCA) > maxRuntimeCABundleBytes {
		return nil, fmt.Errorf("coding runtime MCP CA bundle must contain 1 to %d bytes", maxRuntimeCABundleBytes)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(runtimeCA) {
		return nil, fmt.Errorf("coding runtime MCP CA bundle contains no PEM certificates")
	}
	return runtimeCA, nil
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
	if input.Profile == nil || input.Request == nil {
		return nil, fmt.Errorf("docker runtime profile and request are required")
	}
	if d.images == nil {
		return nil, fmt.Errorf("docker profile image catalog is unavailable")
	}
	image, ok := d.images.Image(input.Profile.ID, input.Profile.Revision)
	if !ok || image == "" {
		return nil, fmt.Errorf("docker image is not configured for profile %s@%s", input.Profile.ID, input.Profile.Revision)
	}
	env := make([]string, 0, len(input.Request.ConfigurationBindings))
	for _, binding := range input.Request.ConfigurationBindings {
		if binding.BindingType == iapiserver.InfraConfigBindingTypeMCPServerRef {
			continue
		}
		if binding.BindingType != iapiserver.InfraConfigBindingTypePlainConfig {
			return nil, fmt.Errorf("binding %s requires a configured secret/model resolver", binding.Name)
		}
		env = append(env, normalizedEnvName(binding.Name)+"="+binding.Reference)
	}
	if input.Profile.ID == iapiserver.InfraRuntimeProfileIDAgentCoding && len(d.runtimeCA) > 0 {
		const caEnvPrefix = "NODE_EXTRA_CA_CERTS="
		filtered := env[:0]
		for _, value := range env {
			if !strings.HasPrefix(value, caEnvPrefix) {
				filtered = append(filtered, value)
			}
		}
		env = append(filtered, caEnvPrefix+openCodeRuntimeCAPath)
	}
	hostConfig := map[string]any{"ReadonlyRootfs": true, "Privileged": false, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "NetworkMode": d.networkMode, "AutoRemove": false, "PidsLimit": 256}
	body := map[string]any{"Image": image, "Env": env, "Labels": map[string]string{"io.omnimam.runtime_id": input.RuntimeID, "io.omnimam.profile": input.Profile.ID}, "HostConfig": hostConfig}
	serviceProfile, agentService := agentRuntimeServiceProfile(input.Profile.ID)
	if input.Profile.ID == iapiserver.InfraRuntimeProfileIDAppStudioPreviewWeb {
		// 官方 Nginx 镜像需要写入缓存/PID，并由 master 将缓存目录交给 worker；仅恢复该启动路径所需权限。
		hostConfig["Tmpfs"] = map[string]string{
			"/var/cache/nginx": "rw,nosuid,nodev,noexec,size=16m",
			"/var/run":         "rw,nosuid,nodev,noexec,size=1m",
			previewContentRoot: "rw,nosuid,nodev,noexec,size=64m",
		}
		hostConfig["CapAdd"] = []string{"CHOWN", "SETGID", "SETUID"}
		if err := d.configurePreviewSource(input, body, hostConfig); err != nil {
			return nil, err
		}
	} else if len(input.Request.Mounts) > 0 {
		return nil, fmt.Errorf("docker source resolver does not support mounts for profile %s", input.Profile.ID)
	}
	if agentService {
		if len(input.MCPBindings) > 0 && input.Profile.ID != iapiserver.InfraRuntimeProfileIDAgentCoding {
			return nil, fmt.Errorf("MCP bindings are only supported by the coding runtime")
		}
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
	if input.Profile.ID == iapiserver.InfraRuntimeProfileIDAgentCoding {
		if len(d.runtimeCA) > 0 {
			if err := d.execWithInput(ctx, created.ID, []string{"/bin/sh", "-c", openCodeCAInstallCommand}, d.runtimeCA); err != nil {
				_ = d.Delete(context.Background(), created.ID)
				return nil, fmt.Errorf("install coding runtime MCP CA bundle: %w", err)
			}
		}
		if err := d.injectOpenCodeConfig(ctx, created.ID, input.MCPBindings); err != nil {
			_ = d.Delete(context.Background(), created.ID)
			return nil, err
		}
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

func (d *DockerProvider) configurePreviewSource(input providers.ProviderRequest, body, hostConfig map[string]any) error {
	if len(input.Request.Mounts) != 1 {
		return fmt.Errorf("static web Preview requires exactly one source revision mount")
	}
	mount := input.Request.Mounts[0]
	if mount.MountKind != iapiserver.InfraMountKindStudioWorkspaceRevision || !mount.ReadOnly || mount.TargetPath != iapiserver.InfraRuntimeMountTargetAppStudioStaticWebSource || mount.SourceRef != input.Request.SourceRef {
		return fmt.Errorf("static web Preview source revision mount is invalid")
	}
	if d.sourceVolume == "" || strings.TrimSpace(d.sourceVolume) != d.sourceVolume || strings.ContainsAny(d.sourceVolume, `/\\`) || d.sourceVolume == "." || d.sourceVolume == ".." {
		return fmt.Errorf("AppStudio source volume is not configured")
	}
	subpath, err := previewSourceSubpath(mount.SourceRef, input.Request.AuthorizationRef, input.Request.OwnerReference)
	if err != nil {
		return err
	}
	hostConfig["Mounts"] = []map[string]any{{
		"Type":     "volume",
		"Source":   d.sourceVolume,
		"Target":   mount.TargetPath,
		"ReadOnly": true,
		"VolumeOptions": map[string]any{
			"Subpath": subpath,
		},
	}}
	// 每次容器启动都从受控只读 Revision 重建 tmpfs，避免 stop/start 后丢失 Preview 内容。
	body["Entrypoint"] = []string{"/bin/sh", "-c"}
	body["Cmd"] = []string{previewRuntimeCommand}
	return nil
}

func previewSourceSubpath(sourceRef, authorizationRef, ownerReference string) (string, error) {
	source := strings.TrimPrefix(sourceRef, iapiserver.InfraRefPrefixStudioWorkspaceRevision)
	if source == sourceRef {
		return "", fmt.Errorf("Preview source reference is invalid")
	}
	sourceParts := strings.Split(source, "/")
	if len(sourceParts) != 2 || !validPreviewReferencePart(sourceParts[0]) {
		return "", fmt.Errorf("Preview source reference is invalid")
	}
	revision, err := strconv.ParseInt(sourceParts[1], 10, 64)
	if err != nil || revision < 0 {
		return "", fmt.Errorf("Preview source reference is invalid")
	}
	grant := strings.TrimPrefix(authorizationRef, iapiserver.AppStudioRefPrefixPreviewGrant)
	if grant == authorizationRef {
		return "", fmt.Errorf("Preview authorization reference is invalid")
	}
	grantParts := strings.Split(grant, "/")
	if len(grantParts) != 3 || !validPreviewReferencePart(grantParts[0]) || !validPreviewReferencePart(grantParts[1]) {
		return "", fmt.Errorf("Preview authorization reference is invalid")
	}
	resourceVersion, err := strconv.ParseInt(grantParts[2], 10, 64)
	if err != nil || resourceVersion < 0 || grantParts[0] != sourceParts[0] || grantParts[1] != ownerReference {
		return "", fmt.Errorf("Preview source reference is outside its authorization scope")
	}
	return sourceParts[0] + "/" + strconv.FormatInt(revision, 10), nil
}

func validPreviewReferencePart(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\%\x00`)
}

func (d *DockerProvider) injectOpenCodeConfig(ctx context.Context, ref string, bindings []providers.ResolvedMCPBinding) error {
	mcp := make(map[string]any, len(bindings))
	tools := make(map[string]bool)
	for _, binding := range bindings {
		rawConfiguration, err := json.Marshal(binding.Configuration)
		if err != nil {
			return fmt.Errorf("MCP configuration is invalid")
		}
		configuration, err := agentmcp.ParseConfiguration(rawConfiguration)
		if err != nil {
			return fmt.Errorf("MCP configuration is invalid")
		}
		entry := make(map[string]any, len(configuration)+4)
		for key, value := range configuration {
			entry[key] = value
		}
		entry["type"] = "remote"
		entry["url"] = binding.Endpoint
		entry["enabled"] = true
		if binding.Credential != "" {
			entry["headers"] = map[string]string{"Authorization": "Bearer " + binding.Credential}
		}
		mcp[binding.ServerKey] = entry
		tools[binding.ServerKey+"_*"] = false
		for _, tool := range binding.AllowedTools {
			tools[binding.ServerKey+"_"+tool] = true
		}
	}
	config, err := json.Marshal(map[string]any{"mcp": mcp, "tools": tools})
	if err != nil {
		return err
	}
	return d.execWithInput(ctx, ref, []string{"/bin/sh", "-c", openCodeConfigInstallCommand}, config)
}

// execWithInput 通过 Docker hijacked stream 传递敏感 stdin，避免把配置写入容器命令、环境变量或 daemon 错误正文。
func (d *DockerProvider) execWithInput(ctx context.Context, ref string, command []string, input []byte) error {
	var created struct {
		ID string `json:"Id"`
	}
	if err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(ref)+"/exec", map[string]any{
		"AttachStdin": true, "AttachStdout": false, "AttachStderr": false,
		"Tty": false, "Privileged": false, "Cmd": command,
	}, &created); err != nil {
		return err
	}
	if created.ID == "" {
		return fmt.Errorf("docker returned no stdin exec id")
	}
	startBody, err := json.Marshal(map[string]any{"Detach": false, "Tty": false})
	if err != nil {
		return fmt.Errorf("encode docker exec start request: %w", err)
	}
	operationCtx := ctx
	cancelOperation := func() {}
	if d.client.Timeout > 0 {
		operationCtx, cancelOperation = context.WithTimeout(ctx, d.client.Timeout)
	}
	defer cancelOperation()
	request, err := http.NewRequestWithContext(operationCtx, http.MethodPost, "http://docker/"+d.apiVersion+"/exec/"+url.PathEscape(created.ID)+"/start", bytes.NewReader(startBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "tcp")
	transport := d.client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	// http.Client.Timeout wraps a 101 response body and hides Write/CloseWrite. Use
	// the transport directly, then enforce the same total timeout through the context.
	response, err := transport.RoundTrip(request)
	if err != nil {
		if ctxErr := operationCtx.Err(); ctxErr != nil {
			return fmt.Errorf("start docker stdin exec: %w", ctxErr)
		}
		return fmt.Errorf("start docker stdin exec: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("start docker stdin exec: unexpected status %s", response.Status)
	}
	stream, ok := response.Body.(interface {
		io.Reader
		io.Writer
		CloseWrite() error
	})
	if !ok {
		return fmt.Errorf("start docker stdin exec: upgraded stream is not writable")
	}
	stopCloseOnCancel := context.AfterFunc(operationCtx, func() {
		_ = response.Body.Close()
	})
	defer stopCloseOnCancel()
	if _, err := io.Copy(stream, bytes.NewReader(input)); err != nil {
		if ctxErr := operationCtx.Err(); ctxErr != nil {
			return fmt.Errorf("write docker exec stdin: %w", ctxErr)
		}
		return fmt.Errorf("write docker exec stdin failed")
	}
	if err := stream.CloseWrite(); err != nil {
		if ctxErr := operationCtx.Err(); ctxErr != nil {
			return fmt.Errorf("close docker exec stdin: %w", ctxErr)
		}
		return fmt.Errorf("close docker exec stdin failed")
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		if ctxErr := operationCtx.Err(); ctxErr != nil {
			return fmt.Errorf("wait for docker stdin exec: %w", ctxErr)
		}
		return fmt.Errorf("wait for docker stdin exec failed")
	}
	stopCloseOnCancel()
	var state struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	}
	inspectCtx, cancelInspect := context.WithTimeout(operationCtx, dockerExecInspectTimeout)
	defer cancelInspect()
	if err := wait.PollImmediateUntil(dockerExecInspectInterval, func() (bool, error) {
		if err := d.request(inspectCtx, http.MethodGet, "/exec/"+url.PathEscape(created.ID)+"/json", nil, &state); err != nil {
			return false, fmt.Errorf("inspect docker stdin exec: %w", err)
		}
		return !state.Running, nil
	}, inspectCtx.Done()); err != nil {
		if ctxErr := operationCtx.Err(); ctxErr != nil {
			return fmt.Errorf("wait for docker stdin exec state: %w", ctxErr)
		}
		if inspectCtx.Err() != nil {
			return fmt.Errorf("docker stdin exec did not exit within %s", dockerExecInspectTimeout)
		}
		return err
	}
	if state.ExitCode != 0 {
		return fmt.Errorf("docker stdin exec exited with code %d", state.ExitCode)
	}
	return nil
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

func (d *DockerProvider) Health(ctx context.Context, ref string) (*iapiserver.InfraRuntimeHealthResult, error) {
	checked := imachinery.Now()
	result, err := d.Inspect(ctx, ref)
	if err != nil {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnknown, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonInfrastructureUnavailable}, nil
	}
	if result.Status != iapiserver.InfraRuntimeStatusRunning || result.Endpoint == nil {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnhealthy, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonNotRunning}, nil
	}
	profileID := ""
	if inspected, inspectErr := d.inspectContainer(ctx, ref); inspectErr == nil {
		profileID = inspected.Config.Labels["io.omnimam.profile"]
	}
	profile, ok := agentRuntimeServiceProfile(profileID)
	if !ok {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnknown, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonProbeIndeterminate}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.Endpoint.BaseURL+profile.healthPath, nil)
	if err != nil {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnknown, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonProbeIndeterminate}, nil
	}
	response, err := d.runtimeHTTP.Do(req)
	if err != nil {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnhealthy, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonServiceUnreachable}, nil
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnhealthy, CheckedAt: checked, Reason: iapiserver.AgentRuntimeHealthReasonServiceUnhealthy}, nil
	}
	return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthHealthy, CheckedAt: checked}, nil
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
while [ ! -f /run/omnimam/start ]; do sleep 0.1; done
printf '#!/bin/sh\nexec nc 127.0.0.1 4096\n' > /run/omnimam/forward
chmod 500 /run/omnimam/forward
nc -lk -s 0.0.0.0 -p 14096 -e /run/omnimam/forward &
forwarder_pid=$!
for attempt in $(seq 1 100); do
  if awk '$2 ~ /:3710$/ && $4 == "0A" { found=1 } END { exit found ? 0 : 1 }' /proc/net/tcp /proc/net/tcp6; then
    break
  fi
  if ! kill -0 "$forwarder_pid" 2>/dev/null; then
    exit 1
  fi
  sleep 0.01
done
if ! awk '$2 ~ /:3710$/ && $4 == "0A" { found=1 } END { exit found ? 0 : 1 }' /proc/net/tcp /proc/net/tcp6; then
  exit 1
fi
exec opencode serve --hostname 127.0.0.1 --port 4096`,
			tmpfs: map[string]string{
				"/tmp":                        "rw,nosuid,nodev,noexec,size=64m",
				"/run/omnimam":                "rw,nosuid,nodev,exec,size=1m",
				"/root/.cache/opencode":       "rw,nosuid,nodev,noexec,size=64m",
				"/root/.config/opencode":      "rw,nosuid,nodev,noexec,size=32m",
				"/root/.local/share/opencode": "rw,nosuid,nodev,noexec,size=128m",
				"/root/.local/state/opencode": "rw,nosuid,nodev,noexec,size=16m",
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

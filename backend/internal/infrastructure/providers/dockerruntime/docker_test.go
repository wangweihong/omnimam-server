package dockerruntime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure/providers"
)

func TestReadRuntimeCA(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(tlsServer.Close)
	validPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsServer.TLS.Certificates[0].Certificate[0]})
	validPath := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(validPath, validPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid.crt")
	if err := os.WriteFile(invalidPath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantLen int
		wantErr string
	}{
		{name: "not configured"},
		{name: "valid PEM bundle", path: validPath, wantLen: len(validPEM)},
		{name: "invalid PEM bundle", path: invalidPath, wantErr: "contains no PEM certificates"},
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.crt"), wantErr: "read coding runtime MCP CA bundle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readRuntimeCA(tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("readRuntimeCA() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readRuntimeCA() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("readRuntimeCA() length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestCodingRuntimeListenerKeepsStableListeningSocket(t *testing.T) {
	profile, ok := agentRuntimeServiceProfile(iapiserver.InfraRuntimeProfileIDAgentCoding)
	if !ok {
		t.Fatal("agentRuntimeServiceProfile() did not return Coding Runtime profile")
	}
	if !strings.Contains(profile.command, "printf '#!/bin/sh\\nexec nc 127.0.0.1 4096\\n'") {
		t.Fatalf("Coding Runtime forwarder does not preserve long-running responses: %q", profile.command)
	}
	if strings.Contains(profile.command, "nc -w 1 127.0.0.1 4096") {
		t.Fatalf("Coding Runtime forwarder truncates idle model streams: %q", profile.command)
	}
	if !strings.Contains(profile.command, "nc -lk -s 0.0.0.0 -p 14096 -e /run/omnimam/forward &\nforwarder_pid=$!\nfor attempt in $(seq 1 100); do") {
		t.Fatalf("Coding Runtime listener does not keep a stable listening socket: %q", profile.command)
	}
	if strings.Contains(profile.command, "while :; do\n  if ! nc -l") {
		t.Fatalf("Coding Runtime listener rebinds after each connection: %q", profile.command)
	}
	if !strings.Contains(profile.command, `awk '$2 ~ /:3710$/ && $4 == "0A"`) {
		t.Fatalf("Coding Runtime must wait for the forwarder listener before starting OpenCode: %q", profile.command)
	}
}

func TestEnsurePinsCodingRuntimeCABundle(t *testing.T) {
	createBody := make(chan []byte, 1)
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/create":
			createBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "container-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/start":
			http.Error(response, "startup stopped for environment inspection", http.StatusInternalServerError)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1.44/containers/container-1":
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	provider.images = MapProfileImages{"agent.coding@1.0": "coding-image:test"}
	provider.runtimeCA = []byte("public CA bundle")

	_, err := provider.Ensure(t.Context(), providers.ProviderRequest{
		RuntimeID: "runtime-1",
		Profile: &iapiserver.InfraRuntimeProfile{
			ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraRuntimeProfileIDAgentCoding},
			Revision:   "1.0",
		},
		Request: &iapiserver.InfraCreateRuntimeRequest{
			RuntimeMode: iapiserver.InfraRuntimeModeService,
			ConfigurationBindings: []iapiserver.InfraRuntimeConfigBindingInput{{
				Name: "NODE_EXTRA_CA_CERTS", BindingType: iapiserver.InfraConfigBindingTypePlainConfig, Reference: "/untrusted/override.crt",
			}},
		},
		RuntimeGitAccess: &providers.RuntimeGitAccess{CloneURL: "https://gitlab.example.test/group/project.git", Username: "project_bot", Token: "runtime-token"},
	})
	if err == nil {
		t.Fatal("Ensure() error = nil, want controlled startup failure")
	}
	var request struct {
		Env []string `json:"Env"`
	}
	if err := json.Unmarshal(<-createBody, &request); err != nil {
		t.Fatal(err)
	}
	want := "NODE_EXTRA_CA_CERTS=" + openCodeRuntimeCAPath
	if len(request.Env) != 2 || request.Env[1] != want || request.Env[0] != "OMNIMAM_BINDING_NODE_EXTRA_CA_CERTS=/untrusted/override.crt" {
		t.Fatalf("coding runtime environment = %q, want a namespaced binding followed by %q", request.Env, want)
	}
}

func TestEnsureConfiguresWritableNginxPreviewPaths(t *testing.T) {
	createBody := make(chan []byte, 1)
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/create":
			createBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "container-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/start":
			http.Error(response, "startup stopped for environment inspection", http.StatusInternalServerError)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1.44/containers/container-1":
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	provider.images = MapProfileImages{"appstudio.preview.static-web@1.0": "nginx:test"}

	archive, digest := previewSourceArchive(t)
	_, err := provider.Ensure(t.Context(), providers.ProviderRequest{
		RuntimeID: "runtime-1",
		Profile: &iapiserver.InfraRuntimeProfile{
			ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraRuntimeProfileIDAppStudioPreviewWeb},
			Revision:   "1.0",
		},
		Request: &iapiserver.InfraCreateRuntimeRequest{
			RuntimeMode:      iapiserver.InfraRuntimeModeService,
			OwnerReference:   "preview-1",
			SourceRef:        iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-1/7",
			AuthorizationRef: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace-1/preview-1/3",
			Mounts: []iapiserver.InfraRuntimeMountInput{{
				SourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-1/7", TargetPath: iapiserver.InfraRuntimeMountTargetAppStudioStaticWebSource,
				ReadOnly: true, MountKind: iapiserver.InfraMountKindStudioWorkspaceRevision,
			}},
		},
		SourceArchive: archive, SourceContentDigest: digest,
	})
	if err == nil {
		t.Fatal("Ensure() error = nil, want controlled startup failure")
	}
	var request struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
		HostConfig struct {
			ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
			Tmpfs          map[string]string `json:"Tmpfs"`
			CapAdd         []string          `json:"CapAdd"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(<-createBody, &request); err != nil {
		t.Fatal(err)
	}
	if !request.HostConfig.ReadonlyRootfs {
		t.Fatal("Nginx Preview container rootfs is not read-only")
	}
	for _, path := range []string{"/var/cache/nginx", "/var/run", previewContentRoot} {
		if _, ok := request.HostConfig.Tmpfs[path]; !ok {
			t.Fatalf("Nginx Preview container is missing writable %s tmpfs", path)
		}
	}
	if fmt.Sprint(request.Entrypoint) != fmt.Sprint([]string{"/bin/sh", "-c"}) || len(request.Cmd) != 1 || request.Cmd[0] != previewRuntimeCommand {
		t.Fatalf("Nginx Preview startup command = entrypoint=%q cmd=%q", request.Entrypoint, request.Cmd)
	}
	wantCapabilities := []string{"CHOWN", "SETGID", "SETUID"}
	if fmt.Sprint(request.HostConfig.CapAdd) != fmt.Sprint(wantCapabilities) {
		t.Fatalf("Nginx Preview capabilities = %q, want %q", request.HostConfig.CapAdd, wantCapabilities)
	}
}

func previewSourceArchive(t *testing.T) ([]byte, string) {
	t.Helper()
	content := []byte("<main>preview</main>\n")
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "project/index.html", Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fileDigest := sha256.Sum256(content)
	manifest := sha256.Sum256([]byte("index.html\x00sha256:" + hex.EncodeToString(fileDigest[:]) + "\x00" + fmt.Sprint(len(content)) + "\n"))
	return compressed.Bytes(), "sha256:" + hex.EncodeToString(manifest[:])
}

func TestEnsureBuildInjectsArchiveAndCollectsBundle(t *testing.T) {
	createBody := make(chan []byte, 1)
	execBody := make(chan []byte, 1)
	archiveInput := make(chan []byte, 1)
	bundle := []byte("bundle-content")
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/create":
			createBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "container-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/start":
			response.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/exec":
			execBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "exec-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/exec/exec-1/start":
			hijackDockerTestStream(t, response, archiveInput)
		case request.Method == http.MethodGet && request.URL.Path == "/v1.44/exec/exec-1/json":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"Running": false, "ExitCode": 0})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/wait":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"StatusCode": 0})
		case request.Method == http.MethodGet && request.URL.Path == "/v1.44/containers/container-1/archive":
			if request.URL.Query().Get("path") != "/output/bundle.tar.gz" {
				t.Errorf("output archive path = %q", request.URL.Query().Get("path"))
			}
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(dockerTestTar(t, "bundle.tar.gz", bundle))
		default:
			http.NotFound(response, request)
		}
	}))
	provider.images = MapProfileImages{"appstudio.build.static-web@1.0": "node:test"}
	source, digest := previewSourceArchive(t)
	result, err := provider.Ensure(t.Context(), providers.ProviderRequest{
		RuntimeID: "runtime-build-1",
		Profile:   &iapiserver.InfraRuntimeProfile{ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraRuntimeProfileIDAppStudioBuildWeb}, Revision: "1.0"},
		Request: &iapiserver.InfraCreateRuntimeRequest{
			RuntimeMode: iapiserver.InfraRuntimeModeJob, SourceRef: iapiserver.InfraRefPrefixStudioSnapshot + "snapshot-1",
			OutputDeclarations: []iapiserver.InfraRuntimeOutputDeclaration{{OutputKey: "bundle", RelativePath: "bundle.tar.gz", MediaType: "application/gzip"}},
		},
		SourceArchive: source, SourceContentDigest: digest,
	})
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if result.Status != iapiserver.InfraRuntimeStatusSucceeded || len(result.Outputs) != 1 || result.Outputs[0].OutputKey != "bundle" {
		t.Fatalf("Ensure() result = %#v", result)
	}
	output, ok := result.OutputContents["bundle"]
	if !ok || output.SizeBytes != int64(len(bundle)) || output.Open == nil {
		t.Fatalf("collected output = %#v", output)
	}
	reader, err := output.Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	collected, err := io.ReadAll(reader)
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil || !bytes.Equal(collected, bundle) {
		t.Fatalf("collected output = %q, error = %v", collected, err)
	}
	var create struct {
		Cmd        []string `json:"Cmd"`
		HostConfig struct {
			Tmpfs map[string]string `json:"Tmpfs"`
		} `json:"HostConfig"`
	}
	createRaw := <-createBody
	if err := json.Unmarshal(createRaw, &create); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/workspace", "/output", "/run/omnimam"} {
		if _, ok := create.HostConfig.Tmpfs[target]; !ok {
			t.Fatalf("Build container is missing tmpfs %s", target)
		}
	}
	if len(create.Cmd) != 1 || !strings.Contains(create.Cmd[0], "while [ ! -f /run/omnimam/source-ready ]") || !strings.Contains(create.Cmd[0], "corepack pnpm build") {
		t.Fatalf("Build command = %q", create.Cmd)
	}
	if bytes.Contains(<-execBody, source) || bytes.Contains(createRaw, source) {
		t.Fatal("source archive leaked into Docker metadata")
	}
	if got := <-archiveInput; !bytes.Equal(got, source) {
		t.Fatal("Build source archive stdin does not match the resolved archive")
	}
}

func dockerTestTar(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestPreviewSourceSubpathRejectsCrossScopeAndTraversal(t *testing.T) {
	tests := []struct {
		name          string
		sourceRef     string
		authorization string
		owner         string
		want          string
		wantErr       bool
	}{
		{name: "matching scope", sourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-1/7", authorization: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace-1/preview-1/3", owner: "preview-1", want: "workspace-1/7"},
		{name: "different workspace", sourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-2/7", authorization: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace-1/preview-1/3", owner: "preview-1", wantErr: true},
		{name: "different Preview owner", sourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-1/7", authorization: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace-1/preview-2/3", owner: "preview-1", wantErr: true},
		{name: "encoded path segment", sourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace%2Fother/7", authorization: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace%2Fother/preview-1/3", owner: "preview-1", wantErr: true},
		{name: "negative revision", sourceRef: iapiserver.InfraRefPrefixStudioWorkspaceRevision + "workspace-1/-1", authorization: iapiserver.AppStudioRefPrefixPreviewGrant + "workspace-1/preview-1/3", owner: "preview-1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := previewSourceSubpath(tt.sourceRef, tt.authorization, tt.owner)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("previewSourceSubpath() = %q, want error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("previewSourceSubpath() = (%q, %v), want (%q, nil)", got, err, tt.want)
			}
		})
	}
}

func TestInjectOpenCodeConfigStreamsSensitiveConfigThroughExecStdin(t *testing.T) {
	const credential = "credential-must-only-travel-through-stdin"
	execCreateBody := make(chan []byte, 1)
	execStartBody := make(chan []byte, 1)
	execInput := make(chan []byte, 1)

	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1.44/containers/runtime-1/exec":
			execCreateBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "exec-1"})
		case "/v1.44/exec/exec-1/start":
			execStartBody <- readDockerTestBody(t, request)
			if request.Header.Get("Connection") != "Upgrade" || request.Header.Get("Upgrade") != "tcp" {
				t.Errorf("exec start upgrade headers = %q/%q", request.Header.Get("Connection"), request.Header.Get("Upgrade"))
			}
			hijackDockerTestStream(t, response, execInput)
		case "/v1.44/exec/exec-1/json":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"Running": false, "ExitCode": 0})
		default:
			http.NotFound(response, request)
		}
	}))

	err := provider.injectOpenCodeConfig(t.Context(), "runtime-1", []providers.ResolvedMCPBinding{{
		ServerKey:     "platform",
		Endpoint:      "http://mcp.internal",
		Credential:    credential,
		AllowedTools:  []string{"read_asset"},
		Configuration: map[string]any{"timeout": 30},
	}})
	if err != nil {
		t.Fatalf("injectOpenCodeConfig() error = %v", err)
	}

	createBody := <-execCreateBody
	startBody := <-execStartBody
	input := <-execInput
	if bytes.Contains(createBody, []byte(credential)) || bytes.Contains(startBody, []byte(credential)) ||
		bytes.Contains(createBody, input) || bytes.Contains(startBody, input) {
		t.Fatal("OpenCode configuration leaked into Docker exec metadata")
	}
	if !bytes.Contains(input, []byte("Bearer "+credential)) {
		t.Fatal("Docker exec stdin does not contain the resolved credential")
	}
	var createRequest struct {
		AttachStdin  bool     `json:"AttachStdin"`
		AttachStdout bool     `json:"AttachStdout"`
		AttachStderr bool     `json:"AttachStderr"`
		Tty          bool     `json:"Tty"`
		Privileged   bool     `json:"Privileged"`
		Cmd          []string `json:"Cmd"`
		Env          []string `json:"Env"`
	}
	if err := json.Unmarshal(createBody, &createRequest); err != nil {
		t.Fatalf("decode Docker exec create request: %v", err)
	}
	if !createRequest.AttachStdin || createRequest.AttachStdout || createRequest.AttachStderr || createRequest.Tty || createRequest.Privileged {
		t.Fatalf("Docker exec attachment/security flags = %+v", createRequest)
	}
	if len(createRequest.Env) != 0 {
		t.Fatalf("Docker exec environment = %q, want empty", createRequest.Env)
	}
	wantCommand := []string{"/bin/sh", "-c", openCodeConfigInstallCommand}
	if fmt.Sprint(createRequest.Cmd) != fmt.Sprint(wantCommand) {
		t.Fatalf("Docker exec command = %q, want %q", createRequest.Cmd, wantCommand)
	}
	var startRequest struct {
		Detach bool `json:"Detach"`
		Tty    bool `json:"Tty"`
	}
	if err := json.Unmarshal(startBody, &startRequest); err != nil {
		t.Fatalf("decode Docker exec start request: %v", err)
	}
	if startRequest.Detach || startRequest.Tty {
		t.Fatalf("Docker exec start flags = %+v, want interactive non-TTY", startRequest)
	}
	var config map[string]any
	if err := json.Unmarshal(input, &config); err != nil {
		t.Fatalf("Docker exec stdin is not valid JSON: %v", err)
	}
}

func TestExecWithInputPreservesWritableUpgradeWhenDockerClientHasTimeout(t *testing.T) {
	execInput := make(chan []byte, 1)
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1.44/containers/runtime-1/exec":
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "exec-1"})
		case "/v1.44/exec/exec-1/start":
			_ = readDockerTestBody(t, request)
			hijackDockerTestStream(t, response, execInput)
		case "/v1.44/exec/exec-1/json":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"Running": false, "ExitCode": 0})
		default:
			http.NotFound(response, request)
		}
	}))
	provider.client.Timeout = 2 * time.Minute

	const input = "timeout-wrapped-upgrade-input"
	if err := provider.execWithInput(t.Context(), "runtime-1", []string{"fixed-command"}, []byte(input)); err != nil {
		t.Fatalf("execWithInput() error = %v", err)
	}
	if got := <-execInput; string(got) != input {
		t.Fatalf("Docker exec stdin = %q, want %q", got, input)
	}
}

func TestExecWithInputHonorsDockerClientTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const sensitiveInput = "timed-out-sensitive-input"
		closed := make(chan struct{})
		var closeOnce sync.Once
		stream := &dockerTestStream{
			writeFunc:      func(value []byte) (int, error) { return len(value), nil },
			closeWriteFunc: func() error { return nil },
			readFunc: func([]byte) (int, error) {
				<-closed
				return 0, net.ErrClosed
			},
			closeFunc: func() error {
				closeOnce.Do(func() { close(closed) })
				return nil
			},
		}
		provider := newDockerStreamTestProvider(stream, nil)
		provider.client.Timeout = 2 * time.Minute

		err := provider.execWithInput(context.Background(), "runtime-1", []string{"fixed-command"}, []byte(sensitiveInput))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("execWithInput() error = %v, want client deadline", err)
		}
		assertDockerTestErrorDoesNotContain(t, err, sensitiveInput)
		select {
		case <-closed:
		default:
			t.Fatal("execWithInput() did not close the timed-out upgraded stream")
		}
	})
}

func TestExecWithInputRequiresSwitchingProtocolsWithoutLeakingInput(t *testing.T) {
	const sensitiveInput = "non-101-sensitive-input"
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1.44/containers/runtime-1/exec":
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "exec-1"})
		case "/v1.44/exec/exec-1/start":
			http.Error(response, sensitiveInput, http.StatusBadRequest)
		default:
			http.NotFound(response, request)
		}
	}))

	err := provider.execWithInput(t.Context(), "runtime-1", []string{"fixed-command"}, []byte(sensitiveInput))
	if err == nil || !strings.Contains(err.Error(), "unexpected status 400 Bad Request") {
		t.Fatalf("execWithInput() error = %v, want non-101 status", err)
	}
	assertDockerTestErrorDoesNotContain(t, err, sensitiveInput)
}

func TestExecWithInputRejectsNonWritableUpgradeStream(t *testing.T) {
	const sensitiveInput = "non-writable-sensitive-input"
	provider := newDockerStreamTestProvider(io.NopCloser(strings.NewReader("")), nil)

	err := provider.execWithInput(t.Context(), "runtime-1", []string{"fixed-command"}, []byte(sensitiveInput))
	if err == nil || !strings.Contains(err.Error(), "upgraded stream is not writable") {
		t.Fatalf("execWithInput() error = %v, want non-writable stream error", err)
	}
	assertDockerTestErrorDoesNotContain(t, err, sensitiveInput)
}

func TestExecWithInputDoesNotLeakInputFromStreamFailures(t *testing.T) {
	const sensitiveInput = "stream-failure-sensitive-input"
	tests := []struct {
		name   string
		stream *dockerTestStream
		want   string
	}{
		{
			name: "write failure",
			stream: &dockerTestStream{
				readFunc: func([]byte) (int, error) { return 0, io.EOF },
				writeFunc: func(value []byte) (int, error) {
					return 0, fmt.Errorf("write rejected %s", value)
				},
			},
			want: "write docker exec stdin",
		},
		{
			name: "close-write failure",
			stream: &dockerTestStream{
				readFunc:  func([]byte) (int, error) { return 0, io.EOF },
				writeFunc: func(value []byte) (int, error) { return len(value), nil },
				closeWriteFunc: func() error {
					return fmt.Errorf("close rejected %s", sensitiveInput)
				},
			},
			want: "close docker exec stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newDockerStreamTestProvider(tt.stream, nil)
			err := provider.execWithInput(t.Context(), "runtime-1", []string{"fixed-command"}, []byte(sensitiveInput))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("execWithInput() error = %v, want %q", err, tt.want)
			}
			assertDockerTestErrorDoesNotContain(t, err, sensitiveInput)
		})
	}
}

func TestExecWithInputClosesUpgradeStreamOnContextCancellation(t *testing.T) {
	const sensitiveInput = "canceled-sensitive-input"
	ctx, cancel := context.WithCancel(t.Context())
	closed := make(chan struct{})
	var closeOnce sync.Once
	stream := &dockerTestStream{
		writeFunc: func(value []byte) (int, error) { return len(value), nil },
		closeWriteFunc: func() error {
			cancel()
			return nil
		},
		readFunc: func([]byte) (int, error) {
			select {
			case <-closed:
				return 0, net.ErrClosed
			case <-time.After(250 * time.Millisecond):
				return 0, errors.New("upgrade stream remained open after cancellation")
			}
		},
		closeFunc: func() error {
			closeOnce.Do(func() { close(closed) })
			return nil
		},
	}
	provider := newDockerStreamTestProvider(stream, nil)

	err := provider.execWithInput(ctx, "runtime-1", []string{"fixed-command"}, []byte(sensitiveInput))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("execWithInput() error = %v, want context cancellation", err)
	}
	assertDockerTestErrorDoesNotContain(t, err, sensitiveInput)
	select {
	case <-closed:
	default:
		t.Fatal("execWithInput() did not close the upgraded stream")
	}
}

func TestExecWithInputReportsNonzeroExitWithoutLeakingInput(t *testing.T) {
	const sensitiveInput = "sensitive-input-marker"
	execInput := make(chan []byte, 1)
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1.44/containers/runtime-1/exec":
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "exec-1"})
		case "/v1.44/exec/exec-1/start":
			_ = readDockerTestBody(t, request)
			hijackDockerTestStream(t, response, execInput)
		case "/v1.44/exec/exec-1/json":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"Running": false, "ExitCode": 23})
		default:
			http.NotFound(response, request)
		}
	}))

	err := provider.execWithInput(t.Context(), "runtime-1", []string{"/bin/sh", "-c", openCodeConfigInstallCommand}, []byte(sensitiveInput))
	if err == nil || !strings.Contains(err.Error(), "exited with code 23") {
		t.Fatalf("execWithInput() error = %v, want exit code 23", err)
	}
	if strings.Contains(err.Error(), sensitiveInput) {
		t.Fatal("execWithInput() leaked stdin in its error")
	}
	if input := <-execInput; string(input) != sensitiveInput {
		t.Fatalf("Docker exec stdin = %q, want exact sensitive input", input)
	}
}

func TestExecWithInputWaitsForTerminalStateAfterStreamCompletion(t *testing.T) {
	provider := newDockerStreamTestProvider(&dockerTestStream{}, nil)
	baseTransport := provider.client.Transport
	inspectCalls := 0
	provider.client.Transport = dockerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/v1.44/exec/exec-1/json" {
			inspectCalls++
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(`{"Running":%t,"ExitCode":0}`, inspectCalls == 1)), nil
		}
		return baseTransport.RoundTrip(request)
	})

	if err := provider.execWithInput(t.Context(), "runtime-1", []string{"fixed-command"}, []byte("input")); err != nil {
		t.Fatalf("execWithInput() error = %v", err)
	}
	if inspectCalls != 2 {
		t.Fatalf("Docker exec inspect calls = %d, want 2", inspectCalls)
	}
}

func TestEnsureDeletesContainerWhenConfigInjectionFails(t *testing.T) {
	const credential = "cleanup-path-credential"
	const runtimeGitToken = "runtime-git-token"
	deleted := make(chan string, 1)
	containerCreateBody := make(chan []byte, 1)
	execCreateBody := make(chan []byte, 2)
	gitExecInput := make(chan []byte, 1)
	execCount := 0
	provider := newDockerTestProvider(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/create":
			containerCreateBody <- readDockerTestBody(t, request)
			writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "container-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/start":
			response.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/containers/container-1/exec":
			execCount++
			execCreateBody <- readDockerTestBody(t, request)
			if execCount == 1 {
				writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "git-exec"})
			} else {
				writeDockerTestJSON(t, response, http.StatusCreated, map[string]any{"Id": "config-exec"})
			}
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/exec/git-exec/start":
			_ = readDockerTestBody(t, request)
			hijackDockerTestStream(t, response, gitExecInput)
		case request.Method == http.MethodGet && request.URL.Path == "/v1.44/exec/git-exec/json":
			writeDockerTestJSON(t, response, http.StatusOK, map[string]any{"Running": false, "ExitCode": 0})
		case request.Method == http.MethodPost && request.URL.Path == "/v1.44/exec/config-exec/start":
			_ = readDockerTestBody(t, request)
			http.Error(response, "upgrade unavailable", http.StatusBadRequest)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1.44/containers/container-1":
			deleted <- request.URL.RawQuery
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	provider.images = MapProfileImages{"agent.coding@1.0": "coding-image:test"}
	healthChecks := 0
	provider.runtimeHTTP = &http.Client{Transport: dockerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		healthChecks++
		return dockerTestResponse(http.StatusInternalServerError, ""), nil
	})}

	result, err := provider.Ensure(t.Context(), providers.ProviderRequest{
		RuntimeID: "runtime-1",
		Profile: &iapiserver.InfraRuntimeProfile{
			ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraRuntimeProfileIDAgentCoding},
			Revision:   "1.0",
		},
		Request: &iapiserver.InfraCreateRuntimeRequest{RuntimeMode: iapiserver.InfraRuntimeModeService},
		MCPBindings: []providers.ResolvedMCPBinding{{
			ServerKey:     "platform",
			Endpoint:      "http://mcp.internal",
			Credential:    credential,
			Configuration: map[string]any{"timeout": 30},
		}},
		RuntimeGitAccess: &providers.RuntimeGitAccess{CloneURL: "https://gitlab.example.test/group/project.git", Username: "project_bot", Token: runtimeGitToken},
	})
	if err == nil || result != nil {
		t.Fatalf("Ensure() = (%v, %v), want nil result and config injection error", result, err)
	}
	if strings.Contains(err.Error(), credential) || strings.Contains(err.Error(), runtimeGitToken) {
		t.Fatal("Ensure() leaked a resolved credential in its error")
	}
	createBody := <-containerCreateBody
	if bytes.Contains(createBody, []byte(credential)) {
		t.Fatal("credential leaked into Docker container metadata")
	}
	var createRequest struct {
		Cmd        []string `json:"Cmd"`
		Env        []string `json:"Env"`
		HostConfig struct {
			ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
			Tmpfs          map[string]string `json:"Tmpfs"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(createBody, &createRequest); err != nil {
		t.Fatalf("decode Docker container create request: %v", err)
	}
	if !createRequest.HostConfig.ReadonlyRootfs {
		t.Fatal("Docker container rootfs is not read-only")
	}
	if _, ok := createRequest.HostConfig.Tmpfs["/root/.config/opencode"]; !ok {
		t.Fatal("Docker container is missing writable OpenCode config tmpfs")
	}
	if _, ok := createRequest.HostConfig.Tmpfs["/root/.local/state/opencode"]; !ok {
		t.Fatal("Docker container is missing writable OpenCode state tmpfs")
	}
	if _, ok := createRequest.HostConfig.Tmpfs["/run/omnimam"]; !ok {
		t.Fatal("Docker container is missing writable startup-gate tmpfs")
	}
	if _, ok := createRequest.HostConfig.Tmpfs["/workspace"]; !ok {
		t.Fatal("Docker container is missing disposable workspace tmpfs")
	}
	if len(createRequest.Cmd) != 1 || !strings.Contains(createRequest.Cmd[0], "[ ! -f /run/omnimam/start ] || [ ! -f /run/omnimam/git-ready ]") {
		t.Fatalf("Docker container command does not preserve the startup gate: %q", createRequest.Cmd)
	}
	for i := 0; i < 2; i++ {
		execBody := <-execCreateBody
		if bytes.Contains(execBody, []byte(credential)) || bytes.Contains(execBody, []byte(runtimeGitToken)) {
			t.Fatal("credential leaked into Docker exec create metadata")
		}
	}
	if input := <-gitExecInput; !bytes.Contains(input, []byte(runtimeGitToken)) {
		t.Fatal("runtime Git token was not sent through Docker exec stdin")
	}
	if query := <-deleted; query != "force=true&v=true" {
		t.Fatalf("Delete container query = %q, want force=true&v=true", query)
	}
	if healthChecks != 0 {
		t.Fatalf("Ensure() made %d health checks after config injection failed", healthChecks)
	}
}

func newDockerTestProvider(t *testing.T, handler http.Handler) *DockerProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	address := strings.TrimPrefix(server.URL, "http://")
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &DockerProvider{client: &http.Client{Transport: transport}, apiVersion: "v1.44", networkMode: "bridge"}
}

func newDockerStreamTestProvider(stream io.ReadCloser, inspectState map[string]any) *DockerProvider {
	return &DockerProvider{
		client: &http.Client{Transport: dockerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.Path {
			case "/v1.44/containers/runtime-1/exec":
				return dockerTestResponse(http.StatusCreated, `{"Id":"exec-1"}`), nil
			case "/v1.44/exec/exec-1/start":
				return &http.Response{
					StatusCode: http.StatusSwitchingProtocols,
					Status:     "101 Switching Protocols",
					Header:     make(http.Header),
					Body:       stream,
					Request:    request,
				}, nil
			case "/v1.44/exec/exec-1/json":
				if inspectState == nil {
					inspectState = map[string]any{"Running": false, "ExitCode": 0}
				}
				raw, err := json.Marshal(inspectState)
				if err != nil {
					return nil, err
				}
				return dockerTestResponse(http.StatusOK, string(raw)), nil
			default:
				return dockerTestResponse(http.StatusNotFound, ""), nil
			}
		})},
		apiVersion:  "v1.44",
		networkMode: "bridge",
	}
}

type dockerRoundTripFunc func(*http.Request) (*http.Response, error)

func (function dockerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type dockerTestStream struct {
	readFunc       func([]byte) (int, error)
	writeFunc      func([]byte) (int, error)
	closeWriteFunc func() error
	closeFunc      func() error
}

func (stream *dockerTestStream) Read(value []byte) (int, error) {
	if stream.readFunc == nil {
		return 0, io.EOF
	}
	return stream.readFunc(value)
}

func (stream *dockerTestStream) Write(value []byte) (int, error) {
	if stream.writeFunc == nil {
		return len(value), nil
	}
	return stream.writeFunc(value)
}

func (stream *dockerTestStream) CloseWrite() error {
	if stream.closeWriteFunc == nil {
		return nil
	}
	return stream.closeWriteFunc()
}

func (stream *dockerTestStream) Close() error {
	if stream.closeFunc == nil {
		return nil
	}
	return stream.closeFunc()
}

func dockerTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertDockerTestErrorDoesNotContain(t *testing.T, err error, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(err.Error(), value) {
			t.Fatalf("error leaked sensitive value %q: %v", value, err)
		}
	}
}

func hijackDockerTestStream(t *testing.T, response http.ResponseWriter, input chan<- []byte) {
	t.Helper()
	connection, buffer, err := response.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("Hijack() error = %v", err)
		return
	}
	defer connection.Close()
	if _, err := buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"); err != nil {
		t.Errorf("write upgrade response: %v", err)
		return
	}
	if err := buffer.Flush(); err != nil {
		t.Errorf("flush upgrade response: %v", err)
		return
	}
	payload, err := io.ReadAll(connection)
	if err != nil {
		t.Errorf("read upgraded stdin: %v", err)
		return
	}
	input <- payload
}

func readDockerTestBody(t *testing.T, request *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Errorf("read request body: %v", err)
	}
	return body
}

func writeDockerTestJSON(t *testing.T, response http.ResponseWriter, status int, value any) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

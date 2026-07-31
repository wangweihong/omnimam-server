package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	comfyuiadapter "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/comfyui"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestOpenAIAdapterExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request: path=%s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "provider-model-v1" {
			t.Errorf("provider model was not resolved: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	executor := NewOperationExecutors()["deepseek_chat_completions"]
	output, err := executor.Execute(context.Background(), testEngine(server.URL, "deepseek_official"), &iapiserver.ApplicationRun{
		InputSnapshot:            map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}, "model_id": "internal-model"},
		CapabilitySourceSnapshot: map[string]any{"provider_capability": map[string]any{"models": []any{map[string]any{"id": "internal-model", "provider_model_id": "provider-model-v1"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	values, _ := output["values"].(map[string]any)
	if values["id"] != "chat-1" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestApplyComfyInputsUsesWorkflowCopy(t *testing.T) {
	workflow := map[string]any{"6": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "old"}}}
	contract := map[string]any{"request_mapping": map[string]any{
		"prompt": map[string]any{"node_id": "6", "input_name": "text"},
		"seed":   "6.inputs.seed",
	}}
	resolved, err := comfyuiadapter.ApplyInputs(workflow, map[string]any{"prompt": "new", "seed": 42}, contract)
	if err != nil {
		t.Fatal(err)
	}
	inputs := typeutil.As[map[string]any](typeutil.As[map[string]any](resolved["6"])["inputs"])
	if inputs["text"] != "new" || inputs["seed"] != 42 {
		t.Fatalf("workflow inputs were not resolved: %#v", resolved)
	}
	originalInputs := typeutil.As[map[string]any](typeutil.As[map[string]any](workflow["6"])["inputs"])
	if originalInputs["text"] != "old" {
		t.Fatalf("immutable workflow snapshot was mutated: %#v", workflow)
	}
}

func TestApplyComfyInputsSupportsImportedTemplateContract(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"seed": float64(1)}}}
	contract := map[string]any{"fixed_parameters": []any{}, "parameter_mappings": []any{map[string]any{"input_key": "seed", "conversion_type": "DIRECT", "targets": []any{map[string]any{"node_id": "1", "input_name": "seed"}}}}}
	resolved, err := comfyuiadapter.ApplyInputs(workflow, map[string]any{"seed": float64(42)}, contract)
	if err != nil {
		t.Fatal(err)
	}
	seed := typeutil.As[map[string]any](typeutil.As[map[string]any](resolved["1"])["inputs"])["seed"]
	if seed != float64(42) {
		t.Fatalf("seed=%v, want 42", seed)
	}
	original := typeutil.As[map[string]any](typeutil.As[map[string]any](workflow["1"])["inputs"])["seed"]
	if original != float64(1) {
		t.Fatalf("source workflow was mutated: %v", original)
	}
}

func TestComfyUIAdapterSubmitPollAndCancel(t *testing.T) {
	var interrupted atomic.Bool
	promptSubmitted := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/prompt":
			promptSubmitted <- struct{}{}
			_, _ = w.Write([]byte(`{"prompt_id":"prompt-1"}`))
		case "/history/prompt-1":
			_, _ = w.Write([]byte(`{"prompt-1":{"status":{"completed":true},"outputs":{"7":{"images":[{"filename":"result.png","subfolder":"runs","type":"output"}]}}}}`))
		case "/interrupt":
			interrupted.Store(true)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewOperationExecutors()["comfyui_workflow"]
	run := &iapiserver.ApplicationRun{CapabilitySourceSnapshot: map[string]any{"comfyui_api_workflow": map[string]any{"7": map[string]any{"class_type": "SaveImage"}}}}
	output, err := executor.Execute(context.Background(), testEngine(server.URL, "comfyui"), run)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := sliceutil.ToInterfaceSlice(output["artifacts"])
	if len(artifacts) != 1 || !strings.Contains(maputil.FirstString(artifacts[0].(map[string]any), "content_ref"), "/view?") {
		t.Fatalf("unexpected ComfyUI artifacts: %#v", output)
	}
	<-promptSubmitted

	cancelCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = executor.Execute(cancelCtx, testEngine(server.URL, "comfyui"), run)
	}()
	<-promptSubmitted
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if !interrupted.Load() {
		t.Fatal("ComfyUI interrupt was not sent on cancellation")
	}
}

func TestModelArkAdapterSubmitAndPoll(t *testing.T) {
	var submitted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer authentication: %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/contents/generations/tasks":
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":"task-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/contents/generations/tasks/task-1":
			_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded","video_url":"https://cdn.example/result.mp4"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	output, err := NewOperationExecutors()["byteplus_text_to_video"].Execute(context.Background(), testEngine(server.URL, "byteplus_modelark"), &iapiserver.ApplicationRun{InputSnapshot: map[string]any{"prompt": "hello", "model": "seedance"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := submitted["content"]; !ok {
		t.Fatalf("prompt was not translated to content: %#v", submitted)
	}
	if len(sliceutil.ToInterfaceSlice(output["artifacts"])) != 1 {
		t.Fatalf("video artifact was not normalized: %#v", output)
	}
}

func TestProviderErrorAndAKSKMapping(t *testing.T) {
	t.Run("authentication rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "unauthorized", http.StatusUnauthorized) }))
		defer server.Close()
		_, err := NewEngineAdapters()["deepseek_official"].Check(context.Background(), testEngine(server.URL, "deepseek_official"))
		if errors.ToStatus(err).Code != code.ErrAIAppEngineAuthConfigInvalid {
			t.Fatalf("unexpected error mapping: %v", err)
		}
	})

	t.Run("ak sk signature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "HMAC-SHA256 Credential=access/") || r.Header.Get("X-Date") == "" {
				t.Errorf("request was not signed: %#v", r.Header)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()
		engine := testEngine(server.URL, "byteplus_modelark")
		engine.AuthType = "ak_sk"
		engine.AuthConfig = map[string]any{"access_key": "access", "secret_key": "secret"}
		if _, err := NewEngineAdapters()["byteplus_modelark"].Check(context.Background(), engine); err != nil {
			t.Fatal(err)
		}
	})
}

func testEngine(baseURL, engineType string) *iapiserver.EngineInstance {
	return &iapiserver.EngineInstance{
		ApplicationEngineTypeID: engineType,
		BaseURL:                 baseURL,
		AuthType:                "api_key",
		AuthConfig:              map[string]any{"api_key": "secret"},
		RequestTimeoutSeconds:   2,
		TaskTimeoutSeconds:      5,
		Enabled:                 true,
		HealthStatus:            iapiserver.EngineHealthOnline,
	}
}

func TestProviderRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	engine := testEngine(server.URL, "deepseek_official")
	engine.RequestTimeoutSeconds = 0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := NewEngineAdapters()["deepseek_official"].Check(ctx, engine)
	if errors.ToStatus(err).Code != code.ErrAIAppEngineUnavailable {
		t.Fatalf("unexpected timeout error: %v", err)
	}
}

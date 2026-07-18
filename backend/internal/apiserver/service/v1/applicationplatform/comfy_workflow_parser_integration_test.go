package applicationplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestComfy2GoAcceptanceWorkflow(t *testing.T) {
	path := os.Getenv("OMNIMAM_COMFY_ACCEPTANCE_WORKFLOW")
	if path == "" {
		t.Skip("OMNIMAM_COMFY_ACCEPTANCE_WORKFLOW is not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if err := json.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	if nodes, _ := workflow["nodes"].([]any); len(nodes) != 16 {
		t.Fatalf("expected 16 nodes, got %d", len(nodes))
	}
	if links, _ := workflow["links"].([]any); len(links) != 15 {
		t.Fatalf("expected 15 links, got %d", len(links))
	}
	endpoint := os.Getenv("OMNIMAM_COMFY_OBJECT_INFO_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8188/object_info"
	}
	response, err := http.Get(endpoint)
	if err != nil {
		t.Skipf("ComfyUI unavailable: %v", err)
	}
	defer response.Body.Close()
	var objectInfo map[string]any
	if err := json.NewDecoder(response.Body).Decode(&objectInfo); err != nil {
		t.Fatal(err)
	}
	api, err := (comfy2GoWorkflowParser{}).VisualToAPI(workflow, objectInfo)
	if err != nil {
		t.Fatal(err)
	}
	if len(api) == 0 {
		t.Fatal("comfy2go returned an empty API prompt")
	}
	kSampler := mapValue(mapValue(api["2"])["inputs"])
	seed, _ := kSampler["seed"].(float64)
	if seed != 395983186260226 || fmt.Sprint(kSampler["steps"]) != "8" || fmt.Sprint(kSampler["sampler_name"]) != "er_sde" || fmt.Sprint(kSampler["scheduler"]) != "simple" {
		t.Fatalf("KSampler widgets were mapped out of order: %#v", kSampler)
	}
	latent := mapValue(mapValue(api["10"])["inputs"])
	if fmt.Sprint(latent["batch_size"]) != "1" {
		t.Fatalf("EmptyLatentImage batch_size was mapped incorrectly: %#v", latent)
	}
	lora := mapValue(mapValue(api["38"])["inputs"])
	if stringValue(lora["lora_loader_data"]) == "" {
		t.Fatal("ZmlPowerLoraLoader adapter did not preserve lora_loader_data")
	}
}

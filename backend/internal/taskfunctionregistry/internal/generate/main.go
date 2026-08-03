package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	registrySource = "ssot/01_contracts/domains/task-center/function-registry.yaml"
	schemaSource   = "ssot/01_contracts/domains/task-center/function-registry.schema.yaml"
)

func main() {
	root := flag.String("root", "", "repository root")
	flag.Parse()
	if *root == "" {
		fatalf("-root is required")
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fatalf("resolve repository root: %v", err)
	}
	destination := filepath.Join(absRoot, "backend/internal/taskfunctionregistry/assets")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		fatalf("create asset directory: %v", err)
	}

	registry := read(filepath.Join(absRoot, registrySource))
	schema := read(filepath.Join(absRoot, schemaSource))
	write(filepath.Join(destination, "function-registry.yaml"), registry)
	write(filepath.Join(destination, "function-registry.schema.yaml"), schema)

	referenced := retryableReferences(registry)
	retryability := loadRetryability(filepath.Join(absRoot, "ssot/01_contracts/domains"))
	selected := make(map[string]bool, len(referenced))
	for _, code := range referenced {
		retryable, ok := retryability[code]
		if !ok {
			fatalf("registry references unknown error code %q", code)
		}
		selected[code] = retryable
	}
	rawRetryability, err := json.MarshalIndent(selected, "", "  ")
	if err != nil {
		fatalf("marshal retryability catalog: %v", err)
	}
	write(filepath.Join(destination, "error-retryability.json"), append(rawRetryability, '\n'))

	commit := command(absRoot, "git", "-C", "ssot", "rev-parse", "HEAD")
	registrySum := sha256.Sum256(registry)
	metadata := fmt.Sprintf("ssot_commit=%s\nregistry_sha256=%s\n", commit, hex.EncodeToString(registrySum[:]))
	write(filepath.Join(destination, "SOURCE"), []byte(metadata))
}

func retryableReferences(raw []byte) []string {
	var document map[string]any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		fatalf("decode function registry: %v", err)
	}
	functions, ok := document["functions"].([]any)
	if !ok {
		fatalf("function registry has no functions array")
	}
	seen := make(map[string]struct{})
	for _, item := range functions {
		entry, ok := item.(map[string]any)
		if !ok {
			fatalf("function registry entry has unexpected type %T", item)
		}
		policy, ok := entry["retry_policy"].(map[string]any)
		if !ok {
			fatalf("function registry entry %v has no retry policy", entry["function_ref"])
		}
		codes, ok := policy["retryable_error_codes"].([]any)
		if !ok {
			fatalf("function registry entry %v has no retryable error codes", entry["function_ref"])
		}
		for _, value := range codes {
			code, ok := value.(string)
			if !ok || code == "" {
				fatalf("function registry entry %v has an invalid retryable error code", entry["function_ref"])
			}
			seen[code] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for code := range seen {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func loadRetryability(domainsRoot string) map[string]bool {
	paths, err := filepath.Glob(filepath.Join(domainsRoot, "*", "errors.yaml"))
	if err != nil {
		fatalf("enumerate SSOT error catalogs: %v", err)
	}
	result := make(map[string]bool)
	for _, path := range paths {
		var document any
		if err := yaml.Unmarshal(read(path), &document); err != nil {
			fatalf("decode %s: %v", path, err)
		}
		collectErrors(document, result)
	}
	return result
}

func collectErrors(value any, result map[string]bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectErrors(item, result)
		}
	case map[string]any:
		if code, ok := typed["code"].(string); ok {
			if retryable, present := typed["retryable"].(bool); present {
				result[code] = retryable
			}
		}
		for key, item := range typed {
			if _, hasCode := typed["code"]; !hasCode {
				if metadata, ok := item.(map[string]any); ok {
					if _, present := metadata["code"]; !present {
						metadata["code"] = key
					}
				}
			}
			collectErrors(item, result)
		}
	}
}

func command(workdir string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	raw, err := cmd.Output()
	if err != nil {
		fatalf("run %s: %v", name, err)
	}
	for len(raw) > 0 && (raw[len(raw)-1] == '\n' || raw[len(raw)-1] == '\r') {
		raw = raw[:len(raw)-1]
	}
	return string(raw)
}

func read(path string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	return raw
}

func write(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

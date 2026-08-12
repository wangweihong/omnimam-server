package appstudio

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	BlueprintWebReactID      = "web-react"
	BlueprintWebReactVersion = "v1"
	BlueprintPromptSystem    = "system"
	BlueprintPromptInitial   = "initial"
	BlueprintPromptFollowup  = "followup"
	// BlueprintPromptFix 随 Blueprint 发布供后续阶段使用；spec-v1.23.1 明确禁止当前阶段路由。
	BlueprintPromptFix = "fix"
)

// 目录模式默认忽略点文件，必须显式嵌入 Blueprint 清单要求的受控文件。
//
//go:embed blueprints blueprints/web-react/v1/template/.gitignore blueprints/web-react/v1/template/.gitlab-ci.yml
var blueprintFS embed.FS

// Blueprint 是随 Server 发布的只读模板、prompt 和 validation 合同。
type Blueprint struct {
	ID         string
	Version    string
	Files      map[string][]byte
	Prompts    map[string]string
	Validation []string
	CIInclude  string
}

type blueprintManifest struct {
	ID         string            `yaml:"id"`
	Version    string            `yaml:"version"`
	Files      []string          `yaml:"files"`
	Prompts    map[string]string `yaml:"prompts"`
	Validation []string          `yaml:"validation"`
	CIInclude  string            `yaml:"ci_include"`
}

// LoadBlueprint 只加载编译进 Server 的 Blueprint，未知版本或缺失资产直接失败。
func LoadBlueprint(id, version string) (*Blueprint, error) {
	if id != BlueprintWebReactID || version != BlueprintWebReactVersion {
		return nil, fmt.Errorf("unsupported blueprint %s@%s", id, version)
	}
	root := path.Join("blueprints", id, version)
	manifestData, err := fs.ReadFile(blueprintFS, path.Join(root, "blueprint.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read blueprint manifest: %w", err)
	}
	var manifest blueprintManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("decode blueprint manifest: %w", err)
	}
	if manifest.ID != id || manifest.Version != version || len(manifest.Files) == 0 {
		return nil, fmt.Errorf("blueprint manifest identity or files are invalid")
	}
	files := make(map[string][]byte, len(manifest.Files))
	for _, name := range manifest.Files {
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || path.Dir(name) == ".." || name == "." {
			return nil, fmt.Errorf("blueprint file path is invalid: %q", name)
		}
		data, err := fs.ReadFile(blueprintFS, path.Join(root, "template", name))
		if err != nil {
			return nil, fmt.Errorf("read blueprint file %q: %w", name, err)
		}
		files[name] = data
	}
	prompts := make(map[string]string, len(manifest.Prompts))
	for kind, name := range manifest.Prompts {
		if kind == "" || name == "" {
			return nil, fmt.Errorf("blueprint prompt mapping is invalid")
		}
		data, err := fs.ReadFile(blueprintFS, path.Join(root, "prompts", name))
		if err != nil {
			return nil, fmt.Errorf("read blueprint prompt %q: %w", kind, err)
		}
		prompts[kind] = string(data)
	}
	if prompts[BlueprintPromptSystem] == "" || prompts[BlueprintPromptInitial] == "" || prompts[BlueprintPromptFollowup] == "" || prompts[BlueprintPromptFix] == "" || len(manifest.Validation) == 0 {
		return nil, fmt.Errorf("blueprint prompt or validation contract is incomplete")
	}
	if err := validateBlueprintCIInclude(manifest.CIInclude, files[".gitlab-ci.yml"]); err != nil {
		return nil, err
	}
	return &Blueprint{ID: manifest.ID, Version: manifest.Version, Files: files, Prompts: prompts, Validation: append([]string(nil), manifest.Validation...), CIInclude: manifest.CIInclude}, nil
}

func validateBlueprintCIInclude(expected string, data []byte) error {
	var config struct {
		Include []struct {
			Project string `yaml:"project"`
			File    string `yaml:"file"`
			Ref     string `yaml:"ref"`
		} `yaml:"include"`
	}
	if expected == "" || len(data) == 0 || yaml.Unmarshal(data, &config) != nil || len(config.Include) != 1 {
		return fmt.Errorf("blueprint CI include contract is invalid")
	}
	include := config.Include[0]
	actual := include.Project + ":" + include.File + "@" + include.Ref
	if strings.TrimSpace(include.Project) == "" || strings.TrimSpace(include.File) == "" || strings.TrimSpace(include.Ref) == "" || actual != expected {
		return fmt.Errorf("blueprint CI include does not match its manifest")
	}
	return nil
}

// TemplatePaths 返回稳定排序的模板文件名，便于 Starter commit 和测试确定性。
func (b *Blueprint) TemplatePaths() []string {
	paths := make([]string, 0, len(b.Files))
	for name := range b.Files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}

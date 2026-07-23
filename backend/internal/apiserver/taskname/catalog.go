// Package taskname resolves stable system task name keys into localized display names.
package taskname

import (
	"fmt"
	"maps"
	"strings"
)

const (
	LanguageChinese = "zh-CN"
	LanguageEnglish = "en-US"

	ApplicationRun         = "application-platform.run"
	AssetThumbnail         = "asset-library.thumbnail.generate"
	ArtifactProcess        = "asset-library.artifact.process"
	RepresentationInspect  = "asset-library.representation.inspect"
	RepresentationGenerate = "asset-library.representation.generate"
	RepresentationFinalize = "asset-library.representation.finalize"
	RepresentationBuild    = "asset-library.representation.build"
	RepresentationBackfill = "asset-library.representation.backfill"
	ComfyUIWorkflowTest    = "application-platform.comfyui.workflow-test"
	ComfyUISubmit          = "application-platform.comfyui.submit"
	ComfyUIPoll            = "application-platform.comfyui.poll"
	ComfyUICollectPreview  = "application-platform.comfyui.collect-preview"
	EngineHealthReconcile  = "application-platform.engine-health"
	ObjectInfoRefresh      = "application-platform.comfyui-object-info-refresh"
	BackfillThumbnail      = "asset-library.representation.backfill-thumbnail"
)

type definition struct {
	params []string
	names  map[string]string
}

var catalog = map[string]definition{
	ApplicationRun:         {names: names("应用平台运行", "Application Platform Run")},
	AssetThumbnail:         {names: names("生成素材缩略图", "Generate asset thumbnail")},
	ArtifactProcess:        {names: names("处理已上传制品", "Process uploaded Artifact")},
	RepresentationInspect:  {names: names("检查 AssetVersion 表现形式", "Inspect AssetVersion representations")},
	RepresentationGenerate: {params: []string{"representation_type"}, names: names("生成 {{representation_type}} 表现形式", "Generate {{representation_type}}")},
	RepresentationFinalize: {names: names("完成 AssetVersion 表现形式构建", "Finalize AssetVersion representations")},
	RepresentationBuild:    {names: names("构建 AssetVersion 表现形式", "Build AssetVersion representations")},
	RepresentationBackfill: {names: names("每日补全 AssetVersion 表现形式", "Daily AssetVersion representation backfill")},
	ComfyUIWorkflowTest:    {names: names("ComfyUI 工作流试运行", "ComfyUI workflow test")},
	ComfyUISubmit:          {names: names("提交生成请求", "Submit prompt")},
	ComfyUIPoll:            {names: names("查询队列与历史", "Poll queue and history")},
	ComfyUICollectPreview:  {names: names("收集临时预览", "Collect temporary preview")},
	EngineHealthReconcile:  {names: names("应用引擎健康巡检", "Application engine health reconcile")},
	ObjectInfoRefresh:      {names: names("每日刷新 ComfyUI object_info", "Daily ComfyUI object_info refresh")},
	BackfillThumbnail:      {names: names("补全 AssetVersion 缩略图", "Backfill AssetVersion thumbnail")},
}

func names(chinese, english string) map[string]string {
	return map[string]string{LanguageChinese: chinese, LanguageEnglish: english}
}

// Resolve validates a registered system name and returns every catalog language.
func Resolve(key string, params map[string]string) (map[string]string, error) {
	entry, ok := catalog[key]
	if !ok {
		return nil, fmt.Errorf("task name key %q is not registered", key)
	}
	if len(params) != len(entry.params) {
		return nil, fmt.Errorf("task name key %q expects %d parameters", key, len(entry.params))
	}
	resolved := maps.Clone(entry.names)
	for _, param := range entry.params {
		value, ok := params[param]
		if !ok || value == "" {
			return nil, fmt.Errorf("task name key %q requires parameter %q", key, param)
		}
		placeholder := "{{" + param + "}}"
		for language, name := range resolved {
			resolved[language] = strings.ReplaceAll(name, placeholder, value)
		}
	}
	return resolved, nil
}

// English resolves the compatibility name stored in the existing name field.
func English(key string, params map[string]string) (string, error) {
	resolved, err := Resolve(key, params)
	if err != nil {
		return "", err
	}
	return resolved[LanguageEnglish], nil
}

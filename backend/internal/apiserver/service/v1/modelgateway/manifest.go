package modelgateway

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// ParseProviderCapabilityManifest 严格解析编译进 provider 包的多文档 const YAML。
func ParseProviderCapabilityManifest(source, manifest string) ([]iapiserver.AIAppProviderCapability, error) {
	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	decoder.KnownFields(true)
	items := make([]iapiserver.AIAppProviderCapability, 0, 1)
	for document := 1; ; document++ {
		var item iapiserver.AIAppProviderCapability
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("provider capability manifest %s document %d: %w", source, document, err)
		}
		if strings.TrimSpace(item.ID) == "" {
			return nil, fmt.Errorf("provider capability manifest %s document %d is empty", source, document)
		}
		if item.SchemaVersion != "1.0" {
			return nil, fmt.Errorf("provider capability manifest %s document %d has unsupported schema_version %q", source, document, item.SchemaVersion)
		}
		item.Origin = iapiserver.ProviderCapabilityOriginStatic
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("provider capability manifest %s has no documents", source)
	}
	return items, nil
}

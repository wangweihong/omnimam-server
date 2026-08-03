package taskfunctionregistry

import _ "embed"

var (
	//go:embed assets/function-registry.yaml
	registryYAML []byte
	//go:embed assets/function-registry.schema.yaml
	registrySchemaYAML []byte
	//go:embed assets/error-retryability.json
	errorRetryabilityJSON []byte
	//go:embed assets/SOURCE
	sourceMetadata []byte
)

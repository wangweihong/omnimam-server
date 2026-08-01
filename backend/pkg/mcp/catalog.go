package mcp

const schemaDialect = "https://json-schema.org/draft/2020-12/schema"

type catalogEntry struct {
	Title       string
	Description string
	Input       map[string]any
	Output      map[string]any
}

var toolCatalog = map[string]catalogEntry{
	ToolCapabilitiesList: {
		Title: "List OmniMAM capabilities", Description: "List public, read-only capability definitions. Capabilities cannot be invoked directly.",
		Input: objectSchema(nil, properties{
			"domain": stringSchema(0, 64), "query": stringSchema(0, 255), "status": enumSchema("available"),
			"cursor": stringSchema(0, 512), "limit": integerSchema(1, 100, 50),
		}),
		Output: listOutputSchema("items", capabilitySummarySchema()),
	},
	ToolCapabilitiesGet: {
		Title: "Get an OmniMAM capability", Description: "Read a public capability definition and navigate to runnable Applications.",
		Input:  objectSchema([]string{"capability_id"}, properties{"capability_id": stringSchema(1, 255)}),
		Output: capabilityDetailSchema(),
	},
	ToolApplicationsList: {
		Title: "List runnable OmniMAM applications", Description: "List visible, published Applications with run_enabled=true.",
		Input: objectSchema(nil, properties{
			"query": stringSchema(0, 255), "capability_id": stringSchema(0, 255),
			"cursor": stringSchema(0, 512), "limit": integerSchema(1, 100, 50),
		}),
		Output: listOutputSchema("items", applicationSummarySchema()),
	},
	ToolApplicationsGet: {
		Title: "Get an OmniMAM application", Description: "Read a visible Application and its immutable published input/output contract.",
		Input:  objectSchema([]string{"application_id"}, properties{"application_id": stringSchema(1, 255)}),
		Output: applicationDetailSchema(),
	},
	ToolApplicationsRun: {
		Title: "Run an OmniMAM application", Description: "Create an idempotent ApplicationRun. Reuse the same idempotency_key for retries.",
		Input: objectSchema([]string{"application_id", "input", "idempotency_key"}, properties{
			"application_id": stringSchema(1, 255), "application_version_id": stringSchema(1, 255),
			"input":           map[string]any{"type": "object", "maxProperties": 256, "additionalProperties": true},
			"idempotency_key": stringSchema(1, 128), "metadata": scalarMapSchema(64),
		}),
		Output: applicationRunAcceptedSchema(),
	},
	ToolApplicationRunsGet: {
		Title: "Get an OmniMAM application run", Description: "Read a visible ApplicationRun, AtomicTask projection, and Artifact links.",
		Input:  objectSchema([]string{"application_run_id"}, properties{"application_run_id": stringSchema(1, 255)}),
		Output: applicationRunProjectionSchema(),
	},
	ToolApplicationRunsCancel: {
		Title: "Cancel an OmniMAM application run", Description: "Request cooperative cancellation without claiming a final cancellation state.",
		Input: objectSchema([]string{"application_run_id"}, properties{
			"application_run_id": stringSchema(1, 255), "reason": withDefault(stringSchema(0, 500), ""),
		}),
		Output: objectSchema([]string{"application_run_id", "accepted", "status"}, properties{
			"application_run_id": stringSchema(0, 0), "accepted": map[string]any{"type": "boolean"},
			//nolint:misspell // MCP 2026-07-28 fixes this status to the British spelling.
			"status": enumSchema("cancel_requested", "succeeded", "failed", "cancelled"),
		}),
	},
	ToolAssetsSearch: {
		Title: "Search OmniMAM assets", Description: "Search assets visible to the current principal without exposing storage paths.",
		Input: objectSchema(nil, properties{
			"query":       stringSchema(0, 500),
			"media_types": arraySchema(enumSchema("image", "video", "audio", "text", "document", "model_3d", "prompt", "prompt_template", "pdf", "other"), 0, 10, true),
			"tags":        arraySchema(stringSchema(1, 128), 0, 50, true),
			"sort":        withDefault(enumSchema("created_at_asc", "created_at_desc", "updated_at_asc", "updated_at_desc"), "created_at_desc"),
			"cursor":      stringSchema(0, 512), "limit": integerSchema(1, 100, 20),
		}),
		Output: listOutputSchema("items", assetSummarySchema()),
	},
	ToolAssetsGet: {
		Title: "Get an OmniMAM asset", Description: "Read visible asset metadata, current version, tags, and controlled Representations.",
		Input: objectSchema([]string{"asset_id"}, properties{
			"asset_id": stringSchema(1, 255), "include_representations": withDefault(map[string]any{"type": "boolean"}, true),
		}),
		Output: assetDetailSchema(),
	},
	ToolAssetsPrepareUpload: {
		Title: "Prepare an OmniMAM asset upload", Description: "Create an idempotent Asset Library UploadSession. Binary content is sent to the returned controlled API endpoint.",
		Input: objectSchema([]string{"filename", "media_type", "size_bytes", "checksum", "idempotency_key"}, properties{
			"filename": stringSchema(1, 255), "media_type": stringSchema(1, 255),
			"size_bytes": map[string]any{"type": "integer", "format": "int64", "minimum": 1},
			"checksum": objectSchema([]string{"algorithm", "value"}, properties{
				"algorithm": constSchema("sha256"), "value": map[string]any{"type": "string", "pattern": "^[a-fA-F0-9]{64}$"},
			}),
			"metadata": scalarMapSchema(64), "idempotency_key": stringSchema(1, 128),
		}),
		Output: objectSchema([]string{"upload_id", "method", "content_upload_url", "required_headers", "authorization_required", "expires_at"}, properties{
			"upload_id": stringSchema(0, 0), "method": constSchema("POST"),
			"content_upload_url":     map[string]any{"type": "string", "format": "uri", "pattern": "/api/v1/asset-uploads/[^/]+/content$"},
			"required_headers":       objectSchema([]string{"Content-Type"}, properties{"Content-Type": stringSchema(0, 0)}),
			"authorization_required": constSchema(true), "expires_at": dateTimeSchema(),
		}),
	},
	ToolAssetsCompleteUpload: {
		Title: "Complete an OmniMAM asset upload", Description: "Complete an idempotent UploadSession and return the real Asset processing state.",
		Input: objectSchema([]string{"upload_id", "idempotency_key"}, properties{
			"upload_id": stringSchema(1, 255), "idempotency_key": stringSchema(1, 128),
		}),
		Output: objectSchema([]string{"upload_id", "asset", "status", "uri"}, properties{
			"upload_id": stringSchema(0, 0), "asset": assetSummarySchema(),
			"status": enumSchema("processing", "ready", "ready_with_warnings", "failed"), "uri": uriSchema(),
		}),
	},
}

func ToolDefinitions(visible func(string) bool) []ToolDefinition {
	definitions := make([]ToolDefinition, 0, len(ToolNames))
	for _, name := range ToolNames {
		if visible != nil && !visible(name) {
			continue
		}
		entry := toolCatalog[name]
		definitions = append(definitions, ToolDefinition{
			Name: name, Title: entry.Title, Description: entry.Description,
			InputSchema: entry.Input, OutputSchema: entry.Output,
		})
	}
	return definitions
}

func ToolOutputSchema(name string) (map[string]any, bool) {
	entry, ok := toolCatalog[name]
	return entry.Output, ok
}

func ResourceTemplates() []ResourceTemplate {
	return []ResourceTemplate{
		{URITemplate: "omnimam://capabilities/{capability_id}", Name: "OmniMAM Capability", Description: "Read a public capability definition.", MIMEType: "application/json"},
		{URITemplate: "omnimam://applications/{application_id}", Name: "OmniMAM Application", Description: "Read a visible runnable Application.", MIMEType: "application/json"},
		{URITemplate: "omnimam://application-runs/{application_run_id}", Name: "OmniMAM Application Run", Description: "Read an authorized ApplicationRun projection.", MIMEType: "application/json"},
		{URITemplate: "omnimam://assets/{asset_id}", Name: "OmniMAM Asset", Description: "Read authorized Asset metadata.", MIMEType: "application/json"},
		{URITemplate: "omnimam://assets/{asset_id}/representations/{representation}", Name: "OmniMAM Asset Representation", Description: "Read an authorized Representation projection.", MIMEType: "application/json"},
		{URITemplate: "omnimam://artifacts/{artifact_id}", Name: "OmniMAM Artifact", Description: "Read authorized Artifact metadata.", MIMEType: "application/json"},
	}
}

type properties map[string]any

func objectSchema(required []string, props properties) map[string]any {
	schema := map[string]any{"$schema": schemaDialect, "type": "object", "additionalProperties": false, "properties": map[string]any(props)}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(minimum, maximum int) map[string]any {
	schema := map[string]any{"type": "string"}
	if minimum > 0 {
		schema["minLength"] = minimum
	}
	if maximum > 0 {
		schema["maxLength"] = maximum
	}
	return schema
}

func integerSchema(minimum, maximum, defaultValue int) map[string]any {
	return map[string]any{"type": "integer", "minimum": minimum, "maximum": maximum, "default": defaultValue}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func constSchema(value any) map[string]any { return map[string]any{"const": value} }
func uriSchema() map[string]any            { return map[string]any{"type": "string", "format": "uri"} }
func dateTimeSchema() map[string]any       { return map[string]any{"type": "string", "format": "date-time"} }

func arraySchema(items map[string]any, minimum, maximum int, unique bool) map[string]any {
	schema := map[string]any{"type": "array", "items": items}
	if minimum > 0 {
		schema["minItems"] = minimum
	}
	if maximum > 0 {
		schema["maxItems"] = maximum
	}
	if unique {
		schema["uniqueItems"] = true
	}
	return schema
}

func withDefault(schema map[string]any, value any) map[string]any {
	copySchema := make(map[string]any, len(schema)+1)
	for key, item := range schema {
		copySchema[key] = item
	}
	copySchema["default"] = value
	return copySchema
}

func scalarMapSchema(maximum int) map[string]any {
	return map[string]any{"type": "object", "maxProperties": maximum, "additionalProperties": map[string]any{"type": []string{"string", "number", "boolean", "null"}}}
}

func listOutputSchema(field string, item map[string]any) map[string]any {
	return objectSchema([]string{"total", field}, properties{
		"total": map[string]any{"type": "integer", "minimum": 0},
		field:   arraySchema(item, 0, 0, false), "next_cursor": stringSchema(0, 512),
	})
}

func capabilitySummarySchema() map[string]any {
	return objectSchema([]string{"capability_id", "name", "description", "status", "supports_direct_invoke", "schema_uri", "applications"}, properties{
		"capability_id": stringSchema(0, 0), "name": stringSchema(0, 0), "description": stringSchema(0, 0),
		"status": enumSchema("available", "unavailable", "disabled"), "supports_direct_invoke": constSchema(false), "schema_uri": uriSchema(),
		"applications": arraySchema(applicationNavigationSchema(), 0, 0, false),
	})
}

func capabilityDetailSchema() map[string]any {
	schema := capabilitySummarySchema()
	required := schema["required"].([]string)
	schema["required"] = append(required, "input_schema", "output_schema")
	props := schema["properties"].(map[string]any)
	props["input_schema"], props["output_schema"] = jsonSchemaSchema(), jsonSchemaSchema()
	return schema
}

func applicationNavigationSchema() map[string]any {
	return objectSchema([]string{"application_id", "name", "published_version_id", "run_enabled"}, properties{
		"application_id": stringSchema(0, 0), "name": stringSchema(0, 0), "published_version_id": stringSchema(0, 0), "run_enabled": map[string]any{"type": "boolean"},
	})
}

func applicationSummarySchema() map[string]any {
	return objectSchema([]string{"application_id", "name", "description", "published_version_id", "run_enabled", "schema_uri"}, properties{
		"application_id": stringSchema(0, 0), "name": stringSchema(0, 0), "description": stringSchema(0, 0), "published_version_id": stringSchema(0, 0),
		"run_enabled": constSchema(true), "availability": enumSchema("available", "unavailable"), "schema_uri": uriSchema(),
	})
}

func applicationDetailSchema() map[string]any {
	schema := applicationSummarySchema()
	schema["required"] = append(schema["required"].([]string), "published_version", "input_schema", "output_schema")
	props := schema["properties"].(map[string]any)
	props["published_version"] = objectSchema([]string{"application_version_id", "version"}, properties{
		"application_version_id": stringSchema(0, 0), "version": map[string]any{"type": "integer", "minimum": 1},
	})
	props["input_schema"], props["output_schema"] = jsonSchemaSchema(), jsonSchemaSchema()
	return schema
}

func applicationRunAcceptedSchema() map[string]any {
	return objectSchema([]string{"application_run_id", "application_id", "application_version_id", "status", "status_uri"}, properties{
		"application_run_id": stringSchema(0, 0), "application_id": stringSchema(0, 0), "application_version_id": stringSchema(0, 0),
		"status": enumSchema("queued", "running"), "status_uri": uriSchema(), "mcp_task_id": stringSchema(0, 0),
	})
}

func applicationRunProjectionSchema() map[string]any {
	return objectSchema([]string{"application_run_id", "application", "application_version", "status", "progress", "outputs", "created_at", "resource_version"}, properties{
		"application_run_id": stringSchema(0, 0), "application": applicationSummarySchema(),
		"application_version": objectSchema([]string{"application_version_id", "version"}, properties{"application_version_id": stringSchema(0, 0), "version": map[string]any{"type": "integer", "minimum": 1}}),
		"atomic_task":         atomicTaskProjectionSchema(),
		//nolint:misspell // MCP 2026-07-28 fixes this status to the British spelling.
		"status":   enumSchema("queued", "running", "cancel_requested", "succeeded", "failed", "cancelled"),
		"progress": map[string]any{"type": "number", "minimum": 0, "maximum": 1}, "outputs": arraySchema(artifactSchema(), 0, 0, false),
		"error": businessErrorSchema(), "created_at": dateTimeSchema(), "started_at": dateTimeSchema(), "completed_at": dateTimeSchema(),
		"resource_version": map[string]any{"type": "integer", "minimum": 0},
	})
}

func atomicTaskProjectionSchema() map[string]any {
	return objectSchema([]string{"atomic_task_id", "status", "progress", "resource_version"}, properties{
		"atomic_task_id": stringSchema(0, 0),
		"status": enumSchema(
			"PENDING", "BLOCKED", "READY", "RUNNING", "RETRYING", "CANCEL_REQUESTED",
			"SUCCESS", "FAILED", "CANCELED", "TIMEOUT", "SKIPPED",
		),
		"progress":         map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		"phase":            stringSchema(0, 0),
		"resource_version": map[string]any{"type": "integer", "minimum": 0},
	})
}

func artifactSchema() map[string]any {
	return objectSchema([]string{"artifact_id", "output_key", "sequence", "media_type", "processing_status", "registration_status", "uri"}, properties{
		"artifact_id": stringSchema(0, 0), "output_key": stringSchema(0, 0), "sequence": map[string]any{"type": "integer", "minimum": 0},
		"media_type": stringSchema(0, 0), "processing_status": enumSchema("created", "transferring", "processing", "ready", "failed", "deleted"),
		"registration_status": enumSchema("pending", "registered", "failed"), "asset_id": stringSchema(0, 0), "uri": uriSchema(),
	})
}

func assetSummarySchema() map[string]any {
	return objectSchema([]string{"asset_id", "display_name", "media_type", "status", "thumbnail_status", "uri", "created_at"}, properties{
		"asset_id": stringSchema(0, 0), "display_name": stringSchema(0, 0), "media_type": enumSchema("image", "video", "audio", "text", "document", "model_3d", "prompt", "prompt_template", "pdf", "other"),
		"format": stringSchema(0, 0), "size_bytes": map[string]any{"type": "integer", "minimum": 0}, "width": map[string]any{"type": "integer", "minimum": 0},
		"height": map[string]any{"type": "integer", "minimum": 0}, "duration_seconds": map[string]any{"type": "number", "minimum": 0},
		"status": enumSchema("active", "archived"), "thumbnail_status": enumSchema("none", "pending", "ready", "failed"),
		"uri": uriSchema(), "thumbnail_url": uriSchema(), "created_at": dateTimeSchema(),
	})
}

func assetDetailSchema() map[string]any {
	schema := assetSummarySchema()
	schema["required"] = append(schema["required"].([]string), "current_version", "representations", "tags")
	props := schema["properties"].(map[string]any)
	props["current_version"] = objectSchema([]string{"asset_version_id", "version", "status"}, properties{
		"asset_version_id": stringSchema(0, 0), "version": map[string]any{"type": "integer", "minimum": 1}, "status": enumSchema("processing", "ready", "ready_with_warnings", "failed"),
	})
	props["representations"] = arraySchema(representationProjectionSchema(), 0, 0, false)
	props["tags"] = arraySchema(stringSchema(0, 0), 0, 0, false)
	return schema
}

func representationProjectionSchema() map[string]any {
	return objectSchema([]string{"representation_id", "type", "media_type", "uri"}, properties{
		"representation_id": stringSchema(0, 0),
		"type":              enumSchema("original", "thumbnail", "preview", "playback", "manifest"),
		"media_type":        stringSchema(0, 0),
		"width":             map[string]any{"type": "integer", "minimum": 0},
		"height":            map[string]any{"type": "integer", "minimum": 0},
		"uri":               uriSchema(),
		"access_url":        uriSchema(),
	})
}

func businessErrorSchema() map[string]any {
	return objectSchema([]string{"code", "value", "message", "messages", "retryable", "source_domain"}, properties{
		"code": stringSchema(0, 0), "value": map[string]any{"type": "integer"}, "message": stringSchema(0, 0),
		"messages": objectSchema([]string{"zh-CN", "en-US"}, properties{"zh-CN": stringSchema(0, 0), "en-US": stringSchema(0, 0)}),
		"detail":   stringSchema(0, 0), "retryable": map[string]any{"type": "boolean"},
		"source_domain": enumSchema("mcp", "identity", "modelgateway", "application-platform", "task-center", "asset-library"),
		"causes": arraySchema(objectSchema(nil, properties{
			"path": stringSchema(0, 0), "reason": stringSchema(0, 0),
			"expected": map[string]any{}, "actual": map[string]any{},
		}), 0, 0, false),
	})
}

func jsonSchemaSchema() map[string]any {
	return map[string]any{"type": "object", "required": []string{"type"}, "additionalProperties": true, "properties": map[string]any{
		"$schema": map[string]any{"type": "string", "const": schemaDialect}, "type": map[string]any{},
	}}
}

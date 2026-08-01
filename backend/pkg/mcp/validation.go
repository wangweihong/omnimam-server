package mcp

import (
	"encoding/json"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type ResultValidator struct {
	schemas map[string]*jsonschema.Schema
}

func NewResultValidator() (*ResultValidator, error) {
	validator := &ResultValidator{schemas: make(map[string]*jsonschema.Schema, len(ToolNames))}
	for _, name := range ToolNames {
		document, ok := ToolOutputSchema(name)
		if !ok {
			return nil, fmt.Errorf("MCP tool %q has no output schema", name)
		}
		raw, err := json.Marshal(document)
		if err != nil {
			return nil, fmt.Errorf("marshal MCP output schema %q: %w", name, err)
		}
		var canonical any
		if err := json.Unmarshal(raw, &canonical); err != nil {
			return nil, fmt.Errorf("normalize MCP output schema %q: %w", name, err)
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		location := "urn:omnimam:mcp:tool:" + name + ":output"
		if err := compiler.AddResource(location, canonical); err != nil {
			return nil, fmt.Errorf("add MCP output schema %q: %w", name, err)
		}
		compiled, err := compiler.Compile(location)
		if err != nil {
			return nil, fmt.Errorf("compile MCP output schema %q: %w", name, err)
		}
		validator.schemas[name] = compiled
	}
	return validator, nil
}

func (v *ResultValidator) Validate(name string, result any) error {
	if v == nil || v.schemas[name] == nil {
		return fmt.Errorf("MCP tool %q has no compiled output schema", name)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal MCP tool result %q: %w", name, err)
	}
	var canonical any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		return fmt.Errorf("normalize MCP tool result %q: %w", name, err)
	}
	if err := v.schemas[name].Validate(canonical); err != nil {
		return fmt.Errorf("validate MCP tool result %q: %w", name, err)
	}
	return nil
}

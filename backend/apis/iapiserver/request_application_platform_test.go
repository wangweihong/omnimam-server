package iapiserver

import "testing"

func TestApplicationPlatformRequiredZeroValues(t *testing.T) {
	falseValue := false
	engine := &EngineInstanceCreateRequest{AuthConfig: map[string]any{}, Enabled: &falseValue}
	if err := engine.Validate(); err != nil {
		t.Fatalf("required empty auth_config and false enabled should be valid field presence: %v", err)
	}
	binding := &EngineCapabilityBindingCreateRequest{Enabled: &falseValue, Restrictions: map[string]any{}}
	if err := binding.Validate(); err != nil {
		t.Fatalf("required empty restrictions and false enabled should be valid field presence: %v", err)
	}
}

func TestApplicationVersionCreateRequestValidatesSemverAndObjects(t *testing.T) {
	valid := &ApplicationVersionCreateRequest{SemanticVersion: "1.2.3-rc.1+build.7", InputSchema: map[string]any{}, OutputSchema: map[string]any{}, ParameterPolicies: map[string]any{}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid semantic version rejected: %v", err)
	}
	invalid := *valid
	invalid.SemanticVersion = "v1"
	if err := invalid.Validate(); err == nil {
		t.Fatal("invalid semantic version was accepted")
	}
}

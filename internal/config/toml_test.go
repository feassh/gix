package config

import (
	"strings"
	"testing"
)

func TestParseAndEncodeTOML(t *testing.T) {
	raw := `
[ai]
model = "gpt-5-mini"
timeout = 30

[commit]
confirm = false
style = "conventional"
`
	values, err := ParseTOML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseTOML() error = %v", err)
	}
	if values["ai.model"] != "gpt-5-mini" {
		t.Fatalf("expected ai.model to be parsed")
	}
	if values["commit.confirm"] != "false" {
		t.Fatalf("expected commit.confirm to be parsed")
	}
	encoded := EncodeTOML(values)
	if !strings.Contains(encoded, "[ai]") || !strings.Contains(encoded, "model = \"gpt-5-mini\"") {
		t.Fatalf("unexpected encoded toml: %s", encoded)
	}
}

func TestToConfigUsesDirectAPIKey(t *testing.T) {
	values := DefaultValues()
	values["ai.api_key"] = "sk-test-direct"

	cfg, err := values.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig() error = %v", err)
	}
	if cfg.AI.APIKey != "sk-test-direct" {
		t.Fatalf("cfg.AI.APIKey = %q", cfg.AI.APIKey)
	}
}

func TestToConfigSupportsLegacyAPIKeyEnvValue(t *testing.T) {
	values := DefaultValues()
	values["ai.api_key_env"] = "sk-test-legacy"

	cfg, err := values.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig() error = %v", err)
	}
	if cfg.AI.APIKey != "sk-test-legacy" {
		t.Fatalf("cfg.AI.APIKey = %q", cfg.AI.APIKey)
	}
}

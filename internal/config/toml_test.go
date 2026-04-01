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

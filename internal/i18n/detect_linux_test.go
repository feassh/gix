//go:build linux

package i18n

import "testing"

func TestParseLocaleCommand(t *testing.T) {
	output := "LANG=zh_CN.UTF-8\nLC_ALL=\n"
	values := parseLocaleCommand(output)
	if len(values) == 0 || values[0] != "zh_CN.UTF-8" {
		t.Fatalf("parseLocaleCommand() = %#v", values)
	}
}

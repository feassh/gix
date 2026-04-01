//go:build darwin

package i18n

import (
	"os/exec"
	"testing"
)

func TestParseAppleLanguagesOutput(t *testing.T) {
	raw := "(\n    \"zh-Hans-CN\",\n    \"en-US\"\n)"
	if got := parseAppleLanguagesOutput(raw); got != "zh-Hans-CN" {
		t.Fatalf("parseAppleLanguagesOutput() = %q", got)
	}
}

func TestDetectUsesMacOSDefaultsWhenEnvIsNeutral(t *testing.T) {
	t.Setenv("GIX_LANG", "")
	t.Setenv("LC_ALL", "C.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LANG", "C.UTF-8")

	original := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) >= 3 && args[0] == "read" && args[1] == "-g" && args[2] == "AppleLanguages" {
			return exec.Command("sh", "-c", "printf '(\n    \"zh-Hans-CN\"\n)\n'")
		}
		if len(args) >= 3 && args[0] == "read" && args[1] == "-g" && args[2] == "AppleLocale" {
			return exec.Command("sh", "-c", "printf 'zh_CN\n'")
		}
		return exec.Command("sh", "-c", "exit 1")
	}
	defer func() { execCommand = original }()

	if got := Detect(); got != SimplifiedChinese {
		t.Fatalf("Detect() = %s, want %s", got, SimplifiedChinese)
	}
}

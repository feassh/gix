package i18n

import (
	"errors"
	"testing"
)

func TestDetectUsesOverride(t *testing.T) {
	t.Setenv("GIX_LANG", "zh-CN")
	if got := Detect(); got != SimplifiedChinese {
		t.Fatalf("Detect() = %s, want %s", got, SimplifiedChinese)
	}
}

func TestCatalogFallsBackToEnglish(t *testing.T) {
	catalog := NewCatalog(Locale("fr"))
	if got := catalog.S("message.install_complete"); got != "Install complete." {
		t.Fatalf("catalog fallback = %q", got)
	}
}

func TestParseLocaleSkipsNeutralCLocale(t *testing.T) {
	match := parseLocale("C.UTF-8")
	if match.meaningful || match.supported {
		t.Fatalf("parseLocale(C.UTF-8) = %+v, want neutral", match)
	}
}

func TestParseLocaleRecognizesSimplifiedChinese(t *testing.T) {
	match := parseLocale("zh_Hans_CN.UTF-8")
	if !match.supported || match.locale != SimplifiedChinese {
		t.Fatalf("parseLocale(zh_Hans_CN.UTF-8) = %+v", match)
	}
}

func TestLocalizeErrorForGitHubReleaseFailure(t *testing.T) {
	catalog := NewCatalog(SimplifiedChinese)
	got := LocalizeError(errors.New("github releases request failed: 404 Not Found"), catalog)
	want := "GitHub Releases 请求失败: 404 Not Found"
	if got != want {
		t.Fatalf("LocalizeError() = %q, want %q", got, want)
	}
}

func TestLocalizeErrorForUnsupportedShell(t *testing.T) {
	catalog := NewCatalog(SimplifiedChinese)
	got := LocalizeError(errors.New(`unsupported shell "tcsh"`), catalog)
	want := `不支持的 shell "tcsh"`
	if got != want {
		t.Fatalf("LocalizeError() = %q, want %q", got, want)
	}
}

package i18n

import (
	"os"
	"os/exec"
	"strings"
)

type localeMatch struct {
	locale     Locale
	supported  bool
	meaningful bool
}

var execCommand = exec.Command

func Detect() Locale {
	candidates := []string{os.Getenv(overrideEnvName)}
	candidates = append(candidates, detectPlatformLocales()...)
	candidates = append(candidates,
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANGUAGE"),
		os.Getenv("LANG"),
	)

	seenMeaningful := false
	for _, candidate := range candidates {
		match := parseLocale(candidate)
		if !match.meaningful {
			continue
		}
		if match.supported {
			return match.locale
		}
		seenMeaningful = true
	}
	if seenMeaningful {
		return English
	}
	return English
}

func parseLocale(raw string) localeMatch {
	value := canonicalLocale(raw)
	if value == "" {
		return localeMatch{}
	}
	switch {
	case value == "c", value == "posix":
		return localeMatch{}
	case value == "en", strings.HasPrefix(value, "en-"):
		return localeMatch{locale: English, supported: true, meaningful: true}
	case value == "zh", strings.HasPrefix(value, "zh-"), strings.Contains(value, "hans"):
		return localeMatch{locale: SimplifiedChinese, supported: true, meaningful: true}
	default:
		return localeMatch{locale: English, supported: false, meaningful: true}
	}
}

func canonicalLocale(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.Trim(value, `"'`)
	if idx := strings.IndexAny(value, ",;"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "."); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "@"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.ReplaceAll(value, "_", "-")
	return strings.ToLower(strings.TrimSpace(value))
}

func readKeyValueLocaleFiles(paths ...string) []string {
	var candidates []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE":
				candidates = append(candidates, strings.Trim(strings.TrimSpace(value), `"'`))
			}
		}
	}
	return candidates
}

func commandOutput(name string, args ...string) string {
	cmd := execCommand(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

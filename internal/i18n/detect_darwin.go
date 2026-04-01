//go:build darwin

package i18n

import "strings"

func detectPlatformLocales() []string {
	var candidates []string
	if language := readAppleLanguages(); language != "" {
		candidates = append(candidates, language)
	}
	if locale := commandOutput("defaults", "read", "-g", "AppleLocale"); locale != "" {
		candidates = append(candidates, locale)
	}
	return candidates
}

func readAppleLanguages() string {
	raw := commandOutput("defaults", "read", "-g", "AppleLanguages")
	return parseAppleLanguagesOutput(raw)
}

func parseAppleLanguagesOutput(raw string) string {
	if raw == "" {
		return ""
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "(" || line == ")" {
			continue
		}
		line = strings.TrimSuffix(line, ",")
		line = strings.Trim(line, `"'`)
		if line != "" {
			return line
		}
	}
	return ""
}

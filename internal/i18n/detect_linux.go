//go:build linux

package i18n

import (
	"os"
	"path/filepath"
	"strings"
)

func detectPlatformLocales() []string {
	home, _ := osUserHomeDir()
	paths := []string{
		filepath.Join(home, ".config", "locale.conf"),
		"/etc/locale.conf",
		"/etc/default/locale",
	}
	candidates := readKeyValueLocaleFiles(paths...)
	if output := commandOutput("locale"); output != "" {
		candidates = append(candidates, parseLocaleCommand(output)...)
	}
	return candidates
}

var osUserHomeDir = userHomeDir

func userHomeDir() (string, error) {
	return os.UserHomeDir()
}

func parseLocaleCommand(output string) []string {
	var candidates []string
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE":
			candidates = append(candidates, strings.Trim(value, `"'`))
		}
	}
	return candidates
}

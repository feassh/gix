package i18n

import (
	"errors"
	"fmt"
	"strings"
)

func LocalizeError(err error, catalog Catalog) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if message == "" {
		return ""
	}
	if localized, ok := localizeKnownError(message, catalog); ok {
		return localized
	}
	return message
}

func localizeKnownError(message string, catalog Catalog) (string, bool) {
	switch {
	case message == "current directory is not a Git repository":
		return catalog.S("error.not_git_repo"), true
	case message == "self-update repo is not configured":
		return catalog.S("error.self_update_repo_not_set"), true
	case message == "source binary path is required":
		return catalog.S("error.source_binary_required"), true
	case message == "project config requires a git repository":
		return catalog.S("error.project_config_requires_repo"), true
	case message == "empty tag":
		return catalog.S("error.empty_tag"), true
	case message == "unsupported value type":
		return catalog.S("error.unsupported_value_type"), true
	case message == "unsupported scalar type":
		return catalog.S("error.unsupported_scalar_type"), true
	}
	if arg, ok := parseSuffix(message, "invalid ai.base_url: "); ok {
		return catalog.S("error.invalid_ai_base_url", arg), true
	}
	if arg, ok := parseSuffix(message, "ai stream interrupted: "); ok {
		return catalog.S("error.ai_stream_interrupted", arg), true
	}
	if message == "ai stream ended without message content" {
		return catalog.S("error.ai_stream_no_content"), true
	}

	if arg, ok := parseQuoted(message, "unknown command "); ok {
		return catalog.S("error.unknown_command", arg), true
	}
	if arg, ok := parseQuoted(message, "unsupported shell "); ok {
		return catalog.S("error.unsupported_shell", arg), true
	}
	if arg, ok := parseQuotedWithSuffix(message, "shell ", " does not use a standalone completion file"); ok {
		return catalog.S("error.shell_no_standalone_file", arg), true
	}
	if arg, ok := parseQuotedWithSuffix(message, "release asset ", " not found"); ok {
		return catalog.S("error.release_asset_not_found", arg), true
	}
	if arg, ok := parseQuotedWithSuffix(message, "config key ", " not found"); ok {
		return catalog.S("error.config_key_not_found", arg), true
	}
	if arg, ok := parseQuoted(message, "unknown config key "); ok {
		return catalog.S("error.unknown_config_key", arg), true
	}
	if arg, ok := parseQuotedWithSuffix(message, "tag ", " is not semver"); ok {
		return catalog.S("error.tag_not_semver", arg), true
	}
	if arg, ok := parseQuoted(message, "unsupported increment "); ok {
		return catalog.S("error.unsupported_increment", arg), true
	}
	if arg, ok := parseQuoted(message, "expected boolean value for "); ok {
		return catalog.S("error.expected_bool", arg), true
	}
	if arg, ok := parseQuoted(message, "expected integer value for "); ok {
		return catalog.S("error.expected_int", arg), true
	}

	if arg, ok := parseSuffix(message, "github releases request failed: "); ok {
		return catalog.S("error.github_release_request", arg), true
	}
	if arg, ok := parseSuffix(message, "download failed: "); ok {
		return catalog.S("error.download_failed", arg), true
	}
	if arg, ok := parseSuffix(message, "checksum verification failed for "); ok {
		return catalog.S("error.checksum_failed", arg), true
	}
	if arg, ok := parseSuffix(message, "checksum for "); ok && strings.HasSuffix(message, " not found") {
		arg = strings.TrimSuffix(arg, " not found")
		return catalog.S("error.checksum_not_found", arg), true
	}
	if arg, ok := parseSuffix(message, "failed to update Windows PATH: "); ok {
		return catalog.S("error.failed_update_windows_path", localizePowerShellMessage(arg, catalog)), true
	}
	if arg, ok := parseSuffix(message, "failed to schedule binary removal: "); ok {
		return catalog.S("error.failed_schedule_remove", localizePowerShellMessage(arg, catalog)), true
	}
	if arg, ok := parseSuffix(message, "failed to schedule binary replacement: "); ok {
		return catalog.S("error.failed_schedule_replace", localizePowerShellMessage(arg, catalog)), true
	}
	if field, rest, ok := parseInvalidField(message); ok {
		return catalog.S("error.invalid_config_field", field, localizeNested(rest, catalog)), true
	}
	if field, line, rest, ok := parseInvalidTomlField(message); ok {
		return catalog.S("error.invalid_toml_field", field, line, localizeNested(rest, catalog)), true
	}
	if line, ok := parseIntSuffix(message, "invalid toml line "); ok {
		return catalog.S("error.invalid_toml_line", line), true
	}
	if usage, ok := parseSuffix(message, "usage: "); ok {
		return catalog.S("error.usage", usage), true
	}
	if group, sub, ok := parseUnknownSubcommand(message); ok {
		return catalog.S("error.unknown_subcommand", group, sub), true
	}
	if message == "cannot create a tag before the first commit" {
		return catalog.S("error.tag_before_first_commit"), true
	}

	return "", false
}

func parseQuoted(message string, prefix string) (string, bool) {
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", false
	}
	return strings.Trim(rest, `"`), true
}

func parseQuotedWithSuffix(message string, prefix string, suffix string) (string, bool) {
	if !strings.HasPrefix(message, prefix) || !strings.HasSuffix(message, suffix) {
		return "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(message, prefix), suffix)
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", false
	}
	return strings.Trim(rest, `"`), true
}

func parseSuffix(message string, prefix string) (string, bool) {
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(message, prefix)), true
}

func parseIntSuffix(message string, prefix string) (int, bool) {
	if !strings.HasPrefix(message, prefix) {
		return 0, false
	}
	var value int
	_, err := fmt.Sscanf(strings.TrimPrefix(message, prefix), "%d", &value)
	return value, err == nil
}

func parseInvalidField(message string) (string, string, bool) {
	if !strings.HasPrefix(message, "invalid ") {
		return "", "", false
	}
	rest := strings.TrimPrefix(message, "invalid ")
	parts := strings.SplitN(rest, ": ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseInvalidTomlField(message string) (string, int, string, bool) {
	if !strings.HasPrefix(message, "invalid ") {
		return "", 0, "", false
	}
	rest := strings.TrimPrefix(message, "invalid ")
	var field string
	var line int
	n, err := fmt.Sscanf(rest, "%s at line %d: ", &field, &line)
	if err != nil || n != 2 {
		return "", 0, "", false
	}
	marker := fmt.Sprintf("%s at line %d: ", field, line)
	idx := strings.Index(rest, marker)
	if idx != 0 {
		return "", 0, "", false
	}
	return field, line, strings.TrimPrefix(rest, marker), true
}

func parseUnknownSubcommand(message string) (string, string, bool) {
	if !strings.HasPrefix(message, "unknown ") || !strings.Contains(message, " subcommand ") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(message, "unknown "), " subcommand ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	sub := strings.Trim(parts[1], `"`)
	return parts[0], sub, true
}

func localizePowerShellMessage(message string, catalog Catalog) string {
	if strings.TrimSpace(message) == "PowerShell is not available" {
		return catalog.S("error.powershell_unavailable")
	}
	return message
}

func localizeNested(message string, catalog Catalog) string {
	if localized, ok := localizeKnownError(message, catalog); ok {
		return localized
	}
	return message
}

func IsLocalizedError(err error, catalog Catalog) error {
	if err == nil {
		return nil
	}
	return errors.New(LocalizeError(err, catalog))
}

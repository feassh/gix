package app

import (
	"fmt"
	"strings"

	"gix/internal/config"
	"gix/internal/i18n"
	"gix/internal/ui"
)

func (a *App) tr(key string, args ...any) string {
	return a.console.T(key, args...)
}

func (a *App) applyUIConfig(cfg config.Config) {
	a.console.SetColorEnabled(cfg.UI.Color)
}

func (a *App) localeValue(value string) string {
	if value == "" {
		return a.tr("common.none")
	}
	return value
}

func (a *App) localeForCommitSource(value string) string {
	if a.console.Locale() != i18n.SimplifiedChinese {
		return value
	}
	switch value {
	case "ai":
		return "AI"
	case "fallback":
		return "fallback"
	case "manual":
		return "手动"
	default:
		return value
	}
}

func detail(label string, value string) ui.Detail {
	return ui.Detail{Label: fmt.Sprintf("%s:", label), Value: value}
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "********"
	}
	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}

func (a *App) localizeNote(note string) string {
	switch strings.TrimSpace(note) {
	case "The current process PATH was updated for this session.":
		return a.tr("common.current_session_path")
	case "Open a new terminal, or source your shell profile to use gix everywhere.":
		return a.tr("note.open_terminal_use_gix")
	case "Open a new terminal, or source your shell profile to remove gix from PATH everywhere.":
		return a.tr("note.open_terminal_remove_gix")
	case "PATH was already clean for the gix bin directory.":
		return a.tr("label.path_already_clean")
	case "The installed gix binary was not present.":
		return a.tr("note.binary_not_present")
	case "The updated gix binary will replace the current executable after this process exits.":
		return a.tr("note.replace_after_exit")
	case "Open a new terminal window to load the updated PATH.":
		return a.tr("note.open_terminal_windows_path")
	case "The user PATH already contains the gix bin directory.":
		return a.tr("note.windows_path_contains")
	case "The user PATH does not contain the gix bin directory.":
		return a.tr("note.windows_path_not_contains")
	case "Installed bash completion.":
		return a.tr("message.completion_installed", "bash")
	case "Installed fish completion.":
		return a.tr("message.completion_installed", "fish")
	case "Installed zsh completion.":
		return a.tr("message.completion_installed", "zsh")
	case "Installed PowerShell completion.":
		return a.tr("message.completion_installed", "powershell")
	case "Removed bash completion.":
		return a.tr("message.completion_removed", "bash")
	case "Removed fish completion.":
		return a.tr("message.completion_removed", "fish")
	case "Removed zsh completion.":
		return a.tr("message.completion_removed", "zsh")
	case "Removed PowerShell completion.":
		return a.tr("message.completion_removed", "powershell")
	default:
		return note
	}
}

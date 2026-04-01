package shellcompletion

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Shell string

const (
	ShellBash       Shell = "bash"
	ShellZsh        Shell = "zsh"
	ShellFish       Shell = "fish"
	ShellPowerShell Shell = "powershell"
)

const (
	powerShellBlockStart = "# >>> gix completion >>>"
	powerShellBlockEnd   = "# <<< gix completion <<<"
	zshBlockStart        = "# >>> gix zsh completion >>>"
	zshBlockEnd          = "# <<< gix zsh completion <<<"
)

type Result struct {
	Shell        Shell
	UpdatedFiles []string
	Notes        []string
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Install(shellName string) (Result, error) {
	shell, err := detectOrResolveShell(shellName)
	if err != nil {
		return Result{}, err
	}
	switch shell {
	case ShellBash, ShellFish:
		path, err := completionScriptPath(shell)
		if err != nil {
			return Result{}, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(path, []byte(Script(shell)), 0o644); err != nil {
			return Result{}, err
		}
		return Result{
			Shell:        shell,
			UpdatedFiles: []string{path},
			Notes:        []string{fmt.Sprintf("Installed %s completion.", shell)},
		}, nil
	case ShellZsh:
		return installZsh()
	case ShellPowerShell:
		return installPowerShell()
	default:
		return Result{}, fmt.Errorf("unsupported shell %q", shell)
	}
}

func (m *Manager) Uninstall(shellName string) (Result, error) {
	shell, err := detectOrResolveShell(shellName)
	if err != nil {
		return Result{}, err
	}
	switch shell {
	case ShellBash, ShellFish:
		path, err := completionScriptPath(shell)
		if err != nil {
			return Result{}, err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return Result{}, err
		}
		return Result{
			Shell:        shell,
			UpdatedFiles: []string{path},
			Notes:        []string{fmt.Sprintf("Removed %s completion.", shell)},
		}, nil
	case ShellZsh:
		return uninstallZsh()
	case ShellPowerShell:
		return uninstallPowerShell()
	default:
		return Result{}, fmt.Errorf("unsupported shell %q", shell)
	}
}

func Script(shell Shell) string {
	switch shell {
	case ShellBash:
		return bashScript()
	case ShellZsh:
		return zshScript()
	case ShellFish:
		return fishScript()
	case ShellPowerShell:
		return powerShellScript()
	default:
		return ""
	}
}

func detectOrResolveShell(shellName string) (Shell, error) {
	if strings.TrimSpace(shellName) != "" {
		return normalizeShell(shellName)
	}
	return DetectShell()
}

func DetectShell() (Shell, error) {
	if override := strings.TrimSpace(os.Getenv("GIX_SHELL")); override != "" {
		return normalizeShell(override)
	}
	if runtime.GOOS == "windows" {
		return ShellPowerShell, nil
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell != "" {
		return normalizeShell(shell)
	}
	if runtime.GOOS == "darwin" {
		return ShellZsh, nil
	}
	return ShellBash, nil
}

func normalizeShell(value string) (Shell, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bash":
		return ShellBash, nil
	case "zsh":
		return ShellZsh, nil
	case "fish":
		return ShellFish, nil
	case "powershell", "pwsh", "ps":
		return ShellPowerShell, nil
	default:
		return "", fmt.Errorf("unsupported shell %q", value)
	}
}

func completionScriptPath(shell Shell) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch shell {
	case ShellBash:
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "gix"), nil
	case ShellZsh:
		return filepath.Join(home, ".zsh", "completions", "_gix"), nil
	case ShellFish:
		return filepath.Join(home, ".config", "fish", "completions", "gix.fish"), nil
	default:
		return "", fmt.Errorf("shell %q does not use a standalone completion file", shell)
	}
}

func powerShellProfilePaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		}, nil
	}
	return []string{filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")}, nil
}

func installPowerShell() (Result, error) {
	paths, err := powerShellProfilePaths()
	if err != nil {
		return Result{}, err
	}
	block := powerShellManagedBlock()
	var updated []string
	for _, path := range paths {
		changed, err := ensureManagedBlock(path, block, powerShellBlockStart, powerShellBlockEnd)
		if err != nil {
			return Result{}, err
		}
		if changed {
			updated = append(updated, path)
		}
	}
	return Result{
		Shell:        ShellPowerShell,
		UpdatedFiles: updated,
		Notes:        []string{"Installed PowerShell completion."},
	}, nil
}

func installZsh() (Result, error) {
	path, err := completionScriptPath(ShellZsh)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(path, []byte(Script(ShellZsh)), 0o644); err != nil {
		return Result{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	zshrc := filepath.Join(home, ".zshrc")
	changed, err := ensureManagedBlock(zshrc, zshManagedBlock(), zshBlockStart, zshBlockEnd)
	if err != nil {
		return Result{}, err
	}
	updated := []string{path}
	if changed {
		updated = append(updated, zshrc)
	}
	return Result{
		Shell:        ShellZsh,
		UpdatedFiles: updated,
		Notes:        []string{"Installed zsh completion."},
	}, nil
}

func uninstallPowerShell() (Result, error) {
	paths, err := powerShellProfilePaths()
	if err != nil {
		return Result{}, err
	}
	var updated []string
	for _, path := range paths {
		changed, err := removeManagedBlock(path, powerShellBlockStart, powerShellBlockEnd)
		if err != nil {
			return Result{}, err
		}
		if changed {
			updated = append(updated, path)
		}
	}
	return Result{
		Shell:        ShellPowerShell,
		UpdatedFiles: updated,
		Notes:        []string{"Removed PowerShell completion."},
	}, nil
}

func uninstallZsh() (Result, error) {
	path, err := completionScriptPath(ShellZsh)
	if err != nil {
		return Result{}, err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	zshrc := filepath.Join(home, ".zshrc")
	changed, err := removeManagedBlock(zshrc, zshBlockStart, zshBlockEnd)
	if err != nil {
		return Result{}, err
	}
	updated := []string{path}
	if changed {
		updated = append(updated, zshrc)
	}
	return Result{
		Shell:        ShellZsh,
		UpdatedFiles: updated,
		Notes:        []string{"Removed zsh completion."},
	}, nil
}

func powerShellManagedBlock() string {
	return strings.Join([]string{
		powerShellBlockStart,
		powerShellScript(),
		powerShellBlockEnd,
		"",
	}, "\n")
}

func zshManagedBlock() string {
	return strings.Join([]string{
		zshBlockStart,
		"if [[ \" ${fpath[*]} \" != *\" $HOME/.zsh/completions \"* ]]; then",
		"  fpath=(\"$HOME/.zsh/completions\" $fpath)",
		"fi",
		"autoload -Uz compinit",
		"compinit",
		zshBlockEnd,
		"",
	}, "\n")
}

func ensureManagedBlock(path string, block string, startMarker string, endMarker string) (bool, error) {
	var current string
	if data, err := os.ReadFile(path); err == nil {
		current = string(data)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if strings.Contains(current, startMarker) && strings.Contains(current, endMarker) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimRight(current, "\n"))
	if strings.TrimSpace(current) != "" {
		builder.WriteString("\n\n")
	}
	builder.WriteString(block)
	return true, os.WriteFile(path, []byte(builder.String()), 0o644)
}

func removeManagedBlock(path string, startMarker string, endMarker string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	current := string(data)
	start := strings.Index(current, startMarker)
	end := strings.Index(current, endMarker)
	if start == -1 || end == -1 || end < start {
		return false, nil
	}
	end += len(endMarker)
	updated := current[:start] + current[end:]
	updated = strings.TrimLeft(updated, "\n")
	updated = normalizeSpacing(updated)
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

func normalizeSpacing(value string) string {
	lines := strings.Split(value, "\n")
	var filtered []string
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
			filtered = append(filtered, "")
			continue
		}
		blank = false
		filtered = append(filtered, line)
	}
	return strings.TrimLeft(strings.Join(filtered, "\n"), "\n")
}

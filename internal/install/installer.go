package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	blockStart = "# >>> gix install >>>"
	blockEnd   = "# <<< gix install <<<"
)

type Result struct {
	SourcePath         string
	TargetPath         string
	PathUpdated        bool
	PathAlreadyPresent bool
	UpdatedFiles       []string
	Notes              []string
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Install(ctx context.Context) (Result, error) {
	sourcePath, err := os.Executable()
	if err != nil {
		return Result{}, err
	}
	return m.InstallFromPath(ctx, sourcePath, true)
}

func (m *Manager) InstallFromPath(ctx context.Context, sourcePath string, ensurePATH bool) (Result, error) {
	runningExecutable, err := os.Executable()
	if err == nil {
		runningExecutable, err = filepath.EvalSymlinks(runningExecutable)
		if err != nil {
			runningExecutable = filepath.Clean(runningExecutable)
		}
	}
	installSource := sourcePath
	if installSource == "" {
		return Result{}, fmt.Errorf("source binary path is required")
	}
	installSource, err = filepath.EvalSymlinks(installSource)
	if err != nil {
		installSource = filepath.Clean(installSource)
	}

	targetDir, err := m.userBinDir()
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return Result{}, err
	}

	targetPath := filepath.Join(targetDir, executableName())
	replaceNotes, err := installBinary(ctx, installSource, targetPath, runningExecutable)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		SourcePath: installSource,
		TargetPath: targetPath,
	}
	result.Notes = append(result.Notes, replaceNotes...)

	if ensurePATH && addCurrentProcessPath(targetDir) {
		result.Notes = append(result.Notes, "The current process PATH was updated for this session.")
	}

	if ensurePATH {
		switch runtime.GOOS {
		case "windows":
			updated, alreadyPresent, notes, err := ensureWindowsUserPath(ctx, targetDir)
			if err != nil {
				return Result{}, err
			}
			result.PathUpdated = updated
			result.PathAlreadyPresent = alreadyPresent
			result.Notes = append(result.Notes, notes...)
		default:
			updatedFiles, alreadyPresent, err := ensureUnixUserPath(targetDir)
			if err != nil {
				return Result{}, err
			}
			result.UpdatedFiles = updatedFiles
			result.PathUpdated = len(updatedFiles) > 0
			result.PathAlreadyPresent = alreadyPresent
			if len(updatedFiles) > 0 {
				result.Notes = append(result.Notes, "Open a new terminal, or source your shell profile to use gix everywhere.")
			}
		}
	}

	return result, nil
}

func (m *Manager) Uninstall(ctx context.Context) (Result, error) {
	targetDir, err := m.userBinDir()
	if err != nil {
		return Result{}, err
	}
	targetPath := filepath.Join(targetDir, executableName())
	result := Result{
		TargetPath: targetPath,
	}

	sourcePath, err := os.Executable()
	if err == nil {
		sourcePath, evalErr := filepath.EvalSymlinks(sourcePath)
		if evalErr == nil {
			result.SourcePath = sourcePath
		} else {
			result.SourcePath = filepath.Clean(sourcePath)
		}
	}

	currentPathRemoved := false
	switch runtime.GOOS {
	case "windows":
		updated, alreadyPresent, notes, err := removeWindowsUserPath(ctx, targetDir)
		if err != nil {
			return Result{}, err
		}
		result.PathUpdated = updated
		result.PathAlreadyPresent = alreadyPresent
		result.Notes = append(result.Notes, notes...)
	default:
		updatedFiles, alreadyPresent, err := removeUnixUserPath(targetDir)
		if err != nil {
			return Result{}, err
		}
		result.UpdatedFiles = updatedFiles
		result.PathUpdated = len(updatedFiles) > 0
		result.PathAlreadyPresent = alreadyPresent
		if len(updatedFiles) > 0 {
			result.Notes = append(result.Notes, "Open a new terminal, or source your shell profile to remove gix from PATH everywhere.")
		}
	}

	if removeCurrentProcessPath(targetDir) {
		currentPathRemoved = true
		result.Notes = append(result.Notes, "The current process PATH was updated for this session.")
	}
	if !result.PathUpdated && !currentPathRemoved {
		if runtime.GOOS == "windows" {
			if result.PathAlreadyPresent {
				result.Notes = append(result.Notes, "PATH was already clean for the gix bin directory.")
			}
		} else {
			result.Notes = append(result.Notes, "PATH was already clean for the gix bin directory.")
		}
	}

	removed, removeNotes, err := removeInstalledBinary(ctx, result.SourcePath, targetPath)
	if err != nil {
		return Result{}, err
	}
	if !removed {
		result.Notes = append(result.Notes, "The installed gix binary was not present.")
	}
	result.Notes = append(result.Notes, removeNotes...)
	return result, nil
}

func (m *Manager) userBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "bin"), nil
	default:
		return filepath.Join(home, ".local", "bin"), nil
	}
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "gix.exe"
	}
	return "gix"
}

func (m *Manager) TargetPath() (string, error) {
	targetDir, err := m.userBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(targetDir, executableName()), nil
}

func installBinary(ctx context.Context, sourcePath string, targetPath string, runningExecutable string) ([]string, error) {
	if runtime.GOOS == "windows" && runningExecutable != "" && filepath.Clean(runningExecutable) == filepath.Clean(targetPath) && filepath.Clean(sourcePath) != filepath.Clean(targetPath) {
		stagingPath := targetPath + ".new"
		if err := copyExecutable(sourcePath, stagingPath); err != nil {
			return nil, err
		}
		if err := scheduleWindowsReplace(ctx, stagingPath, targetPath); err != nil {
			return nil, err
		}
		return []string{"The updated gix binary will replace the current executable after this process exits."}, nil
	}
	return nil, copyExecutable(sourcePath, targetPath)
}

func copyExecutable(sourcePath string, targetPath string) error {
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	if filepath.Clean(sourceAbs) == filepath.Clean(targetAbs) {
		return ensureExecutableMode(targetPath)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	tempPath := targetPath + ".tmp"
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(tempFile, sourceFile); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0o755); err != nil {
			_ = os.Remove(tempPath)
			return err
		}
	}

	_ = os.Remove(targetPath)
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return ensureExecutableMode(targetPath)
}

func ensureExecutableMode(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o755)
}

func addCurrentProcessPath(dir string) bool {
	parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, part := range parts {
		if filepath.Clean(part) == filepath.Clean(dir) {
			return false
		}
	}
	newPath := dir
	if current := os.Getenv("PATH"); current != "" {
		newPath = dir + string(os.PathListSeparator) + current
	}
	_ = os.Setenv("PATH", newPath)
	return true
}

func removeCurrentProcessPath(dir string) bool {
	parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	filtered := make([]string, 0, len(parts))
	removed := false
	for _, part := range parts {
		if filepath.Clean(part) == filepath.Clean(dir) {
			removed = true
			continue
		}
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	if removed {
		_ = os.Setenv("PATH", strings.Join(filtered, string(os.PathListSeparator)))
	}
	return removed
}

func ensureUnixUserPath(binDir string) ([]string, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false, err
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	targets := unixProfileTargets(runtime.GOOS, shell, home)
	alreadyPresent := pathContains(binDir, os.Getenv("PATH"))

	var updated []string
	for _, target := range targets {
		changed, err := ensureManagedBlock(target.Path, managedBlock(binDir, target.Kind))
		if err != nil {
			return nil, false, err
		}
		if changed {
			updated = append(updated, target.Path)
		}
	}
	return updated, alreadyPresent, nil
}

func removeUnixUserPath(binDir string) ([]string, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false, err
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	targets := unixProfileTargets(runtime.GOOS, shell, home)
	alreadyPresent := pathContains(binDir, os.Getenv("PATH"))

	var updated []string
	for _, target := range targets {
		changed, err := removeManagedBlock(target.Path)
		if err != nil {
			return nil, false, err
		}
		if changed {
			updated = append(updated, target.Path)
		}
	}
	return updated, alreadyPresent, nil
}

type profileTarget struct {
	Path string
	Kind string
}

func unixProfileTargets(goos string, shell string, home string) []profileTarget {
	switch {
	case shell == "fish":
		return []profileTarget{{Path: filepath.Join(home, ".config", "fish", "config.fish"), Kind: "fish"}}
	case shell == "zsh" || goos == "darwin":
		return []profileTarget{{Path: filepath.Join(home, ".zprofile"), Kind: "posix"}}
	case shell == "bash":
		if fileExists(filepath.Join(home, ".bash_profile")) {
			return []profileTarget{{Path: filepath.Join(home, ".bash_profile"), Kind: "posix"}}
		}
		return []profileTarget{{Path: filepath.Join(home, ".profile"), Kind: "posix"}}
	default:
		return []profileTarget{{Path: filepath.Join(home, ".profile"), Kind: "posix"}}
	}
}

func managedBlock(binDir string, kind string) string {
	if kind == "fish" {
		return fishManagedBlock(binDir)
	}
	return unixManagedBlock(binDir)
}

func unixManagedBlock(binDir string) string {
	escaped := shellPath(binDir)
	return strings.Join([]string{
		blockStart,
		fmt.Sprintf("case \":$PATH:\" in *\":%s:\"*) ;; *) export PATH=\"%s:$PATH\" ;; esac", escaped, escaped),
		blockEnd,
		"",
	}, "\n")
}

func fishManagedBlock(binDir string) string {
	escaped := shellPath(binDir)
	return strings.Join([]string{
		blockStart,
		fmt.Sprintf("if not contains \"%s\" $PATH", escaped),
		fmt.Sprintf("    set -gx PATH \"%s\" $PATH", escaped),
		"end",
		blockEnd,
		"",
	}, "\n")
}

func shellPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "$HOME" + strings.TrimPrefix(path, home)
	}
	return path
}

func ensureManagedBlock(path string, block string) (bool, error) {
	var current string
	if data, err := os.ReadFile(path); err == nil {
		current = string(data)
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if strings.Contains(current, blockStart) && strings.Contains(current, blockEnd) {
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
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func removeManagedBlock(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	current := string(data)
	start := strings.Index(current, blockStart)
	end := strings.Index(current, blockEnd)
	if start == -1 || end == -1 || end < start {
		return false, nil
	}
	end += len(blockEnd)
	updated := current[:start] + current[end:]
	updated = strings.TrimLeft(updated, "\n")
	updated = normalizeSpacing(updated)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func ensureWindowsUserPath(ctx context.Context, binDir string) (bool, bool, []string, error) {
	script := fmt.Sprintf(
		"$dir = '%s'; "+
			"$path = [Environment]::GetEnvironmentVariable('Path', 'User'); "+
			"if ([string]::IsNullOrWhiteSpace($path)) { $path = '' }; "+
			"$parts = @(); "+
			"if ($path -ne '') { $parts = $path -split ';' | Where-Object { $_ -ne '' } }; "+
			"if ($parts -contains $dir) { exit 10 }; "+
			"$newPath = if ($path -eq '') { $dir } else { $dir + ';' + $path }; "+
			"[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')",
		powerShellQuote(binDir),
	)
	return runWindowsPathScript(ctx, script, "Open a new terminal window to load the updated PATH.", "The user PATH already contains the gix bin directory.")
}

func removeWindowsUserPath(ctx context.Context, binDir string) (bool, bool, []string, error) {
	script := fmt.Sprintf(
		"$dir = '%s'; "+
			"$path = [Environment]::GetEnvironmentVariable('Path', 'User'); "+
			"if ([string]::IsNullOrWhiteSpace($path)) { exit 10 }; "+
			"$parts = $path -split ';' | Where-Object { $_ -ne '' }; "+
			"if (-not ($parts -contains $dir)) { exit 10 }; "+
			"$filtered = @($parts | Where-Object { $_ -ne $dir }); "+
			"$newPath = [string]::Join(';', $filtered); "+
			"[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')",
		powerShellQuote(binDir),
	)
	return runWindowsPathScript(ctx, script, "Open a new terminal window to load the updated PATH.", "The user PATH does not contain the gix bin directory.")
}

func runWindowsPathScript(ctx context.Context, script string, successNote string, alreadyNote string) (bool, bool, []string, error) {
	for _, shell := range []string{"powershell", "pwsh"} {
		cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return true, false, []string{successNote}, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 10 {
			return false, true, []string{alreadyNote}, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			continue
		}
		return false, false, nil, fmt.Errorf("failed to update Windows PATH: %s", strings.TrimSpace(string(output)))
	}
	return false, false, nil, fmt.Errorf("failed to update Windows PATH: PowerShell is not available")
}

func removeInstalledBinary(ctx context.Context, sourcePath string, targetPath string) (bool, []string, error) {
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}

	sameBinary := sourcePath != "" && filepath.Clean(sourcePath) == filepath.Clean(targetPath)
	if runtime.GOOS == "windows" && sameBinary {
		if err := scheduleWindowsDelete(ctx, targetPath); err != nil {
			return true, nil, err
		}
		return true, []string{"The installed gix binary will be removed after this process exits."}, nil
	}

	if err := os.Remove(targetPath); err != nil {
		return true, nil, err
	}
	return true, nil, nil
}

func scheduleWindowsDelete(ctx context.Context, targetPath string) error {
	command := fmt.Sprintf(`Start-Process cmd -WindowStyle Hidden -ArgumentList '/C ping 127.0.0.1 -n 2 >NUL & del /F /Q "%s"'`, targetPath)
	for _, shell := range []string{"powershell", "pwsh"} {
		cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
		if err := cmd.Run(); err == nil {
			return nil
		} else if errors.Is(err, exec.ErrNotFound) {
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("failed to schedule binary removal: PowerShell is not available")
}

func scheduleWindowsReplace(ctx context.Context, sourcePath string, targetPath string) error {
	command := fmt.Sprintf(`Start-Process cmd -WindowStyle Hidden -ArgumentList '/C ping 127.0.0.1 -n 2 >NUL & copy /Y "%s" "%s" >NUL & del /F /Q "%s"'`, sourcePath, targetPath, sourcePath)
	for _, shell := range []string{"powershell", "pwsh"} {
		cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
		if err := cmd.Run(); err == nil {
			return nil
		} else if errors.Is(err, exec.ErrNotFound) {
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("failed to schedule binary replacement: PowerShell is not available")
}

func powerShellQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func pathContains(dir string, rawPath string) bool {
	for _, part := range strings.Split(rawPath, string(os.PathListSeparator)) {
		if filepath.Clean(part) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

package shellcompletion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectShellDefaults(t *testing.T) {
	t.Setenv("GIX_SHELL", "zsh")
	shell, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() error = %v", err)
	}
	if shell != ShellZsh {
		t.Fatalf("expected zsh, got %s", shell)
	}
}

func TestInstallAndUninstallZshCompletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix completion path test")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager := NewManager()
	result, err := manager.Install("zsh")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	target := filepath.Join(home, ".zsh", "completions", "_gix")
	zshrc := filepath.Join(home, ".zshrc")
	if len(result.UpdatedFiles) != 2 || result.UpdatedFiles[0] != target || result.UpdatedFiles[1] != zshrc {
		t.Fatalf("unexpected updated files: %v", result.UpdatedFiles)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "compdef _gix_completion gix") {
		t.Fatalf("expected zsh completion script")
	}
	rcData, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("ReadFile(zshrc) error = %v", err)
	}
	if !strings.Contains(string(rcData), "fpath=(\"$HOME/.zsh/completions\" $fpath)") {
		t.Fatalf("expected zshrc to load completion directory")
	}

	uninstallResult, err := manager.Uninstall("zsh")
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if len(uninstallResult.UpdatedFiles) != 2 || uninstallResult.UpdatedFiles[0] != target || uninstallResult.UpdatedFiles[1] != zshrc {
		t.Fatalf("unexpected uninstall files: %v", uninstallResult.UpdatedFiles)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected completion file to be removed, err=%v", err)
	}
	rcData, err = os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("ReadFile(zshrc) after uninstall error = %v", err)
	}
	if strings.Contains(string(rcData), zshBlockStart) {
		t.Fatalf("expected zshrc managed block to be removed")
	}
}

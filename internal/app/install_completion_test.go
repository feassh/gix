package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAndUninstallManageCompletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix completion integration test")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("GIX_SHELL", "zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	app := New(bytes.NewBuffer(nil), &bytes.Buffer{}, &bytes.Buffer{})
	if err := app.Run(context.Background(), []string{"install"}); err != nil {
		t.Fatalf("install: %v", err)
	}

	completionPath := filepath.Join(home, ".zsh", "completions", "_gix")
	if _, err := os.Stat(completionPath); err != nil {
		t.Fatalf("expected completion file to be installed: %v", err)
	}

	if err := app.Run(context.Background(), []string{"uninstall"}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(completionPath); !os.IsNotExist(err) {
		t.Fatalf("expected completion file to be removed, err=%v", err)
	}
}

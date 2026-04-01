package install

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestManagedBlockForFish(t *testing.T) {
	block := managedBlock("/tmp/gix-bin", "fish")
	if !strings.Contains(block, "set -gx PATH") {
		t.Fatalf("expected fish PATH syntax, got %q", block)
	}
}

func TestEnsureManagedBlockIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".profile")
	changed, err := ensureManagedBlock(path, managedBlock("/tmp/gix-bin", "posix"))
	if err != nil {
		t.Fatalf("first ensureManagedBlock: %v", err)
	}
	if !changed {
		t.Fatalf("expected first write to change file")
	}

	changed, err = ensureManagedBlock(path, managedBlock("/tmp/gix-bin", "posix"))
	if err != nil {
		t.Fatalf("second ensureManagedBlock: %v", err)
	}
	if changed {
		t.Fatalf("expected second write to be idempotent")
	}
}

func TestInstallCopiesBinaryAndUpdatesProfiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-style profile test")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	manager := NewManager()
	result, err := manager.Install(context.Background())
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	target := filepath.Join(home, ".local", "bin", "gix")
	if result.TargetPath != target {
		t.Fatalf("unexpected target path: got %s want %s", result.TargetPath, target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected target binary: %v", err)
	}
	profilePath := filepath.Join(home, ".zprofile")
	profileContent, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if !strings.Contains(string(profileContent), blockStart) {
		t.Fatalf("expected managed PATH block in .profile")
	}
	if !strings.Contains(string(profileContent), "case \":$PATH:\"") {
		t.Fatalf("expected PATH guard in managed block")
	}
	if !strings.Contains(os.Getenv("PATH"), filepath.Join(home, ".local", "bin")) {
		t.Fatalf("expected current PATH to include installed bin")
	}
}

func TestUninstallRemovesBinaryAndManagedBlock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-style profile test")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	manager := NewManager()
	if _, err := manager.Install(context.Background()); err != nil {
		t.Fatalf("install: %v", err)
	}

	result, err := manager.Uninstall(context.Background())
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	if _, err := os.Stat(result.TargetPath); !os.IsNotExist(err) {
		t.Fatalf("expected installed binary to be removed, err=%v", err)
	}
	profilePath := filepath.Join(home, ".zprofile")
	profileContent, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if strings.Contains(string(profileContent), blockStart) {
		t.Fatalf("expected managed PATH block to be removed")
	}
	if strings.Contains(os.Getenv("PATH"), filepath.Join(home, ".local", "bin")) {
		t.Fatalf("expected current PATH to remove installed bin")
	}
}

func TestUnixProfileTargetsUseSingleCanonicalFile(t *testing.T) {
	home := t.TempDir()

	targets := unixProfileTargets("linux", "bash", home)
	if len(targets) != 1 || targets[0].Path != filepath.Join(home, ".profile") {
		t.Fatalf("unexpected bash targets: %+v", targets)
	}

	targets = unixProfileTargets("darwin", "zsh", home)
	if len(targets) != 1 || targets[0].Path != filepath.Join(home, ".zprofile") {
		t.Fatalf("unexpected zsh targets: %+v", targets)
	}

	targets = unixProfileTargets("linux", "fish", home)
	if len(targets) != 1 || targets[0].Path != filepath.Join(home, ".config", "fish", "config.fish") {
		t.Fatalf("unexpected fish targets: %+v", targets)
	}
}

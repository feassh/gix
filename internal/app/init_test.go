package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesGitAndGixConfig(t *testing.T) {
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()
	t.Setenv("GIX_LANG", "en")

	stdout := &bytes.Buffer{}
	app := New(bytes.NewBuffer(nil), stdout, &bytes.Buffer{})

	if err := app.Run(context.Background(), []string{"init"}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, ".git"))
	assertFileExists(t, filepath.Join(dir, ".git", "gix.toml"))
	if !strings.Contains(stdout.String(), "Initialized Git repository.") {
		t.Fatalf("expected git init message, got %q", stdout.String())
	}
}

func TestInitOnlyInitializesGixWhenGitExists(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	restore := chdir(t, dir)
	defer restore()
	t.Setenv("GIX_LANG", "en")

	stdout := &bytes.Buffer{}
	app := New(bytes.NewBuffer(nil), stdout, &bytes.Buffer{})

	if err := app.Run(context.Background(), []string{"init"}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, ".git", "gix.toml"))
	if !strings.Contains(stdout.String(), "Git repository already exists. Initializing gix project config.") {
		t.Fatalf("expected gix init message, got %q", stdout.String())
	}
}

func TestInitNoopsWhenAlreadyInitialized(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	restore := chdir(t, dir)
	defer restore()
	t.Setenv("GIX_LANG", "en")

	stdout := &bytes.Buffer{}
	app := New(bytes.NewBuffer(nil), stdout, &bytes.Buffer{})

	if err := app.Run(context.Background(), []string{"config", "init"}); err != nil {
		t.Fatalf("run config init: %v", err)
	}
	stdout.Reset()

	if err := app.Run(context.Background(), []string{"init"}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	if !strings.Contains(stdout.String(), "Git and gix are already initialized. Nothing to do.") {
		t.Fatalf("expected noop message, got %q", stdout.String())
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

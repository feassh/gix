package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRootCompletionPrefix(t *testing.T) {
	app := New(bytes.NewBuffer(nil), &bytes.Buffer{}, &bytes.Buffer{})
	got := app.complete(context.Background(), completionContext{args: nil, toComplete: "a"})
	want := []string{"add", "ac", "acp"}
	assertEqualStrings(t, got, want)
}

func TestCommitCompletionValues(t *testing.T) {
	app := New(bytes.NewBuffer(nil), &bytes.Buffer{}, &bytes.Buffer{})
	got := app.complete(context.Background(), completionContext{args: []string{"commit", "--lang"}, toComplete: ""})
	want := []string{"en", "zh"}
	assertEqualStrings(t, got, want)
}

func TestDynamicGitCompletion(t *testing.T) {
	repo := initCompletionRepo(t)
	restore := chdir(t, repo)
	defer restore()

	app := New(bytes.NewBuffer(nil), &bytes.Buffer{}, &bytes.Buffer{})
	ctx := context.Background()

	remoteCompletions := app.complete(ctx, completionContext{args: []string{"push", "--remote"}, toComplete: "o"})
	assertEqualStrings(t, remoteCompletions, []string{"origin"})

	branchCompletions := app.complete(ctx, completionContext{args: []string{"branch", "switch"}, toComplete: "f"})
	assertEqualStrings(t, branchCompletions, []string{"feature/completion"})

	tagCompletions := app.complete(ctx, completionContext{args: []string{"tag", "checkout"}, toComplete: "v"})
	assertEqualStrings(t, tagCompletions, []string{"v1.2.3"})
}

func initCompletionRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "gix-test")
	runGit(t, dir, "config", "user.email", "gix@example.com")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "hello.txt")
	runGit(t, dir, "commit", "-m", "feat: initial commit")
	runGit(t, dir, "branch", "feature/completion")
	runGit(t, dir, "tag", "v1.2.3")
	runGit(t, dir, "remote", "add", "origin", "git@example.com:origin/repo.git")
	runGit(t, dir, "remote", "add", "upstream", "git@example.com:upstream/repo.git")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore chdir: %v", err)
		}
	}
}

func assertEqualStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value mismatch: got %v want %v", got, want)
		}
	}
}

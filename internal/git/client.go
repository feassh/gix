package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Client struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type Remote struct {
	Name string
	URL  string
}

type StatusSummary struct {
	RepoRoot       string
	CurrentBranch  string
	Upstream       string
	Ahead          int
	Behind         int
	StagedCount    int
	UnstagedCount  int
	UntrackedCount int
	LatestTag      string
	DefaultRemote  string
}

func NewClient(stdin io.Reader, stdout io.Writer, stderr io.Writer) *Client {
	return &Client{stdin: stdin, stdout: stdout, stderr: stderr}
}

func (c *Client) EnsureRepo(ctx context.Context) error {
	_, err := c.runQuiet(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("current directory is not a Git repository")
	}
	return nil
}

func (c *Client) RepoRoot(ctx context.Context) (string, error) {
	out, err := c.runQuiet(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("current directory is not a Git repository")
	}
	return strings.TrimSpace(out), nil
}

func (c *Client) Init(ctx context.Context) error {
	_, err := c.runQuiet(ctx, "init")
	return err
}

func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (c *Client) HasRemote(ctx context.Context, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := c.runQuiet(ctx, "remote", "get-url", name)
	return err == nil
}

func (c *Client) ListRemotes(ctx context.Context) ([]Remote, error) {
	names, err := c.run(ctx, "remote")
	if err != nil {
		return nil, err
	}
	var remotes []Remote
	for _, name := range strings.Fields(names) {
		url, err := c.run(ctx, "remote", "get-url", name)
		if err != nil {
			return nil, err
		}
		remotes = append(remotes, Remote{Name: name, URL: strings.TrimSpace(url)})
	}
	return remotes, nil
}

func (c *Client) DetectMainBranch(ctx context.Context) (string, error) {
	out, err := c.runQuiet(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		raw := strings.TrimSpace(out)
		parts := strings.Split(raw, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}
	for _, branch := range []string{"main", "master"} {
		if _, err := c.runQuiet(ctx, "rev-parse", "--verify", branch); err == nil {
			return branch, nil
		}
	}
	return c.CurrentBranch(ctx)
}

func (c *Client) UpstreamRef(ctx context.Context) (string, error) {
	out, err := c.runQuiet(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (c *Client) Status(ctx context.Context, defaultRemote string) (StatusSummary, error) {
	root, err := c.RepoRoot(ctx)
	if err != nil {
		return StatusSummary{}, err
	}
	branch, err := c.CurrentBranch(ctx)
	if err != nil {
		return StatusSummary{}, err
	}
	upstream, err := c.UpstreamRef(ctx)
	if err != nil {
		return StatusSummary{}, err
	}
	ahead, behind := 0, 0
	if upstream != "" {
		out, err := c.run(ctx, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		if err == nil {
			parts := strings.Fields(strings.TrimSpace(out))
			if len(parts) == 2 {
				ahead, _ = strconv.Atoi(parts[0])
				behind, _ = strconv.Atoi(parts[1])
			}
		}
	}
	staged, _ := c.ListChangedFiles(ctx, true)
	unstaged, _ := c.ListChangedFiles(ctx, false)
	untracked, _ := c.UntrackedFiles(ctx)
	latestTag, _ := c.LatestTag(ctx)
	return StatusSummary{
		RepoRoot:       root,
		CurrentBranch:  branch,
		Upstream:       upstream,
		Ahead:          ahead,
		Behind:         behind,
		StagedCount:    len(staged),
		UnstagedCount:  len(unstaged),
		UntrackedCount: len(untracked),
		LatestTag:      latestTag,
		DefaultRemote:  defaultRemote,
	}, nil
}

func (c *Client) ListChangedFiles(ctx context.Context, cached bool) ([]string, error) {
	args := []string{"diff", "--name-only"}
	if cached {
		args = append(args, "--cached")
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}

func (c *Client) UntrackedFiles(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}

func (c *Client) StagedDiff(ctx context.Context) (string, error) {
	return c.run(ctx, "diff", "--cached", "--minimal")
}

func (c *Client) Add(ctx context.Context, all bool, update bool, patch bool, paths []string) error {
	args := []string{"add"}
	switch {
	case patch:
		args = append(args, "-p")
	case all:
		args = append(args, "-A")
	case update:
		args = append(args, "-u")
	default:
		if len(paths) == 0 {
			args = append(args, ".")
		}
	}
	args = append(args, paths...)
	if patch {
		return c.runInteractive(ctx, args...)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) Commit(ctx context.Context, message string, amend bool) error {
	args := []string{"commit"}
	if amend {
		args = append(args, "--amend")
	}
	subject, body := splitCommitMessage(message)
	args = append(args, "-m", subject)
	if body != "" {
		args = append(args, "-m", body)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) Push(ctx context.Context, remote string, branch string, force bool, setUpstream bool, tags bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if setUpstream {
		args = append(args, "--set-upstream")
	}
	if tags {
		args = append(args, "--tags")
	}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) Fetch(ctx context.Context, remote string) error {
	args := []string{"fetch"}
	if remote != "" {
		args = append(args, remote)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) Merge(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "merge", ref)
	return err
}

func (c *Client) Rebase(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "rebase", ref)
	return err
}

func (c *Client) ListTags(ctx context.Context, sortBy string) ([]string, error) {
	sortArg := "--sort=-v:refname"
	if strings.EqualFold(sortBy, "date") {
		sortArg = "--sort=-creatordate"
	}
	out, err := c.run(ctx, "tag", sortArg)
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}

func (c *Client) LatestTag(ctx context.Context) (string, error) {
	out, err := c.runQuiet(ctx, "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (c *Client) HeadExists(ctx context.Context) bool {
	_, err := c.runQuiet(ctx, "rev-parse", "--verify", "HEAD")
	return err == nil
}

func (c *Client) CreateTag(ctx context.Context, name string, message string, ref string, annotated bool) error {
	args := []string{"tag"}
	if annotated {
		if message == "" {
			message = name
		}
		args = append(args, "-a", name, "-m", message)
	} else {
		args = append(args, name)
	}
	if ref != "" {
		args = append(args, ref)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) PushTag(ctx context.Context, remote string, name string, all bool) error {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
	}
	if all {
		args = append(args, "--tags")
	} else if name != "" {
		args = append(args, name)
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) DeleteTag(ctx context.Context, name string, remote bool, remoteName string) error {
	if !remote {
		_, err := c.run(ctx, "tag", "-d", name)
		return err
	}
	target := remoteName
	if target == "" {
		target = "origin"
	}
	_, err := c.run(ctx, "push", target, "--delete", name)
	return err
}

func (c *Client) Checkout(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "checkout", ref)
	return err
}

func (c *Client) BranchFromRef(ctx context.Context, ref string, name string) error {
	_, err := c.run(ctx, "checkout", "-b", name, ref)
	return err
}

func (c *Client) BranchList(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}

func (c *Client) CreateBranch(ctx context.Context, name string) error {
	_, err := c.run(ctx, "switch", "-c", name)
	return err
}

func (c *Client) SwitchBranch(ctx context.Context, name string) error {
	_, err := c.run(ctx, "switch", name)
	return err
}

func (c *Client) DeleteBranch(ctx context.Context, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.run(ctx, "branch", flag, name)
	return err
}

func (c *Client) RenameBranch(ctx context.Context, oldName string, newName string) error {
	_, err := c.run(ctx, "branch", "-m", oldName, newName)
	return err
}

func (c *Client) CleanupMergedBranches(ctx context.Context, protected ...string) ([]string, error) {
	out, err := c.run(ctx, "branch", "--merged", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	current, _ := c.CurrentBranch(ctx)
	blocked := map[string]bool{current: true}
	for _, name := range protected {
		if name != "" {
			blocked[name] = true
		}
	}
	var deletable []string
	for _, branch := range splitNonEmptyLines(out) {
		if !blocked[branch] {
			deletable = append(deletable, branch)
		}
	}
	return deletable, nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = c.stdin
	cmd.Stderr = c.stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *Client) runQuiet(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = c.stdin
	cmd.Stderr = io.Discard
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *Client) runInteractive(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	return cmd.Run()
}

func splitNonEmptyLines(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitCommitMessage(message string) (string, string) {
	parts := strings.SplitN(strings.ReplaceAll(message, "\r\n", "\n"), "\n\n", 2)
	subject := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) == 2 {
		body = strings.TrimSpace(parts[1])
	}
	return subject, body
}

func RepoName(repoRoot string) string {
	return filepath.Base(repoRoot)
}

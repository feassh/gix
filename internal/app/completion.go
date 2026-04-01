package app

import (
	"context"
	"fmt"
	"strings"

	"gix/internal/config"
	"gix/internal/shellcompletion"
)

type completionContext struct {
	args       []string
	toComplete string
}

func (a *App) runCompletion(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.installCompletion("")
	}
	switch args[0] {
	case "install":
		shell := ""
		if len(args) > 1 {
			shell = args[1]
		}
		return a.installCompletion(shell)
	case "uninstall":
		shell := ""
		if len(args) > 1 {
			shell = args[1]
		}
		return a.uninstallCompletion(shell)
	case "print":
		shell := ""
		if len(args) > 1 {
			shell = args[1]
		}
		resolvedShell, err := resolvedShell(shell)
		if err != nil {
			return err
		}
		a.console.Println(shellcompletion.Script(resolvedShell))
		return nil
	case "bash", "zsh", "fish", "powershell", "pwsh":
		shell, err := parseShell(args[0])
		if err != nil {
			return err
		}
		a.console.Println(shellcompletion.Script(shell))
		return nil
	default:
		return fmt.Errorf("unknown completion subcommand %q", args[0])
	}
}

func (a *App) runComplete(ctx context.Context, args []string) error {
	ctxInfo := parseCompletionArgs(args)
	for _, candidate := range a.complete(ctx, ctxInfo) {
		a.console.Println(candidate)
	}
	return nil
}

func parseCompletionArgs(args []string) completionContext {
	if len(args) == 0 {
		return completionContext{args: nil, toComplete: ""}
	}
	return completionContext{
		args:       args[:len(args)-1],
		toComplete: args[len(args)-1],
	}
}

func (a *App) complete(ctx context.Context, c completionContext) []string {
	if len(c.args) == 0 {
		return filterPrefix(rootCompletions(), c.toComplete)
	}
	if strings.HasPrefix(c.toComplete, "-") {
		return filterPrefix(a.flagCompletions(ctx, c.args), c.toComplete)
	}

	switch c.args[0] {
	case "tag":
		return a.completeTag(ctx, c)
	case "branch":
		return a.completeBranch(ctx, c)
	case "repo":
		return a.completeRepo(ctx, c)
	case "config":
		return a.completeConfig(c)
	case "completion":
		if len(c.args) == 1 {
			return filterPrefix([]string{"install", "uninstall", "print", "bash", "zsh", "fish", "powershell"}, c.toComplete)
		}
		if len(c.args) == 2 && (c.args[1] == "install" || c.args[1] == "uninstall" || c.args[1] == "print") {
			return filterPrefix([]string{"bash", "zsh", "fish", "powershell"}, c.toComplete)
		}
		return nil
	case "push":
		return a.completePush(ctx, c)
	case "sync":
		return a.completeSync(ctx, c)
	case "commit", "ac", "acp":
		return a.completeCommit(c)
	default:
		if len(c.args) == 1 {
			return filterPrefix(rootCompletions(), c.toComplete)
		}
		return nil
	}
}

func rootCompletions() []string {
	return []string{
		"help",
		"version",
		"init",
		"install",
		"self-update",
		"uninstall",
		"status",
		"st",
		"add",
		"commit",
		"push",
		"ac",
		"acp",
		"sync",
		"tag",
		"branch",
		"repo",
		"config",
		"completion",
	}
}

func (a *App) flagCompletions(ctx context.Context, args []string) []string {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "add":
		return []string{"-A", "--all", "-u", "--update", "-p", "--patch", "--intent"}
	case "commit", "ac", "acp":
		return []string{"-y", "--yes", "-e", "--edit", "-r", "--regen", "-t", "--type", "-s", "--scope", "-l", "--lang", "--style", "--body", "--amend", "--no-ai", "--dry-run"}
	case "push":
		return []string{"-r", "--remote", "-b", "--branch", "-f", "--force", "-u", "--set-upstream", "--tags"}
	case "self-update":
		return []string{"--check", "--version"}
	case "sync":
		return []string{"--rebase", "--merge", "-r", "--remote", "-b", "--branch", "--push", "--only-fetch", "--from-upstream", "--from-origin"}
	case "completion":
		return nil
	case "tag":
		if len(args) < 2 {
			return nil
		}
		switch args[1] {
		case "create":
			return []string{"-n", "--name", "-m", "--message", "--auto", "--push", "--from"}
		case "list":
			return []string{"--latest", "--sort"}
		case "push":
			return []string{"-n", "--name", "--all"}
		case "delete":
			return []string{"-n", "--name", "--remote", "-y", "--yes"}
		}
	case "branch":
		if len(args) >= 2 && args[1] == "delete" {
			return []string{"--force", "--yes"}
		}
	case "config":
		if len(args) < 2 {
			return nil
		}
		switch args[1] {
		case "get", "set":
			return []string{"--global"}
		case "list":
			return []string{"--global", "--project"}
		}
	}
	return nil
}

func (a *App) completeCommit(c completionContext) []string {
	if expectsValue(c.args, "-l", "--lang") {
		return filterPrefix([]string{"en", "zh"}, c.toComplete)
	}
	if expectsValue(c.args, "--style") {
		return filterPrefix([]string{"conventional", "simple", "team"}, c.toComplete)
	}
	if expectsValue(c.args, "-t", "--type") {
		return filterPrefix([]string{"feat", "fix", "docs", "refactor", "test", "chore", "perf", "build", "ci"}, c.toComplete)
	}
	return nil
}

func (a *App) completePush(ctx context.Context, c completionContext) []string {
	if expectsValue(c.args, "-r", "--remote") {
		return filterPrefix(a.remoteNames(ctx), c.toComplete)
	}
	if expectsValue(c.args, "-b", "--branch") {
		return filterPrefix(a.branchNames(ctx), c.toComplete)
	}
	return nil
}

func (a *App) completeSync(ctx context.Context, c completionContext) []string {
	if expectsValue(c.args, "-r", "--remote") {
		return filterPrefix(a.remoteNames(ctx), c.toComplete)
	}
	if expectsValue(c.args, "-b", "--branch") {
		return filterPrefix(a.branchNames(ctx), c.toComplete)
	}
	return nil
}

func (a *App) completeTag(ctx context.Context, c completionContext) []string {
	if len(c.args) == 1 {
		return filterPrefix([]string{"create", "list", "push", "delete", "checkout", "branch"}, c.toComplete)
	}
	subcommand := c.args[1]
	switch subcommand {
	case "create":
		if expectsValue(c.args, "--from") {
			return filterPrefix(a.refNames(ctx), c.toComplete)
		}
		return nil
	case "list":
		if expectsValue(c.args, "--sort") {
			return filterPrefix([]string{"version", "date"}, c.toComplete)
		}
		return nil
	case "push", "delete", "checkout":
		if subcommand == "push" && expectsValue(c.args, "-n", "--name") {
			return filterPrefix(a.tagNames(ctx), c.toComplete)
		}
		if subcommand == "delete" && expectsValue(c.args, "-n", "--name") {
			return filterPrefix(a.tagNames(ctx), c.toComplete)
		}
		if subcommand == "checkout" && len(c.args) == 2 {
			return filterPrefix(a.tagNames(ctx), c.toComplete)
		}
		return nil
	case "branch":
		if len(c.args) == 2 {
			return filterPrefix(a.tagNames(ctx), c.toComplete)
		}
		return nil
	default:
		return nil
	}
}

func (a *App) completeBranch(ctx context.Context, c completionContext) []string {
	if len(c.args) == 1 {
		return filterPrefix([]string{"list", "new", "switch", "delete", "rename", "cleanup"}, c.toComplete)
	}
	subcommand := c.args[1]
	switch subcommand {
	case "switch", "delete":
		if len(c.args) == 2 {
			return filterPrefix(a.branchNames(ctx), c.toComplete)
		}
	case "rename":
		if len(c.args) == 2 {
			return filterPrefix(a.branchNames(ctx), c.toComplete)
		}
	}
	return nil
}

func (a *App) completeRepo(ctx context.Context, c completionContext) []string {
	if len(c.args) == 1 {
		return filterPrefix([]string{"info", "remote", "set-default-remote", "set-main-branch"}, c.toComplete)
	}
	switch c.args[1] {
	case "set-default-remote":
		if len(c.args) == 2 {
			return filterPrefix(a.remoteNames(ctx), c.toComplete)
		}
	case "set-main-branch":
		if len(c.args) == 2 {
			return filterPrefix(a.branchNames(ctx), c.toComplete)
		}
	}
	return nil
}

func (a *App) completeConfig(c completionContext) []string {
	if len(c.args) == 1 {
		return filterPrefix([]string{"init", "get", "set", "list"}, c.toComplete)
	}
	subcommand := c.args[1]
	switch subcommand {
	case "get", "set":
		if expectsValue(c.args, "--global") {
			return nil
		}
		if shouldCompleteConfigKey(c.args) {
			return filterPrefix(config.DefaultValues().SortedKeys(), c.toComplete)
		}
		if subcommand == "set" && len(c.args) >= 3 {
			return filterPrefix(configValueSuggestions(c.args[len(c.args)-1]), c.toComplete)
		}
	case "list":
		return nil
	}
	return nil
}

func shouldCompleteConfigKey(args []string) bool {
	filtered := removeFlags(args)
	return len(filtered) <= 2
}

func configValueSuggestions(key string) []string {
	switch key {
	case "commit.language", "ai.language":
		return []string{"en", "zh"}
	case "commit.style":
		return []string{"conventional", "simple", "team"}
	case "tag.auto_increment":
		return []string{"patch", "minor", "major"}
	case "ai.provider":
		return []string{"openai"}
	case "ai.thinking", "tag.enabled", "tag.annotated", "tag.push_after_create", "commit.with_body", "commit.confirm", "ui.color", "ui.interactive":
		return []string{"true", "false"}
	default:
		return nil
	}
}

func (a *App) remoteNames(ctx context.Context) []string {
	remotes, err := a.git.ListRemotes(ctx)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(remotes))
	for _, remote := range remotes {
		names = append(names, remote.Name)
	}
	return names
}

func (a *App) branchNames(ctx context.Context) []string {
	branches, err := a.git.BranchList(ctx)
	if err != nil {
		return nil
	}
	return branches
}

func (a *App) tagNames(ctx context.Context) []string {
	tags, err := a.git.ListTags(ctx, "version")
	if err != nil {
		return nil
	}
	return tags
}

func (a *App) refNames(ctx context.Context) []string {
	refs := append([]string{}, a.branchNames(ctx)...)
	refs = append(refs, a.tagNames(ctx)...)
	return refs
}

func filterPrefix(values []string, prefix string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		if strings.HasPrefix(value, prefix) {
			out = append(out, value)
			seen[value] = true
		}
	}
	return out
}

func expectsValue(args []string, names ...string) bool {
	if len(args) == 0 {
		return false
	}
	last := args[len(args)-1]
	for _, name := range names {
		if last == name {
			return true
		}
	}
	return false
}

func removeFlags(args []string) []string {
	var filtered []string
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		switch arg {
		case "--global", "--project", "--yes", "-y", "--force", "--remote":
			continue
		case "-r", "-b", "--branch", "--lang", "-l", "--type", "-t", "--scope", "-s", "--style", "--name", "-n", "--message", "-m", "--from":
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func (a *App) installCompletion(shell string) error {
	result, err := a.setupCompletion(shell)
	if err != nil {
		return err
	}
	a.printCompletionResult(true, result)
	return nil
}

func (a *App) uninstallCompletion(shell string) error {
	result, err := a.removeCompletion(shell)
	if err != nil {
		return err
	}
	a.printCompletionResult(false, result)
	return nil
}

func parseShell(value string) (shellcompletion.Shell, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bash":
		return shellcompletion.ShellBash, nil
	case "zsh":
		return shellcompletion.ShellZsh, nil
	case "fish":
		return shellcompletion.ShellFish, nil
	case "powershell", "pwsh":
		return shellcompletion.ShellPowerShell, nil
	default:
		return "", fmt.Errorf("unsupported shell %q", value)
	}
}

func resolvedShell(value string) (shellcompletion.Shell, error) {
	if strings.TrimSpace(value) != "" {
		return parseShell(value)
	}
	return shellcompletion.DetectShell()
}

func (a *App) setupCompletion(shell string) (shellcompletion.Result, error) {
	manager := shellcompletion.NewManager()
	return manager.Install(shell)
}

func (a *App) removeCompletion(shell string) (shellcompletion.Result, error) {
	manager := shellcompletion.NewManager()
	return manager.Uninstall(shell)
}

func (a *App) printCompletionResult(installed bool, result shellcompletion.Result) {
	if installed {
		a.console.Success(a.tr("message.completion_installed", result.Shell))
	} else {
		a.console.Success(a.tr("message.completion_removed", result.Shell))
	}
	a.console.Detail(a.tr("label.shell"), string(result.Shell))
	if len(result.UpdatedFiles) > 0 {
		a.console.Section(a.tr("common.files"))
		for _, file := range result.UpdatedFiles {
			a.console.Bullet(file)
		}
	}
	for _, note := range result.Notes {
		expected := a.tr("message.completion_removed", result.Shell)
		if installed {
			expected = a.tr("message.completion_installed", result.Shell)
		}
		localized := a.localizeNote(note)
		if localized != expected {
			a.console.Note(localized)
		}
	}
}

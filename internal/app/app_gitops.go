package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gix/internal/ui"
	"gix/internal/version"
)

func (a *App) runTag(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gix tag <create|list|push|delete|checkout|branch>")
	}
	switch args[0] {
	case "create":
		return a.runTagCreate(ctx, args[1:])
	case "list":
		return a.runTagList(ctx, args[1:])
	case "push":
		return a.runTagPush(ctx, args[1:])
	case "delete":
		return a.runTagDelete(ctx, args[1:])
	case "checkout":
		if len(args) < 2 {
			return fmt.Errorf("usage: gix tag checkout <tag>")
		}
		return a.git.Checkout(ctx, args[1])
	case "branch":
		if len(args) < 3 {
			return fmt.Errorf("usage: gix tag branch <tag> <new-branch>")
		}
		return a.git.BranchFromRef(ctx, args[1], args[2])
	default:
		return fmt.Errorf("unknown tag subcommand %q", args[0])
	}
}

func (a *App) runTagCreate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tag create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("n", "", "")
	fs.StringVar(name, "name", "", "")
	message := fs.String("m", "", "")
	fs.StringVar(message, "message", "", "")
	auto := fs.Bool("auto", false, "")
	pushAfter := fs.Bool("push", false, "")
	fromRef := fs.String("from", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	tagName := *name
	if tagName == "" || *auto {
		current := cfg.Tag.Current
		if current == "" {
			current, _ = a.git.LatestTag(ctx)
		}
		tagName, err = version.NextTag(current, cfg.Tag.Prefix, cfg.Tag.AutoIncrement)
		if err != nil {
			return err
		}
	}
	if *fromRef == "" && !a.git.HeadExists(ctx) {
		return fmt.Errorf("cannot create a tag before the first commit")
	}
	ok, err := a.console.Confirm(fmt.Sprintf("Create tag %s?", tagName), true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := a.git.CreateTag(ctx, tagName, *message, *fromRef, cfg.Tag.Annotated); err != nil {
		return err
	}
	if err := a.updateProjectValue(repoRoot, "tag.current", tagName); err != nil {
		return err
	}
	if *pushAfter || cfg.Tag.PushAfterCreate {
		if err := a.git.PushTag(ctx, defaultRemote(cfg), tagName, false); err != nil {
			return err
		}
	}
	a.console.Success(a.tr("message.tag_created", tagName))
	return nil
}

func (a *App) runTagList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tag list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	latest := fs.Bool("latest", false, "")
	sortBy := fs.String("sort", "version", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := a.git.EnsureRepo(ctx); err != nil {
		return err
	}
	tags, err := a.git.ListTags(ctx, *sortBy)
	if err != nil {
		return err
	}
	if *latest && len(tags) > 1 {
		tags = tags[:1]
	}
	for _, tag := range tags {
		a.console.Println(tag)
	}
	return nil
}

func (a *App) runTagPush(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tag push", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("n", "", "")
	fs.StringVar(name, "name", "", "")
	all := fs.Bool("all", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	targetName := *name
	if targetName == "" && !*all {
		targetName, _ = a.git.LatestTag(ctx)
	}
	return a.git.PushTag(ctx, defaultRemote(cfg), targetName, *all)
}

func (a *App) runTagDelete(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tag delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("n", "", "")
	fs.StringVar(name, "name", "", "")
	remote := fs.Bool("remote", false, "")
	yes := fs.Bool("y", false, "")
	fs.BoolVar(yes, "yes", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		return fmt.Errorf("usage: gix tag delete -n <tag>")
	}
	if !*yes {
		ok, err := a.console.Confirm(fmt.Sprintf("Delete tag %s?", *name), false)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	return a.git.DeleteTag(ctx, *name, *remote, "")
}

func (a *App) runBranch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gix branch <list|new|switch|delete|rename|cleanup>")
	}
	switch args[0] {
	case "list":
		branches, err := a.git.BranchList(ctx)
		if err != nil {
			return err
		}
		for _, branch := range branches {
			a.console.Println(branch)
		}
		return nil
	case "new":
		if len(args) < 2 {
			return fmt.Errorf("usage: gix branch new <name>")
		}
		return a.git.CreateBranch(ctx, args[1])
	case "switch":
		if len(args) < 2 {
			return fmt.Errorf("usage: gix branch switch <name>")
		}
		return a.git.SwitchBranch(ctx, args[1])
	case "delete":
		fs := flag.NewFlagSet("branch delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		force := fs.Bool("force", false, "")
		yes := fs.Bool("yes", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(fs.Args()) == 0 {
			return fmt.Errorf("usage: gix branch delete <name>")
		}
		if !*yes {
			ok, err := a.console.Confirm(fmt.Sprintf("Delete branch %s?", fs.Args()[0]), false)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
		return a.git.DeleteBranch(ctx, fs.Args()[0], *force)
	case "rename":
		if len(args) < 3 {
			return fmt.Errorf("usage: gix branch rename <old> <new>")
		}
		return a.git.RenameBranch(ctx, args[1], args[2])
	case "cleanup":
		_, cfg, err := a.mustRepoConfig(ctx)
		if err != nil {
			return err
		}
		branches, err := a.git.CleanupMergedBranches(ctx, cfg.Project.MainBranch, "master")
		if err != nil {
			return err
		}
		if len(branches) == 0 {
			a.console.Note(a.tr("message.no_merged_branches"))
			return nil
		}
		a.console.Section(a.tr("message.merged_branches"))
		for _, branch := range branches {
			a.console.Bullet(branch)
		}
		ok, err := a.console.Confirm("Delete these branches?", false)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		for _, branch := range branches {
			if err := a.git.DeleteBranch(ctx, branch, false); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown branch subcommand %q", args[0])
	}
}

func (a *App) runRepo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gix repo <info|remote|set-default-remote|set-main-branch>")
	}
	switch args[0] {
	case "info":
		repoRoot, cfg, err := a.mustRepoConfig(ctx)
		if err != nil {
			return err
		}
		branch, _ := a.git.CurrentBranch(ctx)
		latestTag, _ := a.git.LatestTag(ctx)
		a.console.Section(a.tr("section.repo_info"))
		a.console.Details([]ui.Detail{
			detail(a.tr("label.repository"), filepath.Base(repoRoot)),
			detail(a.tr("label.current_branch"), a.localeValue(branch)),
			detail(a.tr("label.default_remote"), a.localeValue(defaultRemote(cfg))),
			detail(a.tr("label.upstream_remote"), a.localeValue(cfg.Project.UpstreamRemote)),
			detail(a.tr("label.main_branch"), a.localeValue(cfg.Project.MainBranch)),
			detail(a.tr("label.commit_style"), a.localeValue(cfg.Commit.Style)),
			detail(a.tr("label.latest_tag"), firstNonEmpty(latestTag, a.tr("common.none"))),
		})
		return nil
	case "remote":
		remotes, err := a.git.ListRemotes(ctx)
		if err != nil {
			return err
		}
		for _, remote := range remotes {
			a.console.Printf("%s\t%s\n", remote.Name, remote.URL)
		}
		return nil
	case "set-default-remote":
		if len(args) < 2 {
			return fmt.Errorf("usage: gix repo set-default-remote <name>")
		}
		repoRoot, _, err := a.mustRepoConfig(ctx)
		if err != nil {
			return err
		}
		return a.updateProjectValue(repoRoot, "project.default_remote", args[1])
	case "set-main-branch":
		if len(args) < 2 {
			return fmt.Errorf("usage: gix repo set-main-branch <name>")
		}
		repoRoot, _, err := a.mustRepoConfig(ctx)
		if err != nil {
			return err
		}
		return a.updateProjectValue(repoRoot, "project.main_branch", args[1])
	default:
		return fmt.Errorf("unknown repo subcommand %q", args[0])
	}
}

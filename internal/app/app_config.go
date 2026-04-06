package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"gix/internal/config"
	"gix/internal/ui"
)

func (a *App) runConfig(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gix config <init|get|set|list>")
	}
	switch args[0] {
	case "init":
		return a.runConfigInit(ctx, args[1:])
	case "get":
		return a.runConfigGet(ctx, args[1:])
	case "set":
		return a.runConfigSet(ctx, args[1:])
	case "list":
		return a.runConfigList(ctx, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func (a *App) runConfigInit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		return err
	}
	remotes, err := a.git.ListRemotes(ctx)
	if err != nil {
		return err
	}
	mainBranch, _ := a.git.DetectMainBranch(ctx)
	values, _, err := a.store.LoadProject(repoRoot)
	if err != nil {
		return err
	}
	if values == nil {
		values = make(config.Values)
	}
	_ = values.Set("project.name", filepath.Base(repoRoot))
	_ = values.Set("project.main_branch", mainBranch)
	_ = values.Set("commit.style", "conventional")
	_ = values.Set("commit.language", "en")
	_ = values.Set("tag.enabled", "true")
	_ = values.Set("tag.prefix", "v")
	_ = values.Set("tag.pattern", "semver")
	_ = values.Set("tag.auto_increment", "patch")
	_ = values.Set("tag.annotated", "true")

	for _, remote := range remotes {
		switch remote.Name {
		case "origin":
			_ = values.Set("project.default_remote", "origin")
		case "upstream":
			_ = values.Set("project.upstream_remote", "upstream")
		}
	}
	if _, err := a.store.SaveProject(repoRoot, values); err != nil {
		return err
	}
	a.console.Section(a.tr("section.config_init"))
	a.console.Details([]ui.Detail{
		detail(a.tr("label.detected_repository"), filepath.Base(repoRoot)),
		detail(a.tr("label.default_remote"), firstNonEmpty(values["project.default_remote"], a.tr("common.none"))),
		detail(a.tr("label.upstream_remote"), firstNonEmpty(values["project.upstream_remote"], a.tr("common.none"))),
		detail(a.tr("label.main_branch"), firstNonEmpty(mainBranch, a.tr("common.none"))),
	})
	a.console.Success(a.tr("message.project_config_saved"))
	return nil
}

func (a *App) runConfigGet(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) == 0 {
		return fmt.Errorf("usage: gix config get [--global] <key>")
	}
	values, err := a.loadConfigValues(ctx, *global)
	if err != nil {
		return err
	}
	value, ok := values.Get(fs.Args()[0])
	if !ok {
		return fmt.Errorf("config key %q not found", fs.Args()[0])
	}
	a.console.Println(value)
	return nil
}

func (a *App) runConfigSet(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: gix config set [--global] <key> <value>")
	}
	values, repoRoot, err := a.loadWritableConfigValues(ctx, *global)
	if err != nil {
		return err
	}
	if err := values.Set(fs.Args()[0], fs.Args()[1]); err != nil {
		return err
	}
	if *global {
		_, err = a.store.SaveGlobal(values)
	} else {
		_, err = a.store.SaveProject(repoRoot, values)
	}
	return err
}

func (a *App) runConfigList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "")
	projectOnly := fs.Bool("project", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var values config.Values
	switch {
	case *global:
		var err error
		values, _, err = a.store.LoadGlobal()
		if err != nil {
			return err
		}
	case *projectOnly:
		repoRoot, err := a.git.RepoRoot(ctx)
		if err != nil {
			return err
		}
		var errLoad error
		values, _, errLoad = a.store.LoadProject(repoRoot)
		if errLoad != nil {
			return errLoad
		}
	default:
		repoRoot, _ := a.git.RepoRoot(ctx)
		resolved, _, err := a.store.LoadResolved(repoRoot)
		if err != nil {
			return err
		}
		values = config.DefaultValues()
		cfgValues := values.Merge(config.Values{
			"ai.provider":               resolved.AI.Provider,
			"ai.model":                  resolved.AI.Model,
			"ai.base_url":               resolved.AI.BaseURL,
			"ai.api_key":                maskSecret(resolved.AI.APIKey),
			"ai.timeout":                fmt.Sprintf("%d", resolved.AI.Timeout),
			"ai.language":               resolved.AI.Language,
			"ai.thinking":               fmt.Sprintf("%t", resolved.AI.Thinking),
			"commit.style":              resolved.Commit.Style,
			"commit.language":           resolved.Commit.Language,
			"commit.default_type":       resolved.Commit.DefaultType,
			"commit.default_scope":      resolved.Commit.DefaultScope,
			"commit.with_body":          fmt.Sprintf("%t", resolved.Commit.WithBody),
			"commit.confirm":            fmt.Sprintf("%t", resolved.Commit.Confirm),
			"commit.max_subject_length": fmt.Sprintf("%d", resolved.Commit.MaxSubjectLength),
			"push.default_remote":       resolved.Push.DefaultRemote,
			"ui.color":                  fmt.Sprintf("%t", resolved.UI.Color),
			"ui.interactive":            fmt.Sprintf("%t", resolved.UI.Interactive),
			"project.name":              resolved.Project.Name,
			"project.main_branch":       resolved.Project.MainBranch,
			"project.default_remote":    resolved.Project.DefaultRemote,
			"project.upstream_remote":   resolved.Project.UpstreamRemote,
			"tag.enabled":               fmt.Sprintf("%t", resolved.Tag.Enabled),
			"tag.prefix":                resolved.Tag.Prefix,
			"tag.pattern":               resolved.Tag.Pattern,
			"tag.current":               resolved.Tag.Current,
			"tag.auto_increment":        resolved.Tag.AutoIncrement,
			"tag.annotated":             fmt.Sprintf("%t", resolved.Tag.Annotated),
			"tag.push_after_create":     fmt.Sprintf("%t", resolved.Tag.PushAfterCreate),
			"self_update.repo":          resolved.SelfUpdate.Repo,
			"self_update.base_url":      resolved.SelfUpdate.BaseURL,
		})
		values = cfgValues
	}
	for _, key := range values.SortedKeys() {
		a.console.Printf("%s=%s\n", key, values[key])
	}
	return nil
}

func (a *App) mustRepoConfig(ctx context.Context) (string, config.Config, error) {
	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, _, err := a.store.LoadResolved(repoRoot)
	if err != nil {
		return "", config.Config{}, err
	}
	a.applyUIConfig(cfg)
	if cfg.Project.Name == "" {
		cfg.Project.Name = filepath.Base(repoRoot)
	}
	if cfg.Project.MainBranch == "" {
		cfg.Project.MainBranch, _ = a.git.DetectMainBranch(ctx)
	}
	return repoRoot, cfg, nil
}

func (a *App) loadResolvedConfig(ctx context.Context) (config.Config, error) {
	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		repoRoot = ""
	}
	cfg, _, err := a.store.LoadResolved(repoRoot)
	if err != nil {
		return config.Config{}, err
	}
	a.applyUIConfig(cfg)
	return cfg, nil
}

func (a *App) loadConfigValues(ctx context.Context, global bool) (config.Values, error) {
	if global {
		values, _, err := a.store.LoadGlobal()
		return values, err
	}
	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		return nil, err
	}
	values, _, err := a.store.LoadProject(repoRoot)
	return values, err
}

func (a *App) loadWritableConfigValues(ctx context.Context, global bool) (config.Values, string, error) {
	if global {
		values, _, err := a.store.LoadGlobal()
		if err != nil {
			return nil, "", err
		}
		return values, "", nil
	}
	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		return nil, "", err
	}
	values, _, err := a.store.LoadProject(repoRoot)
	if err != nil {
		return nil, "", err
	}
	return values, repoRoot, nil
}

func (a *App) updateProjectValue(repoRoot string, key string, value string) error {
	values, _, err := a.store.LoadProject(repoRoot)
	if err != nil {
		return err
	}
	if values == nil {
		values = make(config.Values)
	}
	if err := values.Set(key, value); err != nil {
		return err
	}
	_, err = a.store.SaveProject(repoRoot, values)
	return err
}

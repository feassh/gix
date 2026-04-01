package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/ui"
)

func (a *App) runAdd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	all := fs.Bool("A", false, "")
	allLong := fs.Bool("all", false, "")
	update := fs.Bool("u", false, "")
	updateLong := fs.Bool("update", false, "")
	patch := fs.Bool("p", false, "")
	patchLong := fs.Bool("patch", false, "")
	intent := fs.Bool("intent", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := a.git.EnsureRepo(ctx); err != nil {
		return err
	}
	if *intent {
		status, err := a.git.Status(ctx, "origin")
		if err != nil {
			return err
		}
		a.console.Section(a.tr("section.workspace_status"))
		a.console.Details([]ui.Detail{
			detail(a.tr("label.branch"), status.CurrentBranch),
			detail(a.tr("label.staged"), fmt.Sprintf("%d", status.StagedCount)),
			detail(a.tr("label.unstaged"), fmt.Sprintf("%d", status.UnstagedCount)),
			detail(a.tr("label.untracked"), fmt.Sprintf("%d", status.UntrackedCount)),
		})
		return nil
	}
	return a.git.Add(ctx, *all || *allLong, *update || *updateLong, *patch || *patchLong, fs.Args())
}

func (a *App) runCommit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	yes := fs.Bool("y", false, "")
	fs.BoolVar(yes, "yes", false, "")
	edit := fs.Bool("e", false, "")
	fs.BoolVar(edit, "edit", false, "")
	regen := fs.Bool("r", false, "")
	fs.BoolVar(regen, "regen", false, "")
	commitType := fs.String("t", "", "")
	fs.StringVar(commitType, "type", "", "")
	scope := fs.String("s", "", "")
	fs.StringVar(scope, "scope", "", "")
	lang := fs.String("l", "", "")
	fs.StringVar(lang, "lang", "", "")
	style := fs.String("style", "", "")
	withBody := fs.Bool("body", false, "")
	amend := fs.Bool("amend", false, "")
	noAI := fs.Bool("no-ai", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	files, err := a.git.ListChangedFiles(ctx, true)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New(a.tr("message.no_staged_files"))
	}
	diff, err := a.git.StagedDiff(ctx)
	if err != nil {
		return err
	}
	req := ai.CommitRequest{
		Diff:             diff,
		Files:            files,
		Language:         firstNonEmpty(*lang, cfg.Commit.Language, cfg.AI.Language),
		Style:            firstNonEmpty(*style, cfg.Commit.Style),
		Type:             firstNonEmpty(*commitType, cfg.Commit.DefaultType),
		Scope:            firstNonEmpty(*scope, cfg.Commit.DefaultScope),
		WithBody:         *withBody || cfg.Commit.WithBody,
		MaxSubjectLength: cfg.Commit.MaxSubjectLength,
	}
	generator := a.commitGenerator(cfg)
	if *noAI {
		generator = &ai.FallbackGenerator{}
	}

	message, err := a.generateCommitMessage(ctx, generator, req, !*noAI && cfg.UI.Interactive)
	if err != nil {
		return err
	}
	if *regen {
		message, err = a.generateCommitMessage(ctx, generator, req, !*noAI && cfg.UI.Interactive)
		if err != nil {
			return err
		}
	}
	if *edit {
		message, err = a.editCommitMessage(req.WithBody, message)
		if err != nil {
			return err
		}
	}
	if *dryRun {
		a.console.Println(message.Raw)
		return nil
	}
	if *yes || !cfg.Commit.Confirm {
		if err := a.git.Commit(ctx, message.Raw, *amend); err != nil {
			return err
		}
		a.console.Success(a.tr("message.committed_with_source", a.localeForCommitSource(message.Source)))
		return nil
	}

	for {
		a.console.Section(a.tr("section.commit_preview"))
		a.console.Println(message.Raw)
		a.console.BlankLine()
		choice, err := a.console.PromptChoice(a.tr("message.commit_choose"), "Y")
		if err != nil {
			return err
		}
		switch choice {
		case "Y", "YES":
			if err := a.git.Commit(ctx, message.Raw, *amend); err != nil {
				return err
			}
			a.console.Success(a.tr("message.committed_with_source", a.localeForCommitSource(message.Source)))
			return nil
		case "E", "EDIT":
			message, err = a.editCommitMessage(req.WithBody, message)
			if err != nil {
				return err
			}
		case "R", "REGENERATE":
			message, err = a.generateCommitMessage(ctx, generator, req, !*noAI && cfg.UI.Interactive)
			if err != nil {
				return err
			}
		case "C", "CANCEL":
			return nil
		default:
			a.console.Warning(a.tr("message.commit_choose_invalid"))
		}
	}
}

func (a *App) generateCommitMessage(ctx context.Context, generator ai.Generator, req ai.CommitRequest, stream bool) (ai.CommitMessage, error) {
	if !stream {
		return generator.Generate(ctx, req)
	}
	panel := a.console.NewStreamPanel()
	req.Observer = panel
	return generator.Generate(ctx, req)
}

func (a *App) runPush(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	remote := fs.String("r", "", "")
	fs.StringVar(remote, "remote", "", "")
	branch := fs.String("b", "", "")
	fs.StringVar(branch, "branch", "", "")
	force := fs.Bool("f", false, "")
	fs.BoolVar(force, "force", false, "")
	setUpstream := fs.Bool("u", false, "")
	fs.BoolVar(setUpstream, "set-upstream", false, "")
	tags := fs.Bool("tags", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	targetBranch := *branch
	if targetBranch == "" {
		targetBranch, err = a.git.CurrentBranch(ctx)
		if err != nil {
			return err
		}
	}
	targetRemote := firstNonEmpty(*remote, defaultRemote(cfg))
	if *force {
		ok, err := a.console.Confirm("Force push with lease?", false)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	return a.git.Push(ctx, targetRemote, targetBranch, *force, *setUpstream, *tags)
}

func (a *App) runAC(ctx context.Context, args []string) error {
	if err := a.git.EnsureRepo(ctx); err != nil {
		return err
	}
	if err := a.git.Add(ctx, false, false, false, nil); err != nil {
		return err
	}
	return a.runCommit(ctx, args)
}

func (a *App) runACP(ctx context.Context, args []string) error {
	commitArgs, pushArgs := splitArgs(args, map[string]bool{
		"-r": true, "--remote": true, "-b": true, "--branch": true, "--force": false, "-f": false, "--set-upstream": false, "-u": false,
	})
	if err := a.runAC(ctx, commitArgs); err != nil {
		return err
	}
	return a.runPush(ctx, pushArgs)
}

func (a *App) runSync(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rebase := fs.Bool("rebase", false, "")
	merge := fs.Bool("merge", false, "")
	remote := fs.String("r", "", "")
	fs.StringVar(remote, "remote", "", "")
	branch := fs.String("b", "", "")
	fs.StringVar(branch, "branch", "", "")
	pushAfter := fs.Bool("push", false, "")
	onlyFetch := fs.Bool("only-fetch", false, "")
	fromUpstream := fs.Bool("from-upstream", false, "")
	fromOrigin := fs.Bool("from-origin", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	targetRemote := *remote
	if targetRemote == "" {
		switch {
		case *fromOrigin:
			targetRemote = defaultRemote(cfg)
		case *fromUpstream && cfg.Project.UpstreamRemote != "":
			targetRemote = cfg.Project.UpstreamRemote
		case cfg.Project.UpstreamRemote != "" && a.git.HasRemote(ctx, cfg.Project.UpstreamRemote):
			targetRemote = cfg.Project.UpstreamRemote
		default:
			targetRemote = defaultRemote(cfg)
		}
	}
	mainBranch := firstNonEmpty(*branch, cfg.Project.MainBranch)
	if mainBranch == "" {
		mainBranch, err = a.git.DetectMainBranch(ctx)
		if err != nil {
			return err
		}
	}
	if err := a.git.Fetch(ctx, defaultRemote(cfg)); err != nil && defaultRemote(cfg) != targetRemote {
		return err
	}
	if cfg.Project.UpstreamRemote != "" && cfg.Project.UpstreamRemote != defaultRemote(cfg) {
		if err := a.git.Fetch(ctx, cfg.Project.UpstreamRemote); err != nil && cfg.Project.UpstreamRemote == targetRemote {
			return err
		}
	}
	if err := a.git.Fetch(ctx, targetRemote); err != nil {
		return err
	}
	if *onlyFetch {
		a.console.Success(a.tr("message.fetch_completed"))
		return nil
	}
	ref := fmt.Sprintf("%s/%s", targetRemote, mainBranch)
	if *merge {
		err = a.git.Merge(ctx, ref)
	} else {
		if !*rebase {
			*rebase = true
		}
		err = a.git.Rebase(ctx, ref)
	}
	if err != nil {
		return err
	}
	if *pushAfter {
		return a.runPush(ctx, nil)
	}
	return nil
}

func (a *App) commitGenerator(cfg config.Config) ai.Generator {
	return ai.NewOpenAIGenerator(ai.OpenAIConfig{
		BaseURL:   cfg.AI.BaseURL,
		Model:     cfg.AI.Model,
		APIKeyEnv: cfg.AI.APIKeyEnv,
		Timeout:   time.Duration(cfg.AI.Timeout) * time.Second,
		Thinking:  cfg.AI.Thinking,
	}, &ai.FallbackGenerator{})
}

func (a *App) editCommitMessage(withBody bool, current ai.CommitMessage) (ai.CommitMessage, error) {
	subject, err := a.console.PromptLine(fmt.Sprintf("Commit subject [%s]: ", current.Subject))
	if err != nil {
		return ai.CommitMessage{}, err
	}
	if strings.TrimSpace(subject) == "" {
		subject = current.Subject
	}
	body := current.Body
	if withBody {
		newBody, err := a.console.PromptMultiline("Commit body")
		if err != nil {
			return ai.CommitMessage{}, err
		}
		if strings.TrimSpace(newBody) != "" {
			body = newBody
		}
	}
	return ai.CommitMessage{
		Subject: subject,
		Body:    body,
		Raw:     strings.TrimSpace(subject + "\n\n" + body),
		Source:  "manual",
	}, nil
}

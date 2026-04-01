package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gix/internal/install"
	"gix/internal/shellcompletion"
	"gix/internal/ui"
	"gix/internal/update"
)

func (a *App) runStatus(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, cfg, err := a.mustRepoConfig(ctx)
	if err != nil {
		return err
	}
	summary, err := a.git.Status(ctx, defaultRemote(cfg))
	if err != nil {
		return err
	}
	details := []ui.Detail{
		detail(a.tr("label.repository"), filepath.Base(repoRoot)),
		detail(a.tr("label.branch"), summary.CurrentBranch),
	}
	if summary.Upstream != "" {
		details = append(details,
			detail(a.tr("label.upstream"), summary.Upstream),
			detail(a.tr("label.ahead_behind"), fmt.Sprintf("%d/%d", summary.Ahead, summary.Behind)),
		)
	}
	details = append(details,
		detail(a.tr("label.staged"), fmt.Sprintf("%d", summary.StagedCount)),
		detail(a.tr("label.unstaged"), fmt.Sprintf("%d", summary.UnstagedCount)),
		detail(a.tr("label.untracked"), fmt.Sprintf("%d", summary.UntrackedCount)),
	)
	if summary.LatestTag != "" {
		details = append(details, detail(a.tr("label.latest_tag"), summary.LatestTag))
	}
	details = append(details, detail(a.tr("label.default_remote"), summary.DefaultRemote))
	a.console.Section(a.tr("section.workspace_status"))
	a.console.Details(details)
	return nil
}

func (a *App) runInit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoRoot, err := a.git.RepoRoot(ctx)
	if err != nil {
		if err := a.git.Init(ctx); err != nil {
			return err
		}
		a.console.Success(a.tr("message.git_initialized"))
		return a.runConfigInit(ctx, nil)
	}

	if a.projectConfigExists(repoRoot) {
		a.console.Note(a.tr("message.git_and_gix_ready"))
		return nil
	}

	a.console.Note(a.tr("message.git_exists_init_gix"))
	return a.runConfigInit(ctx, nil)
}

func (a *App) runInstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	manager := install.NewManager()
	result, err := manager.Install(ctx)
	if err != nil {
		return err
	}
	completionResult, completionErr := a.setupCompletion("")
	a.printInstallReport(result, completionResult, completionErr)
	return nil
}

func (a *App) runUninstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	manager := install.NewManager()
	result, err := manager.Uninstall(ctx)
	if err != nil {
		return err
	}
	completionResult, completionErr := a.removeCompletion("")
	a.printUninstallReport(result, completionResult, completionErr)
	return nil
}

func (a *App) runSelfUpdate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	checkOnly := fs.Bool("check", false, "")
	versionFlag := fs.String("version", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := a.loadResolvedConfig(ctx)
	if err != nil {
		return err
	}
	manager := update.NewManager(update.NewClient(update.ResolveBaseURL(cfg)), install.NewManager())
	if *checkOnly {
		result, _, err := manager.Check(ctx, cfg, *versionFlag)
		if err != nil {
			return err
		}
		a.console.Section(a.tr("section.update_status"))
		a.console.Details([]ui.Detail{
			detail(a.tr("label.current_version"), result.CurrentVersion),
			detail(a.tr("label.latest_version"), result.LatestVersion),
			detail(a.tr("label.release_repo"), result.Repo),
		})
		if result.UpdateAvailable {
			a.console.Warning(a.tr("message.update_available"))
		} else {
			a.console.Success(a.tr("message.already_up_to_date"))
		}
		return nil
	}

	result, err := manager.Update(ctx, cfg, *versionFlag)
	if err != nil {
		return err
	}
	if !result.UpdateAvailable {
		a.console.Success(a.tr("message.already_up_to_date_version", result.CurrentVersion))
		return nil
	}
	a.console.Success(a.tr("message.updated_from_to", result.CurrentVersion, result.LatestVersion))
	a.console.Detail(a.tr("label.installed_path"), result.InstalledPath)
	for _, note := range result.Notes {
		a.console.Note(a.localizeNote(note))
	}
	return nil
}

func (a *App) printInstallReport(result install.Result, completionResult shellcompletion.Result, completionErr error) {
	a.console.Success(a.tr("message.install_complete"))
	a.console.Section(a.tr("section.installation"))
	a.console.Detail(a.tr("label.binary"), result.TargetPath)
	if len(result.UpdatedFiles) > 0 {
		a.console.Section(a.tr("label.path_files_updated"))
		for _, file := range result.UpdatedFiles {
			a.console.Bullet(file)
		}
	} else if result.PathAlreadyPresent {
		a.console.Note(a.tr("label.path_already_configured"))
	}
	if completionErr == nil {
		a.console.Detail(a.tr("label.completion"), string(completionResult.Shell))
		for _, file := range completionResult.UpdatedFiles {
			a.console.Bullet(file)
		}
	} else {
		a.console.Warning(a.tr("message.completion_skipped", completionErr))
	}
	for _, note := range result.Notes {
		a.console.Note(a.localizeNote(note))
	}
	for _, note := range completionResult.Notes {
		if !strings.Contains(strings.ToLower(note), "installed") {
			a.console.Note(a.localizeNote(note))
		}
	}
	a.console.Note(fmt.Sprintf("%s: %s", a.tr("common.next"), a.tr("message.open_new_terminal")))
}

func (a *App) printUninstallReport(result install.Result, completionResult shellcompletion.Result, completionErr error) {
	a.console.Success(a.tr("message.uninstall_complete"))
	a.console.Section(a.tr("section.uninstall"))
	a.console.Detail(a.tr("label.binary"), result.TargetPath)
	if len(result.UpdatedFiles) > 0 {
		a.console.Section(a.tr("label.path_files_cleaned"))
		for _, file := range result.UpdatedFiles {
			a.console.Bullet(file)
		}
	}
	if completionErr == nil {
		a.console.Detail(a.tr("label.completion"), string(completionResult.Shell))
		for _, file := range completionResult.UpdatedFiles {
			a.console.Bullet(file)
		}
	} else {
		a.console.Warning(a.tr("message.completion_skipped", completionErr))
	}
	for _, note := range result.Notes {
		a.console.Note(a.localizeNote(note))
	}
	for _, note := range completionResult.Notes {
		if !strings.Contains(strings.ToLower(note), "removed") {
			a.console.Note(a.localizeNote(note))
		}
	}
	a.console.Note(fmt.Sprintf("%s: %s", a.tr("common.next"), a.tr("message.reopen_terminal_for_cleanup")))
}

func (a *App) projectConfigExists(repoRoot string) bool {
	if strings.TrimSpace(repoRoot) == "" {
		return false
	}
	_, err := os.Stat(a.store.ProjectPath(repoRoot))
	return err == nil
}

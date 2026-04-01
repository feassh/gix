package app

import (
	"context"
	"fmt"
	"io"

	"gix/internal/buildinfo"
	"gix/internal/config"
	"gix/internal/git"
	"gix/internal/ui"
)

type App struct {
	console *ui.Console
	store   *config.Store
	git     *git.Client
}

func New(in io.Reader, out io.Writer, err io.Writer) *App {
	store := config.NewStore()
	console := ui.NewConsole(in, out, err)
	if cfg, _, loadErr := store.LoadResolved(""); loadErr == nil {
		console.SetColorEnabled(cfg.UI.Color)
	}
	return &App{
		console: console,
		store:   store,
		git:     git.NewClient(in, out, err),
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.printRootHelp()
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		a.printRootHelp()
		return nil
	case "version":
		a.console.Println(buildinfo.Version)
		return nil
	case "init":
		return a.runInit(ctx, args[1:])
	case "install":
		return a.runInstall(ctx, args[1:])
	case "self-update":
		return a.runSelfUpdate(ctx, args[1:])
	case "uninstall":
		return a.runUninstall(ctx, args[1:])
	case "status", "st":
		return a.runStatus(ctx, args[1:])
	case "add":
		return a.runAdd(ctx, args[1:])
	case "commit":
		return a.runCommit(ctx, args[1:])
	case "push":
		return a.runPush(ctx, args[1:])
	case "ac":
		return a.runAC(ctx, args[1:])
	case "acp":
		return a.runACP(ctx, args[1:])
	case "sync":
		return a.runSync(ctx, args[1:])
	case "tag":
		return a.runTag(ctx, args[1:])
	case "branch":
		return a.runBranch(ctx, args[1:])
	case "repo":
		return a.runRepo(ctx, args[1:])
	case "config":
		return a.runConfig(ctx, args[1:])
	case "completion":
		return a.runCompletion(ctx, args[1:])
	case "__complete":
		return a.runComplete(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

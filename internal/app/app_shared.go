package app

import (
	"fmt"
	"strings"

	"gix/internal/config"
	"gix/internal/ui"
)

func (a *App) printRootHelp() {
	a.console.Title(a.tr("root.title"))
	a.console.BlankLine()
	a.console.Note(a.tr("root.help_intro"))
	a.console.BlankLine()

	for _, section := range helpSections() {
		a.console.Section(a.tr(section.TitleKey))
		details := make([]ui.Detail, 0, len(section.Commands))
		for _, command := range section.Commands {
			details = append(details, ui.Detail{
				Label: command.Usage,
				Value: a.tr(command.DescriptionKey),
			})
		}
		a.console.Details(details)
		a.console.BlankLine()
	}

	a.console.Section(a.tr("root.help_more_title"))
	a.console.Bullet(a.tr("root.help_more_help"))
	a.console.Bullet(a.tr("root.help_more_version"))
	a.console.Bullet(a.tr("root.help_more_completion"))
}

type helpSection struct {
	TitleKey string
	Commands []helpCommand
}

type helpCommand struct {
	Usage          string
	DescriptionKey string
}

func helpSections() []helpSection {
	return []helpSection{
		{
			TitleKey: "help.section.bootstrap",
			Commands: []helpCommand{
				{Usage: "gix init", DescriptionKey: "help.desc.init"},
				{Usage: "gix install", DescriptionKey: "help.desc.install"},
				{Usage: "gix uninstall", DescriptionKey: "help.desc.uninstall"},
				{Usage: "gix self-update", DescriptionKey: "help.desc.self_update"},
				{Usage: "gix self-update --check", DescriptionKey: "help.desc.self_update_check"},
			},
		},
		{
			TitleKey: "help.section.workspace",
			Commands: []helpCommand{
				{Usage: "gix status", DescriptionKey: "help.desc.status"},
				{Usage: "gix st", DescriptionKey: "help.desc.st"},
				{Usage: "gix add", DescriptionKey: "help.desc.add"},
				{Usage: "gix commit", DescriptionKey: "help.desc.commit"},
				{Usage: "gix push", DescriptionKey: "help.desc.push"},
				{Usage: "gix ac", DescriptionKey: "help.desc.ac"},
				{Usage: "gix acp", DescriptionKey: "help.desc.acp"},
				{Usage: "gix sync", DescriptionKey: "help.desc.sync"},
			},
		},
		{
			TitleKey: "help.section.tags",
			Commands: []helpCommand{
				{Usage: "gix tag create", DescriptionKey: "help.desc.tag_create"},
				{Usage: "gix tag list", DescriptionKey: "help.desc.tag_list"},
				{Usage: "gix tag push", DescriptionKey: "help.desc.tag_push"},
				{Usage: "gix tag delete", DescriptionKey: "help.desc.tag_delete"},
				{Usage: "gix tag checkout", DescriptionKey: "help.desc.tag_checkout"},
				{Usage: "gix tag branch", DescriptionKey: "help.desc.tag_branch"},
			},
		},
		{
			TitleKey: "help.section.branches",
			Commands: []helpCommand{
				{Usage: "gix branch list", DescriptionKey: "help.desc.branch_list"},
				{Usage: "gix branch new", DescriptionKey: "help.desc.branch_new"},
				{Usage: "gix branch switch", DescriptionKey: "help.desc.branch_switch"},
				{Usage: "gix branch delete", DescriptionKey: "help.desc.branch_delete"},
				{Usage: "gix branch rename", DescriptionKey: "help.desc.branch_rename"},
				{Usage: "gix branch cleanup", DescriptionKey: "help.desc.branch_cleanup"},
			},
		},
		{
			TitleKey: "help.section.repository",
			Commands: []helpCommand{
				{Usage: "gix repo info", DescriptionKey: "help.desc.repo_info"},
				{Usage: "gix repo remote", DescriptionKey: "help.desc.repo_remote"},
				{Usage: "gix repo set-default-remote", DescriptionKey: "help.desc.repo_set_default_remote"},
				{Usage: "gix repo set-main-branch", DescriptionKey: "help.desc.repo_set_main_branch"},
			},
		},
		{
			TitleKey: "help.section.config",
			Commands: []helpCommand{
				{Usage: "gix config init", DescriptionKey: "help.desc.config_init"},
				{Usage: "gix config get", DescriptionKey: "help.desc.config_get"},
				{Usage: "gix config set", DescriptionKey: "help.desc.config_set"},
				{Usage: "gix config list", DescriptionKey: "help.desc.config_list"},
			},
		},
		{
			TitleKey: "help.section.completion",
			Commands: []helpCommand{
				{Usage: "gix completion", DescriptionKey: "help.desc.completion"},
				{Usage: "gix completion uninstall", DescriptionKey: "help.desc.completion_uninstall"},
				{Usage: "gix completion print", DescriptionKey: "help.desc.completion_print"},
				{Usage: "gix completion zsh|bash|fish|powershell", DescriptionKey: "help.desc.completion_legacy"},
			},
		},
	}
}

func defaultRemote(cfg config.Config) string {
	return firstNonEmpty(cfg.Project.DefaultRemote, cfg.Push.DefaultRemote, "origin")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitArgs(args []string, pushFlags map[string]bool) ([]string, []string) {
	var commitArgs []string
	var pushArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		needsValue, isPushFlag := pushFlags[arg]
		if !isPushFlag {
			commitArgs = append(commitArgs, arg)
			continue
		}
		pushArgs = append(pushArgs, arg)
		if needsValue && i+1 < len(args) {
			pushArgs = append(pushArgs, args[i+1])
			i++
		}
	}
	return commitArgs, pushArgs
}

func usagef(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

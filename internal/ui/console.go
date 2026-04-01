package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"gix/internal/i18n"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Detail struct {
	Label string
	Value string
}

type Console struct {
	in           *bufio.Reader
	out          io.Writer
	err          io.Writer
	catalog      i18n.Catalog
	colorCapable bool
	colorEnabled bool
	styles       styles
}

type styles struct {
	title   lipgloss.Style
	section lipgloss.Style
	label   lipgloss.Style
	value   lipgloss.Style
	success lipgloss.Style
	warning lipgloss.Style
	note    lipgloss.Style
	error   lipgloss.Style
	prompt  lipgloss.Style
	bullet  lipgloss.Style
}

func NewConsole(in io.Reader, out io.Writer, err io.Writer) *Console {
	colorCapable := isTerminalWriter(out)
	colorEnabled := colorCapable && strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
	console := &Console{
		in:           bufio.NewReader(in),
		out:          out,
		err:          err,
		catalog:      i18n.NewCatalog(i18n.Detect()),
		colorCapable: colorCapable,
		colorEnabled: colorEnabled,
	}
	console.styles = buildStyles(colorEnabled)
	return console
}

func (c *Console) SetLocale(locale i18n.Locale) {
	c.catalog = i18n.NewCatalog(locale)
}

func (c *Console) SetColorEnabled(enabled bool) {
	c.colorEnabled = enabled && c.colorCapable && strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
	c.styles = buildStyles(c.colorEnabled)
}

func (c *Console) Locale() i18n.Locale {
	return c.catalog.Locale()
}

func (c *Console) T(key string, args ...any) string {
	return c.catalog.S(key, args...)
}

func (c *Console) Printf(format string, args ...any) {
	fmt.Fprintf(c.out, format, args...)
}

func (c *Console) Println(args ...any) {
	fmt.Fprintln(c.out, args...)
}

func (c *Console) Errorf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Fprintln(c.err, c.styles.error.Render(strings.TrimRight(message, "\n")))
}

func (c *Console) Title(text string) {
	fmt.Fprintln(c.out, c.styles.title.Render(text))
}

func (c *Console) Section(text string) {
	fmt.Fprintln(c.out, c.styles.section.Render(text))
}

func (c *Console) Success(text string) {
	fmt.Fprintln(c.out, c.styles.success.Render(text))
}

func (c *Console) Warning(text string) {
	fmt.Fprintln(c.out, c.styles.warning.Render(text))
}

func (c *Console) Note(text string) {
	fmt.Fprintln(c.out, c.styles.note.Render(text))
}

func (c *Console) Bullet(text string) {
	fmt.Fprintf(c.out, "%s %s\n", c.styles.bullet.Render("-"), c.styles.value.Render(text))
}

func (c *Console) BlankLine() {
	fmt.Fprintln(c.out)
}

func (c *Console) Detail(label string, value string) {
	c.Details([]Detail{{Label: label, Value: value}})
}

func (c *Console) Details(details []Detail) {
	if len(details) == 0 {
		return
	}
	maxWidth := 0
	for _, detail := range details {
		width := lipgloss.Width(detail.Label)
		if width > maxWidth {
			maxWidth = width
		}
	}
	for _, detail := range details {
		padding := strings.Repeat(" ", maxWidth-lipgloss.Width(detail.Label))
		label := c.styles.label.Render(detail.Label)
		value := c.styles.value.Render(detail.Value)
		fmt.Fprintf(c.out, "%s%s  %s\n", label, padding, value)
	}
}

func (c *Console) PromptLine(prompt string) (string, error) {
	if prompt != "" {
		fmt.Fprint(c.out, c.styles.prompt.Render(prompt))
	}
	line, err := c.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (c *Console) PromptMultiline(prompt string) (string, error) {
	if prompt != "" {
		fmt.Fprintln(c.out, c.styles.prompt.Render(prompt))
	}
	fmt.Fprintln(c.out, c.styles.note.Render(c.T("prompt.multiline_finish")))
	var lines []string
	for {
		line, err := c.in.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "." {
			break
		}
		lines = append(lines, line)
		if err == io.EOF {
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func (c *Console) PromptChoice(prompt string, defaultChoice string) (string, error) {
	if prompt != "" {
		fmt.Fprintln(c.out, c.styles.prompt.Render(prompt))
	}
	line, err := c.PromptLine("> ")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(line) == "" {
		return strings.ToUpper(defaultChoice), nil
	}
	return strings.ToUpper(strings.TrimSpace(line)), nil
}

func (c *Console) Confirm(prompt string, defaultYes bool) (bool, error) {
	suffix := " [y/N]: "
	defaultChoice := "N"
	if defaultYes {
		suffix = " [Y/n]: "
		defaultChoice = "Y"
	}
	choice, err := c.PromptLine(prompt + suffix)
	if err != nil {
		return false, err
	}
	if choice == "" {
		choice = defaultChoice
	}
	switch strings.ToLower(choice) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

func buildStyles(enabled bool) styles {
	if !enabled {
		return styles{
			title:   lipgloss.NewStyle(),
			section: lipgloss.NewStyle(),
			label:   lipgloss.NewStyle(),
			value:   lipgloss.NewStyle(),
			success: lipgloss.NewStyle(),
			warning: lipgloss.NewStyle(),
			note:    lipgloss.NewStyle(),
			error:   lipgloss.NewStyle(),
			prompt:  lipgloss.NewStyle(),
			bullet:  lipgloss.NewStyle(),
		}
	}
	return styles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		section: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("109")),
		value:   lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		success: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		warning: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		note:    lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		error:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
		prompt:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		bullet:  lipgloss.NewStyle().Foreground(lipgloss.Color("81")),
	}
}

func isTerminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

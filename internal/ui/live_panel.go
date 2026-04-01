package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const panelViewportHeight = 3

type StreamPanel struct {
	out          io.Writer
	enabled      bool
	width        int
	lastHeight   int
	catalog      func(string, ...any) string
	reasoning    strings.Builder
	content      strings.Builder
	hasReasoning bool
	styles       panelStyles
	mu           sync.Mutex
}

type panelStyles struct {
	reasoningBox lipgloss.Style
	contentBox   lipgloss.Style
	title        lipgloss.Style
	text         lipgloss.Style
}

func (c *Console) NewStreamPanel() *StreamPanel {
	if !c.colorCapable {
		return &StreamPanel{enabled: false}
	}
	width := 84
	if file, ok := c.out.(*os.File); ok {
		if termWidth, _, err := term.GetSize(int(file.Fd())); err == nil && termWidth > 0 {
			width = max(56, min(96, termWidth-4))
		}
	}
	return &StreamPanel{
		out:     c.out,
		enabled: true,
		width:   width,
		catalog: c.T,
		styles:  buildPanelStyles(c.colorEnabled),
	}
}

func (p *StreamPanel) OnReasoningDelta(delta string) {
	if delta == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hasReasoning = true
	p.reasoning.WriteString(delta)
	p.renderLocked()
}

func (p *StreamPanel) OnContentDelta(delta string) {
	if delta == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.content.WriteString(delta)
	p.renderLocked()
}

func (p *StreamPanel) OnComplete() {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastHeight > 0 {
		fmt.Fprintln(p.out)
		p.lastHeight = 0
	}
}

func (p *StreamPanel) renderLocked() {
	if !p.enabled {
		return
	}
	view := p.buildView()
	if p.lastHeight > 0 {
		clearPreviousBlock(p.out, p.lastHeight)
	}
	fmt.Fprint(p.out, view)
	p.lastHeight = lipgloss.Height(strings.TrimRight(view, "\n"))
}

func (p *StreamPanel) buildView() string {
	var sections []string
	if p.hasReasoning {
		sections = append(sections, p.renderBox(
			p.catalog("panel.title_thinking"),
			p.reasoning.String(),
			p.styles.reasoningBox,
		))
	}
	sections = append(sections, p.renderBox(
		p.catalog("panel.title_content"),
		p.content.String(),
		p.styles.contentBox,
	))
	return strings.Join(sections, "\n") + "\n"
}

func (p *StreamPanel) renderBox(title string, raw string, style lipgloss.Style) string {
	lines := latestWrappedLines(raw, p.width-6, panelViewportHeight)
	for len(lines) < panelViewportHeight {
		lines = append(lines, "")
	}
	body := p.styles.text.Render(strings.Join(lines, "\n"))
	titleBar := p.styles.title.Render(title)
	box := style.Width(p.width).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, box)
}

func buildPanelStyles(enabled bool) panelStyles {
	if !enabled {
		base := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
		return panelStyles{
			reasoningBox: base,
			contentBox:   base,
			title:        lipgloss.NewStyle().Bold(true),
			text:         lipgloss.NewStyle(),
		}
	}
	return panelStyles{
		reasoningBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),
		contentBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("45")).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("235")).
			Padding(0, 1),
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("117")),
		text: lipgloss.NewStyle(),
	}
}

func latestWrappedLines(raw string, width int, limit int) []string {
	if width <= 0 {
		width = 60
	}
	var lines []string
	for _, paragraph := range strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(paragraph)
		for len(runes) > width {
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		lines = append(lines, string(runes))
	}
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func clearPreviousBlock(out io.Writer, height int) {
	for i := 0; i < height; i++ {
		fmt.Fprint(out, "\x1b[1A\x1b[2K\r")
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

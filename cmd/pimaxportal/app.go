package main

import (
	"strings"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Spinner frames — reverse-mirror bounce: ◇ → ◆ → ❖ → ◆ → ◇
var spinnerFrames = []string{"◇", "◆", "❖"}

const spinnerInterval = 300 * time.Millisecond

// model is the root Bubbletea model for the TUI.
type model struct {
	sidebar      Sidebar
	screens      map[string]Screen
	activeScreen string
	width        int
	height       int
	program      *tea.Program
}

func initialModel() *model {
	entries := []SidebarEntry{
		{Key: "", Label: "Tools", IsHeader: true},
		{Key: "gpu", Label: "GPU Drivers"},
		{Key: "overclock", Label: "GPU Overclock"},
	}

	screens := map[string]Screen{
		"gpu":       NewGPUScreen(),
		"overclock": NewOCScreen(),
	}

	sb := NewSidebar(entries)
	sb.SetActive("gpu")

	return &model{
		sidebar:      sb,
		screens:      screens,
		activeScreen: "gpu",
	}
}

// Init starts the active screen.
func (m *model) Init() tea.Cmd {
	if s, ok := m.screens[m.activeScreen]; ok {
		return s.Init()
	}
	return nil
}

// Update handles messages and routes them to sidebar or active screen.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sidebar.SetHeight(msg.Height - 2)
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Always quit on ctrl+c
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		// Quit on q only if sidebar is focused
		if key == "q" && m.sidebar.Focused() {
			return m, tea.Quit
		}

		// Tab toggles sidebar focus
		if key == "tab" {
			m.sidebar.SetFocused(!m.sidebar.Focused())
			return m, nil
		}

		// Route to sidebar if focused
		if m.sidebar.Focused() {
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.Update(msg)
			return m, cmd
		}

		// Otherwise fall through to active screen

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			// Click in sidebar region?
			if msg.X < ui.SidebarWidth+2 {
				key := m.sidebar.EntryKeyAtY(msg.Y)
				if key != "" && key != m.activeScreen {
					m.activeScreen = key
					m.sidebar.SetActive(key)
					m.sidebar.SetFocused(false)
					return m, m.screens[m.activeScreen].Init()
				}
			}
		}
		return m, nil

	case ScreenSelectedMsg:
		if _, ok := m.screens[msg.Key]; ok {
			m.activeScreen = msg.Key
			m.sidebar.SetFocused(false)
			m.sidebar.SetActive(msg.Key)
			return m, m.screens[m.activeScreen].Init()
		}
		return m, nil
	}

	// Route all other messages to the active screen
	if s, ok := m.screens[m.activeScreen]; ok {
		updated, cmd := s.Update(msg)
		m.screens[m.activeScreen] = updated
		return m, cmd
	}

	return m, nil
}

// View renders the TUI with sidebar and active screen.
func (m *model) View() string {
	// Render sidebar
	sidebarView := m.sidebar.View()

	// Frame chrome: border (1+1) + padding (2+2) = 6 horizontal, 4 vertical
	frameChrome := ui.ContentFrameStyle.GetHorizontalFrameSize()
	frameVChrome := ui.ContentFrameStyle.GetVerticalFrameSize()
	innerW := contentWidth - frameChrome
	if innerW < 20 {
		innerW = 20
	}
	innerH := m.height - frameVChrome - 2 // 2 for footer
	if innerH < 10 {
		innerH = 10
	}

	// Render active screen inside a bordered frame
	var screenView string
	if s, ok := m.screens[m.activeScreen]; ok {
		screenView = s.View(innerW, innerH)
	}
	framedContent := ui.ContentFrameStyle.Width(contentWidth).Height(innerH).Render(screenView)

	// Join sidebar and content horizontally
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, framedContent)

	// Footer
	var footer string
	if m.sidebar.Focused() {
		footer = ui.FooterStyle.Render("↑↓ navigate  enter select  tab content  q quit")
	} else {
		help := ""
		if s, ok := m.screens[m.activeScreen]; ok {
			help = s.FooterHelp()
		}
		if help != "" {
			footer = ui.FooterStyle.Render(help + "  tab menu")
		} else {
			footer = ui.FooterStyle.Render("tab menu")
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, footer)
}

// ---- Shared helpers used by screens ----

const contentWidth = 50

// innerWidth returns the usable width inside the content frame.
func innerWidth() int {
	return contentWidth - ui.ContentFrameStyle.GetHorizontalFrameSize()
}

// center places text horizontally centered within the inner width.
func center(s string) string {
	return lipgloss.PlaceHorizontal(innerWidth(), lipgloss.Center, s)
}

// renderProgressBar draws a text-based progress bar.
func renderProgressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	empty := width - filled
	return ui.ProgressStyle.Render(strings.Repeat("█", filled)) +
		ui.FooterStyle.Render(strings.Repeat("░", empty))
}

// shortenError trims verbose error messages to fit in a popup.
func shortenError(err error) string {
	msg := err.Error()
	// Take only the last meaningful part after the last colon
	if idx := strings.LastIndex(msg, ": "); idx >= 0 && idx < len(msg)-2 {
		short := msg[idx+2:]
		// But keep context prefix if short part is too vague
		if len(short) < 10 {
			msg = msg // keep full
		} else {
			msg = short
		}
	}
	// Truncate to fit popup (~40 chars per line, max 3 lines)
	const maxLen = 120
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}
	return msg
}

// wordWrap breaks a string into lines of at most maxWidth characters.
func wordWrap(s string, maxWidth int) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > maxWidth {
			// Find last space before maxWidth
			idx := strings.LastIndex(line[:maxWidth], " ")
			if idx <= 0 {
				idx = maxWidth
			}
			lines = append(lines, line[:idx])
			line = strings.TrimSpace(line[idx:])
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n  ")
}

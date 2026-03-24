package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Application states
type appState int

const (
	stateIdle         appState = iota
	stateRunning
	stateRootWait
	stateRebootPrompt
	stateError
	stateDone
)

// Messages
type deviceTickMsg struct{}
type progressMsg commands.ProgressMsg
type pipelineDoneMsg struct{ err error }

// Spinner frames — reverse-mirror bounce: ◇ → ◆ → ❖ → ◆ → ◇
var spinnerFrames = []string{"◇", "◆", "❖"}
const spinnerInterval = 300 * time.Millisecond

// model is the root Bubbletea model for the TUI.
type model struct {
	state        appState
	device       adb.DeviceInfo
	log          ui.ProgressLog
	program      *tea.Program
	cancel       context.CancelFunc
	spinnerIdx   int
	spinnerDir   int     // +1 forward, -1 backward (bounce)
	statusLine   string  // current pipeline step text
	statusPct    float64 // current progress 0.0-1.0, -1 for indeterminate
	errorMsg     string  // error to show in popup
	buttonRow    int    // Y position of button for mouse click detection
}

func initialModel() *model {
	// Do an initial device poll
	info, _ := adb.GetDeviceInfo()
	return &model{
		state:      stateIdle,
		device:     info,
		spinnerDir: 1,
	}
}

// Init starts the device polling ticker.
func (m *model) Init() tea.Cmd {
	return tickDevice()
}

// tickDevice returns a Cmd that fires a deviceTickMsg after 3 seconds.
func tickDevice() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return deviceTickMsg{}
	})
}

// Update handles messages and key events.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			// Click on button row triggers the same as Enter
			if m.state == stateIdle && msg.Y >= m.buttonRow && msg.Y <= m.buttonRow+1 {
				return m, m.startPipeline()
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case deviceTickMsg:
		if m.state == stateIdle {
			info, _ := adb.GetDeviceInfo()
			m.device = info
		}
		return m, tickDevice()

	case progressMsg:
		return m.handleProgress(commands.ProgressMsg(msg))

	case pipelineDoneMsg:
		return m.handlePipelineDone(msg)

	case spinTickMsg:
		if m.state == stateRunning || m.state == stateRootWait {
			m.advanceSpinner()
			return m, m.spinTick()
		}
		return m, nil
	}

	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case stateIdle:
		switch key {
		case "enter":
			return m, m.startPipeline()
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			info, _ := adb.GetDeviceInfo()
			m.device = info
			return m, nil
		}

	case stateRunning:
		switch key {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}

	case stateRootWait:
		switch key {
		case "q", "ctrl+c", "c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}

	case stateRebootPrompt:
		switch key {
		case "y":
			m.log.Add(ui.ProgressStyle.Render("Rebooting device..."))
			m.state = stateDone
			_ = adb.Reboot()
			m.log.Add(ui.SuccessStyle.Render("Reboot command sent. Device will restart."))
			return m, tea.Quit
		case "n":
			m.log.Add(ui.SuccessStyle.Render("Skipped reboot. Reboot manually to apply changes."))
			m.state = stateDone
			return m, tea.Quit
		}

	case stateError:
		// Any key dismisses the error popup
		m.state = stateIdle
		m.errorMsg = ""
		return m, nil

	case stateDone:
		return m, tea.Quit
	}

	return m, nil
}

func (m *model) handleProgress(msg commands.ProgressMsg) (tea.Model, tea.Cmd) {
	switch msg.Text {
	case "ROOT_CHECK_WAITING":
		m.state = stateRootWait
		m.spinnerIdx = 0
		return m, m.spinTick()

	case "ROOT_CHECK_GRANTED":
		m.state = stateRunning
		m.log.Add(ui.SuccessStyle.Render("Root access granted."))
		return m, nil

	case "ROOT_CHECK_TIMEOUT":
		m.showError("Root access timed out.\nPlease try again.")
		return m, nil

	case "INSTALL_COMPLETE":
		m.state = stateRebootPrompt
		m.log.Add(ui.SuccessStyle.Render("Module installed successfully!"))
		return m, nil

	default:
		if msg.Percent == 1.0 {
			// Step completed — move to log only if different from last logged line
			entry := ui.SuccessStyle.Render("■") + " " + ui.ProgressStyle.Render(msg.Text)
			if len(m.log.Lines) == 0 || m.log.Lines[len(m.log.Lines)-1] != entry {
				m.log.Add(entry)
			}
			m.statusLine = ""
			m.statusPct = -1
		} else {
			// Ongoing — update the live status line (not logged)
			m.statusLine = msg.Text
			m.statusPct = msg.Percent
		}
		return m, nil
	}
}

type spinTickMsg struct{}

func (m *model) advanceSpinner() {
	m.spinnerIdx += m.spinnerDir
	if m.spinnerIdx >= len(spinnerFrames)-1 {
		m.spinnerDir = -1
	} else if m.spinnerIdx <= 0 {
		m.spinnerDir = 1
	}
}

func (m *model) spinnerFrame() string {
	idx := m.spinnerIdx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(spinnerFrames) {
		idx = len(spinnerFrames) - 1
	}
	return spinnerFrames[idx]
}

func (m *model) spinTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinTickMsg{}
	})
}

func (m *model) handlePipelineDone(msg pipelineDoneMsg) (tea.Model, tea.Cmd) {
	m.statusLine = ""
	m.statusPct = -1
	if msg.err != nil {
		// Suppress context canceled anywhere in the error chain (user-initiated cancel)
		if errors.Is(msg.err, context.Canceled) {
			m.state = stateIdle
			return m, nil
		}
		m.errorMsg = shortenError(msg.err)
		m.state = stateError
		return m, nil
	}
	if m.state != stateRebootPrompt {
		m.state = stateIdle
	}
	return m, nil
}

func (m *model) showError(msg string) {
	m.errorMsg = msg
	m.state = stateError
	m.statusLine = ""
}

// startPipeline launches the build+install pipeline in a goroutine.
func (m *model) startPipeline() tea.Cmd {
	m.state = stateRunning
	m.log.Clear()
	m.statusLine = ""
	m.statusPct = -1
	m.spinnerIdx = 0
	m.spinnerDir = 1

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	p := m.program
	pipelineCmd := func() tea.Msg {
		go func() {
			send := func(msg commands.ProgressMsg) {
				if p != nil {
					p.Send(progressMsg(msg))
				}
			}

			// Run Build
			buildCmd := &commands.BuildCommand{}
			err := buildCmd.Run(ctx, send)
			if err != nil {
				if p != nil {
					p.Send(pipelineDoneMsg{err: err})
				}
				return
			}

			// Check device and run Install if connected
			info, _ := adb.GetDeviceInfo()
			if info.Connected {
				installCmd := &commands.InstallCommand{}
				err = installCmd.Run(ctx, send)
				if err != nil {
					if p != nil {
						p.Send(pipelineDoneMsg{err: err})
					}
					return
				}
			} else {
				send(commands.ProgressMsg{
					Text:    "No device connected. Module built but not installed.",
					Percent: 1.0,
				})
			}

			if p != nil {
				p.Send(pipelineDoneMsg{err: nil})
			}
		}()
		return nil
	}
	return tea.Batch(pipelineCmd, m.spinTick())
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

const contentWidth = 46

// center places text horizontally centered within contentWidth.
func center(s string) string {
	return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, s)
}

// View renders the TUI.
func (m *model) View() string {
	var b strings.Builder

	// 1. Logo (centered)
	art := center(ui.TitleStyle.Render(ui.LogoArt))
	subtitle := center(ui.TitleStyle.Render(ui.LogoSubtitle))
	logo := art + "\n" + subtitle
	b.WriteString(logo)
	b.WriteString("\n\n")

	// 2. Device info (left-aligned)
	deviceInfo := ui.RenderDeviceInfo(m.device)
	b.WriteString(deviceInfo)
	b.WriteString("\n\n")

	// Track button row: frame border(1) + padding(1) + logo lines + blank + device lines + blank
	linesSoFar := 2 + strings.Count(logo, "\n") + 1 + strings.Count(deviceInfo, "\n") + 1
	m.buttonRow = linesSoFar

	// 3. Action button (centered)
	b.WriteString(center(m.renderButton()))
	b.WriteString("\n")

	// 4. Overlays
	if m.state == stateRootWait {
		b.WriteString("\n")
		b.WriteString(m.renderRootWaitOverlay())
		b.WriteString("\n")
	}
	if m.state == stateError {
		b.WriteString("\n")
		b.WriteString(m.renderErrorPopup())
		b.WriteString("\n")
	}

	// 5. Separator
	b.WriteString("\n" + ui.SeparatorStyle.Render(strings.Repeat("─", contentWidth)) + "\n")

	// 6. Progress log + live status
	logContent := m.log.Render()
	if logContent != "" {
		b.WriteString("\n")
		b.WriteString(logContent)
	}
	if m.statusLine != "" && (m.state == stateRunning || m.state == stateRootWait) {
		b.WriteString("\n")
		b.WriteString(ui.ProgressStyle.Render(m.statusLine))
		if m.statusPct >= 0 && m.statusPct < 1.0 {
			b.WriteString("\n")
			b.WriteString(renderProgressBar(m.statusPct, contentWidth))
		}
	}
	b.WriteString("\n")

	// 7. Footer hotkeys (centered)
	b.WriteString("\n")
	b.WriteString(center(m.renderFooter()))

	// Wrap everything in a frame
	return ui.FrameStyle.Render(b.String()) + "\n"
}

func (m *model) renderButton() string {
	switch m.state {
	case stateIdle:
		if m.device.Connected {
			return ui.ButtonStyle.Render("  Build & Install GPU Drivers  ")
		}
		return ui.ButtonStyle.Render("  Build GPU Driver Module  ")
	case stateRunning:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(m.spinnerFrame()) + " Running  ")
	case stateRootWait:
		return ui.ButtonDimStyle.Render("  Waiting for Root...  ")
	case stateRebootPrompt:
		return ui.ButtonStyle.Render("  Reboot Device?  y/n  ")
	case stateError:
		return ui.ButtonDimStyle.Render("  Error  ")
	case stateDone:
		return ui.SuccessStyle.Render("  Done!  ")
	}
	return ""
}

func (m *model) renderFooter() string {
	switch m.state {
	case stateIdle:
		return ui.FooterStyle.Render("enter start  r refresh  q quit")
	case stateRunning:
		return ui.FooterStyle.Render("q cancel")
	case stateRootWait:
		return ui.FooterStyle.Render("c cancel")
	case stateRebootPrompt:
		return ui.FooterStyle.Render("y reboot  n skip")
	case stateError:
		return ui.FooterStyle.Render("any key dismiss")
	default:
		return ""
	}
}

func (m *model) renderErrorPopup() string {
	wrapped := wordWrap(m.errorMsg, 36)
	body := lipgloss.PlaceHorizontal(36, lipgloss.Center, wrapped)
	popup := ui.BoxStyle.Render("\n" + body + "\n")
	return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, popup)
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

func (m *model) renderRootWaitOverlay() string {
	frame := m.spinnerFrame()

	content := "\n" +
		"  Magisk needs to grant root (su) access\n" +
		"  to the ADB shell on your device.\n" +
		"\n" +
		"  On your Pimax Portal screen:\n" +
		"  1. Look for the Magisk superuser prompt\n" +
		"  2. Tap \"Grant\" or \"Allow\"\n" +
		"\n" +
		"  If no prompt appears:\n" +
		"  • Open Magisk app → Settings\n" +
		"  • Ensure \"Superuser\" is enabled\n" +
		"\n" +
		"  Waiting for root access...  " + ui.SpinnerStyle.Render(frame) + "\n" +
		"\n" +
		"  [c] Cancel"

	return ui.BoxStyle.Render(ui.TitleStyle.Render("Root Access Required") + content)
}

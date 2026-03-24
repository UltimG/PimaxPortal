package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Application states
type appState int

const (
	stateIdle         appState = iota
	stateRunning
	stateRootWait
	stateRebootPrompt
	stateDone
)

// Messages
type deviceTickMsg struct{}
type progressMsg commands.ProgressMsg
type pipelineDoneMsg struct{ err error }

// Spinner frames for root wait overlay
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// model is the root Bubbletea model for the TUI.
type model struct {
	state      appState
	device     adb.DeviceInfo
	log        ui.ProgressLog
	program    *tea.Program
	cancel     context.CancelFunc
	spinnerIdx int
}

func initialModel() *model {
	// Do an initial device poll
	info, _ := adb.GetDeviceInfo()
	return &model{
		state:  stateIdle,
		device: info,
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
		if m.state == stateRootWait {
			m.spinnerIdx++
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
		m.state = stateIdle
		m.log.Add(ui.ErrorStyle.Render("Root access timed out. Please try again."))
		return m, nil

	case "INSTALL_COMPLETE":
		m.state = stateRebootPrompt
		m.log.Add(ui.SuccessStyle.Render("Module installed successfully!"))
		return m, nil

	default:
		if msg.Percent == 1.0 {
			m.log.Add(ui.SuccessStyle.Render(msg.Text))
		} else {
			m.log.Add(ui.ProgressStyle.Render(msg.Text))
		}
		return m, nil
	}
}

type spinTickMsg struct{}

func (m *model) spinTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinTickMsg{}
	})
}

func (m *model) handlePipelineDone(msg pipelineDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Suppress context canceled (user-initiated cancel)
		if msg.err == context.Canceled {
			m.state = stateIdle
			m.log.Add(ui.ErrorStyle.Render("Cancelled."))
			return m, nil
		}
		m.state = stateIdle
		m.log.Add(ui.ErrorStyle.Render(fmt.Sprintf("Error: %v", msg.err)))
		return m, nil
	}
	// If we're in rebootPrompt, the pipeline finished successfully and
	// the INSTALL_COMPLETE message already moved us to rebootPrompt.
	// If no install happened (build-only), go back to idle.
	if m.state != stateRebootPrompt {
		m.state = stateIdle
	}
	return m, nil
}

// startPipeline launches the build+install pipeline in a goroutine.
func (m *model) startPipeline() tea.Cmd {
	m.state = stateRunning
	m.log.Clear()
	m.log.Add(ui.ProgressStyle.Render("Starting pipeline..."))

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	p := m.program
	return func() tea.Msg {
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
}

// View renders the TUI.
func (m *model) View() string {
	var b strings.Builder

	// 1. Logo
	b.WriteString(ui.TitleStyle.Render(ui.Logo))
	b.WriteString("\n\n")

	// 2. Device info
	b.WriteString(ui.RenderDeviceInfo(m.device))
	b.WriteString("\n\n")

	// 3. Action button
	b.WriteString(m.renderButton())
	b.WriteString("\n")

	// 4. Root wait overlay (only in rootWait state)
	if m.state == stateRootWait {
		b.WriteString("\n")
		b.WriteString(m.renderRootWaitOverlay())
		b.WriteString("\n")
	}

	// 5. Separator
	b.WriteString("\n")
	b.WriteString(ui.DeviceLabelStyle.Render(strings.Repeat("─", 50)))
	b.WriteString("\n\n")

	// 6. Progress log
	b.WriteString(m.log.Render())
	b.WriteString("\n")

	return b.String()
}

func (m *model) renderButton() string {
	switch m.state {
	case stateIdle:
		if m.device.Connected {
			return ui.ButtonStyle.Render("[ Build & Install ]") +
				"  " + ui.DeviceLabelStyle.Render("[Enter] Start  [r] Refresh  [q] Quit")
		}
		return ui.ButtonStyle.Render("[ Build Module ]") +
			"  " + ui.DeviceLabelStyle.Render("[Enter] Start  [r] Refresh  [q] Quit")
	case stateRunning:
		return ui.ButtonDimStyle.Render("[ Running... ]") +
			"  " + ui.DeviceLabelStyle.Render("[q] Cancel")
	case stateRootWait:
		return ui.ButtonDimStyle.Render("[ Waiting for Root... ]") +
			"  " + ui.DeviceLabelStyle.Render("[c] Cancel")
	case stateRebootPrompt:
		return ui.ButtonStyle.Render("[ Reboot Device? ]") +
			"  " + ui.DeviceLabelStyle.Render("[y] Reboot  [n] Skip")
	case stateDone:
		return ui.ButtonDimStyle.Render("[ Done ]")
	}
	return ""
}

func (m *model) renderRootWaitOverlay() string {
	frame := spinnerFrames[m.spinnerIdx%len(spinnerFrames)]

	content := fmt.Sprintf(`
  Magisk needs to grant root (su) access
  to the ADB shell on your device.

  On your Pimax Portal screen:
  1. Look for the Magisk superuser prompt
  2. Tap "Grant" or "Allow"

  If no prompt appears:
  • Open Magisk app → Settings
  • Ensure "Superuser" is enabled

  Waiting for root access...  %s

  [c] Cancel`, frame)

	return ui.BoxStyle.Render(ui.TitleStyle.Render("Root Access Required") + content)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Overclock screen states
type ocState int

const (
	ocIdle         ocState = iota
	ocRunning
	ocRebootPrompt
	ocError
	ocDone
)

// Overclock-specific message types to avoid conflicts with other screens.
type ocProgressMsg commands.ProgressMsg
type ocPipelineDoneMsg struct{ err error }
type ocDeviceTickMsg struct{}
type ocSpinTickMsg struct{}

// OCScreen implements the Screen interface for the GPU overclock flow.
type OCScreen struct {
	state      ocState
	device     adb.DeviceInfo
	log        ui.ProgressLog
	program    *tea.Program
	cancel     context.CancelFunc
	spinnerIdx int
	spinnerDir int     // +1 forward, -1 backward (bounce)
	statusLine string  // current pipeline step text
	statusPct  float64 // current progress 0.0-1.0, -1 for indeterminate
	errorMsg   string  // error to show in popup
	buttonRow  int     // Y position of button for mouse click detection

	cursor    int    // selected preset index
	activeHz  uint32 // current GPU frequency read from sysfs (0 if unknown)
}

// NewOCScreen creates a new OCScreen with an initial device poll.
func NewOCScreen() *OCScreen {
	info, _ := adb.GetDeviceInfo()
	s := &OCScreen{
		state:      ocIdle,
		device:     info,
		spinnerDir: 1,
		statusPct:  -1,
	}
	s.refreshFreq()
	return s
}

// refreshFreq reads the current GPU frequency from sysfs if device is connected.
func (s *OCScreen) refreshFreq() {
	if s.device.Connected {
		freq, err := commands.ReadCurrentGPUFreq()
		if err == nil {
			s.activeHz = freq
		}
	} else {
		s.activeHz = 0
	}
}

// SetProgram provides the tea.Program reference needed for goroutine
// communication (sending messages from pipeline goroutines).
func (s *OCScreen) SetProgram(p *tea.Program) {
	s.program = p
}

// Key returns the unique identifier for this screen.
func (s *OCScreen) Key() string {
	return "overclock"
}

// Title returns the display title shown in the sidebar.
func (s *OCScreen) Title() string {
	return "GPU Overclock"
}

// Init starts the device polling ticker.
func (s *OCScreen) Init() tea.Cmd {
	return s.tickDevice()
}

func (s *OCScreen) tickDevice() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ocDeviceTickMsg{}
	})
}

// Update handles messages when this screen is active.
func (s *OCScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			onButton := msg.Y >= s.buttonRow && msg.Y <= s.buttonRow+1
			switch s.state {
			case ocIdle:
				if onButton {
					return s, s.startPipeline()
				}
			case ocRebootPrompt:
				if onButton {
					mid := contentWidth / 2
					if msg.X < mid+ui.SidebarWidth {
						return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
					}
					return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
				}
			case ocError:
				return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			}
		}
		return s, nil

	case tea.KeyMsg:
		return s.handleKey(msg)

	case ocDeviceTickMsg:
		if s.state == ocIdle {
			info, _ := adb.GetDeviceInfo()
			s.device = info
			s.refreshFreq()
		}
		return s, s.tickDevice()

	case ocProgressMsg:
		return s.handleProgress(commands.ProgressMsg(msg))

	case ocPipelineDoneMsg:
		return s.handlePipelineDone(msg)

	case ocSpinTickMsg:
		if s.state == ocRunning {
			s.advanceSpinner()
			return s, s.spinTick()
		}
		return s, nil
	}

	return s, nil
}

func (s *OCScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.state {
	case ocIdle:
		switch key {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil
		case "down", "j":
			if s.cursor < len(commands.Presets)-1 {
				s.cursor++
			}
			return s, nil
		case "enter":
			return s, s.startPipeline()
		case "r":
			info, _ := adb.GetDeviceInfo()
			s.device = info
			s.refreshFreq()
			return s, nil
		}

	case ocRunning:
		switch key {
		case "q", "ctrl+c":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		}

	case ocRebootPrompt:
		switch key {
		case "y":
			s.log.Add(ui.ProgressStyle.Render("Rebooting device..."))
			s.state = ocDone
			_ = adb.Reboot()
			s.log.Add(ui.SuccessStyle.Render("Reboot command sent. Device will restart."))
			return s, tea.Quit
		case "n":
			s.log.Add(ui.SuccessStyle.Render("Skipped reboot. Reboot manually to apply changes."))
			s.state = ocDone
			return s, nil
		}

	case ocError:
		// Any key dismisses the error popup
		s.state = ocIdle
		s.errorMsg = ""
		return s, nil

	case ocDone:
		// Any key returns to idle
		s.state = ocIdle
		return s, nil
	}

	return s, nil
}

func (s *OCScreen) handleProgress(msg commands.ProgressMsg) (Screen, tea.Cmd) {
	if msg.Percent == 1.0 {
		// Step completed — move to log only if different from last logged line
		entry := ui.SuccessStyle.Render("■") + " " + ui.ProgressStyle.Render(msg.Text)
		if len(s.log.Lines) == 0 || s.log.Lines[len(s.log.Lines)-1] != entry {
			s.log.Add(entry)
		}
		s.statusLine = ""
		s.statusPct = -1
	} else {
		// Ongoing — update the live status line (not logged)
		s.statusLine = msg.Text
		s.statusPct = msg.Percent
	}
	return s, nil
}

func (s *OCScreen) handlePipelineDone(msg ocPipelineDoneMsg) (Screen, tea.Cmd) {
	s.statusLine = ""
	s.statusPct = -1
	if msg.err != nil {
		// Suppress context canceled anywhere in the error chain (user-initiated cancel)
		if errors.Is(msg.err, context.Canceled) {
			s.state = ocIdle
			return s, nil
		}
		s.errorMsg = shortenError(msg.err)
		s.state = ocError
		return s, nil
	}
	if s.state != ocRebootPrompt {
		s.state = ocRebootPrompt
		s.log.Add(ui.SuccessStyle.Render("Operation complete!"))
	}
	return s, nil
}

func (s *OCScreen) showError(msg string) {
	s.errorMsg = msg
	s.state = ocError
	s.statusLine = ""
}

// selectedPreset returns the currently cursor-selected preset.
func (s *OCScreen) selectedPreset() commands.OCPreset {
	if s.cursor >= 0 && s.cursor < len(commands.Presets) {
		return commands.Presets[s.cursor]
	}
	return commands.Presets[0]
}

// isRestoring returns true if the selected preset is stock (855 MHz) and the
// device is currently running at a different frequency.
func (s *OCScreen) isRestoring() bool {
	preset := s.selectedPreset()
	return preset.FreqHz == commands.Presets[0].FreqHz && s.activeHz != 0 && s.activeHz != preset.FreqHz
}

// startPipeline launches the overclock or restore pipeline in a goroutine.
func (s *OCScreen) startPipeline() tea.Cmd {
	if !s.device.Connected {
		return nil
	}

	preset := s.selectedPreset()

	// Already at selected frequency — nothing to do
	if s.activeHz != 0 && preset.FreqHz == s.activeHz {
		return nil
	}

	s.state = ocRunning
	s.log.Clear()
	s.statusLine = ""
	s.statusPct = -1
	s.spinnerIdx = 0
	s.spinnerDir = 1

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	p := s.program
	restoring := s.isRestoring()
	pipelineCmd := func() tea.Msg {
		go func() {
			send := func(msg commands.ProgressMsg) {
				if p != nil {
					p.Send(ocProgressMsg(msg))
				}
			}

			var err error
			if restoring {
				err = commands.RestoreStock(ctx, send)
			} else {
				err = commands.RunOverclock(ctx, preset, send)
			}

			if p != nil {
				p.Send(ocPipelineDoneMsg{err: err})
			}
		}()
		return nil
	}
	return tea.Batch(pipelineCmd, s.spinTick())
}

// Spinner

func (s *OCScreen) advanceSpinner() {
	s.spinnerIdx += s.spinnerDir
	if s.spinnerIdx >= len(spinnerFrames)-1 {
		s.spinnerDir = -1
	} else if s.spinnerIdx <= 0 {
		s.spinnerDir = 1
	}
}

func (s *OCScreen) spinnerFrame() string {
	idx := s.spinnerIdx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(spinnerFrames) {
		idx = len(spinnerFrames) - 1
	}
	return spinnerFrames[idx]
}

func (s *OCScreen) spinTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return ocSpinTickMsg{}
	})
}

// View renders the overclock screen content within the given dimensions.
func (s *OCScreen) View(width, height int) string {
	var b strings.Builder

	// Use the smaller of provided width and contentWidth for layout
	w := contentWidth
	if width > 0 && width < w {
		w = width
	}

	// 1. Logo (centered)
	art := center(ui.TitleStyle.Render(ui.LogoArt))
	subtitle := center(ui.TitleStyle.Render(ui.LogoSubtitle))
	logo := art + "\n" + subtitle
	b.WriteString(logo)
	b.WriteString("\n\n")

	// 2. Device info (left-aligned)
	deviceInfo := ui.RenderDeviceInfo(s.device)
	b.WriteString(deviceInfo)
	b.WriteString("\n\n")

	// 3. Preset selector (tree-style)
	b.WriteString(s.renderPresetSelector())
	b.WriteString("\n\n")

	// Track button row for mouse click detection
	linesSoFar := 2 + strings.Count(logo, "\n") + 1 + strings.Count(deviceInfo, "\n") + 1 +
		strings.Count(s.renderPresetSelector(), "\n") + 2
	s.buttonRow = linesSoFar

	// 4. Action button (centered)
	b.WriteString(center(s.renderButton()))
	b.WriteString("\n")

	// 5. Error popup if in error state
	if s.state == ocError {
		b.WriteString("\n")
		b.WriteString(s.renderErrorPopup())
		b.WriteString("\n")
	}

	// 6. Separator
	b.WriteString("\n" + ui.SeparatorStyle.Render(strings.Repeat("─", w)) + "\n")

	// 7. Progress log + live status line + progress bar
	logContent := s.log.Render()
	if logContent != "" {
		b.WriteString("\n")
		b.WriteString(logContent)
	}
	if s.statusLine != "" && s.state == ocRunning {
		b.WriteString("\n")
		b.WriteString(ui.ProgressStyle.Render(s.statusLine))
		if s.statusPct >= 0 && s.statusPct < 1.0 {
			b.WriteString("\n")
			b.WriteString(renderProgressBar(s.statusPct, w))
		}
	}
	b.WriteString("\n")

	return b.String()
}

// renderPresetSelector renders the tree-style preset list.
func (s *OCScreen) renderPresetSelector() string {
	var b strings.Builder
	presets := commands.Presets
	for i, p := range presets {
		// Tree connector
		connector := "├"
		if i == len(presets)-1 {
			connector = "└"
		}
		connectorStr := ui.TreeConnectorStyle.Render(connector)

		// Active indicator: ■ for active frequency, □ otherwise
		isActive := s.activeHz != 0 && p.FreqHz == s.activeHz
		var indicator string
		if isActive {
			indicator = ui.SidebarActiveStyle.Render("■")
		} else {
			indicator = ui.TreeConnectorStyle.Render("□")
		}

		// Label styling: cursor gets bold white, active gets bold white, rest are dim
		label := p.Name
		if i == s.cursor {
			label = ui.SidebarCursorStyle.Render(label)
		} else if isActive {
			label = ui.SidebarActiveStyle.Render(label)
		} else {
			label = ui.SidebarItemStyle.Render(label)
		}

		b.WriteString(fmt.Sprintf("  %s %s %s", connectorStr, indicator, label))
		if i < len(presets)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderButton returns the action button text based on state and selection.
func (s *OCScreen) renderButton() string {
	switch s.state {
	case ocIdle:
		if !s.device.Connected {
			return ui.ButtonDimStyle.Render("  Connect Device to Start  ")
		}
		preset := s.selectedPreset()
		if s.activeHz != 0 && preset.FreqHz == s.activeHz {
			return ui.ButtonDimStyle.Render("  Already Active  ")
		}
		if s.isRestoring() {
			return ui.ButtonStyle.Render("  Restore Stock  ")
		}
		return ui.ButtonStyle.Render("  Apply Overclock  ")
	case ocRunning:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(s.spinnerFrame()) + " Running  ")
	case ocRebootPrompt:
		return ui.ButtonStyle.Render("  Reboot Device?  y/n  ")
	case ocError:
		return ui.ButtonDimStyle.Render("  Error  ")
	case ocDone:
		return ui.SuccessStyle.Render("  Done!  ")
	}
	return ""
}

// FooterHelp returns state-appropriate key hints.
func (s *OCScreen) FooterHelp() string {
	switch s.state {
	case ocIdle:
		return "↑↓ select  enter apply  r refresh"
	case ocRunning:
		return "q cancel"
	case ocRebootPrompt:
		return "y reboot  n skip"
	case ocError:
		return "any key dismiss"
	default:
		return ""
	}
}

// Rendering helpers

func (s *OCScreen) renderErrorPopup() string {
	wrapped := wordWrap(s.errorMsg, 36)
	body := lipgloss.PlaceHorizontal(36, lipgloss.Center, wrapped)
	popup := ui.BoxStyle.Render("\n" + body + "\n")
	return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, popup)
}

// Compile-time check that OCScreen implements Screen.
var _ Screen = (*OCScreen)(nil)

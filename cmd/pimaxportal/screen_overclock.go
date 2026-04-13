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

// Overclock screen states
type ocState int

const (
	ocIdle ocState = iota
	ocRunning
	ocRebootPrompt
	ocError
	ocDone
)

// Overclock-specific message types to avoid cross-screen message delivery
// when the user switches tabs mid-pipeline.
type ocProgressMsg commands.ProgressMsg
type ocPipelineDoneMsg struct{ err error }
type ocDeviceTickMsg struct{}
type ocSpinTickMsg struct{}

// OCScreen implements the Screen interface for the GPU overclock flow.
type OCScreen struct {
	baseScreen
	state    ocState
	cursor   int    // selected preset index
	activeHz uint32 // current GPU frequency read from sysfs (0 if unknown)
}

// NewOCScreen creates a new OCScreen with an initial device poll.
func NewOCScreen() *OCScreen {
	info, _ := adb.GetDeviceInfo()
	s := &OCScreen{state: ocIdle}
	s.device = info
	s.spinnerDir = 1
	s.statusPct = -1
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

// Key returns the unique identifier for this screen.
func (s *OCScreen) Key() string { return "overclock" }

// Title returns the display title shown in the sidebar.
func (s *OCScreen) Title() string { return "GPU Overclock" }

// Init starts the device polling ticker.
func (s *OCScreen) Init() tea.Cmd {
	return s.tickDevice()
}

func (s *OCScreen) tickDevice() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ocDeviceTickMsg{}
	})
}

func (s *OCScreen) spinTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return ocSpinTickMsg{}
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
		s.applyProgress(commands.ProgressMsg(msg))
		return s, nil

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
			s.state = ocDone
			return s, s.handleRebootYes()
		case "n":
			s.handleRebootNo()
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

func (s *OCScreen) handlePipelineDone(msg ocPipelineDoneMsg) (Screen, tea.Cmd) {
	canceled, errored := s.finalizePipeline(msg.err)
	switch {
	case canceled:
		s.state = ocIdle
	case errored:
		s.state = ocError
	default:
		if s.state != ocRebootPrompt {
			s.state = ocRebootPrompt
			s.log.Add(ui.SuccessStyle.Render("Operation complete!"))
		}
	}
	return s, nil
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
	s.resetForRun()

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

// View renders the overclock screen content within the given dimensions.
func (s *OCScreen) View(width, height int) string {
	var b strings.Builder
	w := clampContentWidth(width)

	// 1. Header (logo + device info)
	header, headerLines := s.renderHeader(w)
	b.WriteString(header)

	// 2. Preset selector (tree-style)
	selector := s.renderPresetSelector()
	b.WriteString(selector)
	b.WriteString("\n\n")

	// Track button row for mouse click detection
	s.buttonRow = headerLines + strings.Count(selector, "\n") + 2

	// 3. Action button (centered)
	b.WriteString(center(s.renderButton()))
	b.WriteString("\n")

	// 4. Error popup if in error state
	if s.state == ocError {
		b.WriteString("\n")
		b.WriteString(s.renderErrorPopup())
		b.WriteString("\n")
	}

	// 5. Separator
	b.WriteString("\n" + ui.SeparatorStyle.Render(strings.Repeat("─", w)) + "\n")

	// 6. Progress log + live status line + progress bar
	logContent := s.log.Render()
	if logContent != "" {
		b.WriteString("\n")
		b.WriteString(logContent)
	}
	if s.state == ocRunning {
		b.WriteString(s.renderStatusLine(w))
	}
	b.WriteString("\n")

	return b.String()
}

// renderPresetSelector renders the tree-style preset list.
func (s *OCScreen) renderPresetSelector() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Frequencies"))
	b.WriteString("\n")
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

// Compile-time checks.
var _ Screen = (*OCScreen)(nil)
var _ ProgramSetter = (*OCScreen)(nil)

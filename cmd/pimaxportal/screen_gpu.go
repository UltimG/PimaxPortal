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

// GPU screen states
type gpuState int

const (
	gpuIdle         gpuState = iota
	gpuRunning
	gpuRootWait
	gpuRebootPrompt
	gpuError
	gpuDone
)

// GPU-specific message types to avoid conflicts with other screens.
type gpuProgressMsg commands.ProgressMsg
type gpuPipelineDoneMsg struct{ err error }
type gpuDeviceTickMsg struct{}
type gpuSpinTickMsg struct{}

// GPUScreen implements the Screen interface for the GPU driver
// build-and-install pipeline.
type GPUScreen struct {
	state      gpuState
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
}

// NewGPUScreen creates a new GPUScreen with an initial device poll.
func NewGPUScreen() *GPUScreen {
	info, _ := adb.GetDeviceInfo()
	return &GPUScreen{
		state:      gpuIdle,
		device:     info,
		spinnerDir: 1,
		statusPct:  -1,
	}
}

// SetProgram provides the tea.Program reference needed for goroutine
// communication (sending messages from pipeline goroutines).
func (s *GPUScreen) SetProgram(p *tea.Program) {
	s.program = p
}

// Key returns the unique identifier for this screen.
func (s *GPUScreen) Key() string {
	return "gpu"
}

// Title returns the display title shown in the sidebar.
func (s *GPUScreen) Title() string {
	return "GPU Drivers"
}

// Init starts the device polling ticker.
func (s *GPUScreen) Init() tea.Cmd {
	return s.tickDevice()
}

func (s *GPUScreen) tickDevice() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return gpuDeviceTickMsg{}
	})
}

// Update handles messages when this screen is active.
func (s *GPUScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			onButton := msg.Y >= s.buttonRow && msg.Y <= s.buttonRow+1
			switch s.state {
			case gpuIdle:
				if onButton {
					return s, s.startPipeline()
				}
			case gpuRebootPrompt:
				// Left half of button = yes, right half = no
				if onButton {
					mid := contentWidth / 2
					if msg.X < mid+ui.SidebarWidth {
						return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
					}
					return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
				}
			case gpuError:
				return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			case gpuRunning, gpuRootWait:
				// Click anywhere to cancel (like pressing q)
			}
		}
		return s, nil

	case tea.KeyMsg:
		return s.handleKey(msg)

	case gpuDeviceTickMsg:
		if s.state == gpuIdle {
			info, _ := adb.GetDeviceInfo()
			s.device = info
		}
		return s, s.tickDevice()

	case gpuProgressMsg:
		return s.handleProgress(commands.ProgressMsg(msg))

	case gpuPipelineDoneMsg:
		return s.handlePipelineDone(msg)

	case gpuSpinTickMsg:
		if s.state == gpuRunning || s.state == gpuRootWait {
			s.advanceSpinner()
			return s, s.spinTick()
		}
		return s, nil
	}

	return s, nil
}

func (s *GPUScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.state {
	case gpuIdle:
		switch key {
		case "enter":
			return s, s.startPipeline()
		case "r":
			info, _ := adb.GetDeviceInfo()
			s.device = info
			return s, nil
		}

	case gpuRunning:
		switch key {
		case "q", "ctrl+c":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		}

	case gpuRootWait:
		switch key {
		case "q", "ctrl+c", "c":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		}

	case gpuRebootPrompt:
		switch key {
		case "y":
			s.log.Add(ui.ProgressStyle.Render("Rebooting device..."))
			s.state = gpuDone
			_ = adb.Reboot()
			s.log.Add(ui.SuccessStyle.Render("Reboot command sent. Device will restart."))
			return s, tea.Quit
		case "n":
			s.log.Add(ui.SuccessStyle.Render("Skipped reboot. Reboot manually to apply changes."))
			s.state = gpuDone
			return s, tea.Quit
		}

	case gpuError:
		// Any key dismisses the error popup
		s.state = gpuIdle
		s.errorMsg = ""
		return s, nil

	case gpuDone:
		return s, tea.Quit
	}

	return s, nil
}

func (s *GPUScreen) handleProgress(msg commands.ProgressMsg) (Screen, tea.Cmd) {
	switch msg.Text {
	case "ROOT_CHECK_WAITING":
		s.state = gpuRootWait
		s.spinnerIdx = 0
		return s, s.spinTick()

	case "ROOT_CHECK_GRANTED":
		s.state = gpuRunning
		s.log.Add(ui.SuccessStyle.Render("Root access granted."))
		return s, nil

	case "ROOT_CHECK_TIMEOUT":
		s.showError("Root access timed out.\nPlease try again.")
		return s, nil

	case "INSTALL_COMPLETE":
		s.state = gpuRebootPrompt
		s.log.Add(ui.SuccessStyle.Render("Module installed successfully!"))
		return s, nil

	default:
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
}

func (s *GPUScreen) handlePipelineDone(msg gpuPipelineDoneMsg) (Screen, tea.Cmd) {
	s.statusLine = ""
	s.statusPct = -1
	if msg.err != nil {
		// Suppress context canceled anywhere in the error chain (user-initiated cancel)
		if errors.Is(msg.err, context.Canceled) {
			s.state = gpuIdle
			return s, nil
		}
		s.errorMsg = shortenError(msg.err)
		s.state = gpuError
		return s, nil
	}
	if s.state != gpuRebootPrompt {
		s.state = gpuIdle
	}
	return s, nil
}

func (s *GPUScreen) showError(msg string) {
	s.errorMsg = msg
	s.state = gpuError
	s.statusLine = ""
}

// startPipeline launches the build+install pipeline in a goroutine.
func (s *GPUScreen) startPipeline() tea.Cmd {
	s.state = gpuRunning
	s.log.Clear()
	s.statusLine = ""
	s.statusPct = -1
	s.spinnerIdx = 0
	s.spinnerDir = 1

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	p := s.program
	pipelineCmd := func() tea.Msg {
		go func() {
			send := func(msg commands.ProgressMsg) {
				if p != nil {
					p.Send(gpuProgressMsg(msg))
				}
			}

			// Run Build
			buildCmd := &commands.BuildCommand{}
			err := buildCmd.Run(ctx, send)
			if err != nil {
				if p != nil {
					p.Send(gpuPipelineDoneMsg{err: err})
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
						p.Send(gpuPipelineDoneMsg{err: err})
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
				p.Send(gpuPipelineDoneMsg{err: nil})
			}
		}()
		return nil
	}
	return tea.Batch(pipelineCmd, s.spinTick())
}

// Spinner

func (s *GPUScreen) advanceSpinner() {
	s.spinnerIdx += s.spinnerDir
	if s.spinnerIdx >= len(spinnerFrames)-1 {
		s.spinnerDir = -1
	} else if s.spinnerIdx <= 0 {
		s.spinnerDir = 1
	}
}

func (s *GPUScreen) spinnerFrame() string {
	idx := s.spinnerIdx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(spinnerFrames) {
		idx = len(spinnerFrames) - 1
	}
	return spinnerFrames[idx]
}

func (s *GPUScreen) spinTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return gpuSpinTickMsg{}
	})
}

// View renders the GPU screen content within the given dimensions.
func (s *GPUScreen) View(width, height int) string {
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
	deviceInfo := ui.RenderDeviceInfo(s.device, w)
	b.WriteString(deviceInfo)
	b.WriteString("\n\n")

	// Track button row for mouse click detection
	linesSoFar := 2 + strings.Count(logo, "\n") + 1 + strings.Count(deviceInfo, "\n") + 1
	s.buttonRow = linesSoFar

	// 3. Action button (centered)
	b.WriteString(center(s.renderButton()))
	b.WriteString("\n")

	// 4. Overlays
	if s.state == gpuRootWait {
		b.WriteString("\n")
		b.WriteString(s.renderRootWaitOverlay())
		b.WriteString("\n")
	}
	if s.state == gpuError {
		b.WriteString("\n")
		b.WriteString(s.renderErrorPopup())
		b.WriteString("\n")
	}

	// 5. Separator
	b.WriteString("\n" + ui.SeparatorStyle.Render(strings.Repeat("─", w)) + "\n")

	// 6. Progress log + live status
	logContent := s.log.Render()
	if logContent != "" {
		b.WriteString("\n")
		b.WriteString(logContent)
	}
	if s.statusLine != "" && (s.state == gpuRunning || s.state == gpuRootWait) {
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

// FooterHelp returns state-appropriate key hints.
func (s *GPUScreen) FooterHelp() string {
	switch s.state {
	case gpuIdle:
		return "enter start  r refresh  q quit"
	case gpuRunning:
		return "q cancel"
	case gpuRootWait:
		return "c cancel"
	case gpuRebootPrompt:
		return "y reboot  n skip"
	case gpuError:
		return "any key dismiss"
	default:
		return ""
	}
}

// Rendering helpers

func (s *GPUScreen) renderButton() string {
	switch s.state {
	case gpuIdle:
		if s.device.Connected {
			return ui.ButtonStyle.Render("  Build & Install GPU Drivers  ")
		}
		return ui.ButtonStyle.Render("  Build GPU Driver Module  ")
	case gpuRunning:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(s.spinnerFrame()) + " Running  ")
	case gpuRootWait:
		return ui.ButtonDimStyle.Render("  Waiting for Root...  ")
	case gpuRebootPrompt:
		return ui.ButtonStyle.Render("  Reboot Device?  y/n  ")
	case gpuError:
		return ui.ButtonDimStyle.Render("  Error  ")
	case gpuDone:
		return ui.SuccessStyle.Render("  Done!  ")
	}
	return ""
}

func (s *GPUScreen) renderErrorPopup() string {
	wrapped := wordWrap(s.errorMsg, 36)
	body := lipgloss.PlaceHorizontal(36, lipgloss.Center, wrapped)
	popup := ui.BoxStyle.Render("\n" + body + "\n")
	return lipgloss.PlaceHorizontal(innerWidth(), lipgloss.Center, popup)
}

func (s *GPUScreen) renderRootWaitOverlay() string {
	frame := s.spinnerFrame()

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

// Compile-time check that GPUScreen implements Screen.
var _ Screen = (*GPUScreen)(nil)

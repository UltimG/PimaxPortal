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

// Root screen states
type rootState int

const (
	rootIdle      rootState = iota
	rootRunning             // Phase 1: download Magisk, install, push boot.img
	rootPatchWait           // Waiting for user to patch boot.img in Magisk
	rootFlashing            // Phase 2: pull patched image, flash, reboot
	rootError
	rootDone
)

// Root-specific message types to avoid conflicts with other screens.
type rootProgressMsg commands.ProgressMsg
type rootPipelineDoneMsg struct{ err error }
type rootDeviceTickMsg struct{}
type rootSpinTickMsg struct{}
type rootPollPatchMsg struct{}

// RootScreen implements the Screen interface for the automated rooting flow.
type RootScreen struct {
	state      rootState
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

// NewRootScreen creates a new RootScreen with an initial device poll.
func NewRootScreen() *RootScreen {
	info, _ := adb.GetDeviceInfo()
	return &RootScreen{
		state:      rootIdle,
		device:     info,
		spinnerDir: 1,
		statusPct:  -1,
	}
}

// SetProgram provides the tea.Program reference needed for goroutine
// communication (sending messages from pipeline goroutines).
func (s *RootScreen) SetProgram(p *tea.Program) {
	s.program = p
}

// Key returns the unique identifier for this screen.
func (s *RootScreen) Key() string {
	return "root"
}

// Title returns the display title shown in the sidebar.
func (s *RootScreen) Title() string {
	return "Root Device"
}

// Init starts the device polling ticker.
func (s *RootScreen) Init() tea.Cmd {
	return s.tickDevice()
}

func (s *RootScreen) tickDevice() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return rootDeviceTickMsg{}
	})
}

func (s *RootScreen) pollPatchTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return rootPollPatchMsg{}
	})
}

// Update handles messages when this screen is active.
func (s *RootScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			onButton := msg.Y >= s.buttonRow && msg.Y <= s.buttonRow+1
			switch s.state {
			case rootIdle:
				if onButton && s.device.Connected {
					return s, s.startPreparePipeline()
				}
			case rootError:
				return s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			}
		}
		return s, nil

	case tea.KeyMsg:
		return s.handleKey(msg)

	case rootDeviceTickMsg:
		if s.state == rootIdle {
			info, _ := adb.GetDeviceInfo()
			s.device = info
		}
		return s, s.tickDevice()

	case rootProgressMsg:
		return s.handleProgress(commands.ProgressMsg(msg))

	case rootPipelineDoneMsg:
		return s.handlePipelineDone(msg)

	case rootSpinTickMsg:
		if s.state == rootRunning || s.state == rootPatchWait || s.state == rootFlashing {
			s.advanceSpinner()
			return s, s.spinTick()
		}
		return s, nil

	case rootPollPatchMsg:
		if s.state == rootPatchWait {
			// Check for patched image on device
			_, err := adb.FindFile("/sdcard/Download/magisk_patched-*")
			if err == nil {
				// Patched image found — transition to flashing
				s.state = rootFlashing
				s.log.Add(ui.SuccessStyle.Render("Patched boot image detected!"))
				return s, s.startFlashPipeline()
			}
			// Not found yet — keep polling
			return s, s.pollPatchTick()
		}
		return s, nil
	}

	return s, nil
}

func (s *RootScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.state {
	case rootIdle:
		switch key {
		case "enter":
			if s.device.Connected {
				return s, s.startPreparePipeline()
			}
			return s, nil
		case "r":
			info, _ := adb.GetDeviceInfo()
			s.device = info
			return s, nil
		}

	case rootRunning, rootFlashing:
		switch key {
		case "q", "ctrl+c":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		}

	case rootPatchWait:
		switch key {
		case "c", "q", "ctrl+c":
			if s.cancel != nil {
				s.cancel()
			}
			s.state = rootIdle
			s.statusLine = ""
			return s, nil
		}

	case rootError:
		// Any key dismisses the error popup
		s.state = rootIdle
		s.errorMsg = ""
		return s, nil

	case rootDone:
		// Nothing
	}

	return s, nil
}

func (s *RootScreen) handleProgress(msg commands.ProgressMsg) (Screen, tea.Cmd) {
	if msg.Percent == 1.0 {
		// Step completed — move to log
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

func (s *RootScreen) handlePipelineDone(msg rootPipelineDoneMsg) (Screen, tea.Cmd) {
	s.statusLine = ""
	s.statusPct = -1
	if msg.err != nil {
		// Suppress context canceled (user-initiated cancel)
		if errors.Is(msg.err, context.Canceled) {
			s.state = rootIdle
			return s, nil
		}
		s.errorMsg = shortenError(msg.err)
		s.state = rootError
		return s, nil
	}

	switch s.state {
	case rootRunning:
		// Phase 1 done — transition to patch wait
		s.state = rootPatchWait
		s.log.Add(ui.SuccessStyle.Render("Ready for Magisk patching"))
		return s, s.pollPatchTick()
	case rootFlashing:
		// Phase 2 done — rooted!
		s.state = rootDone
		s.log.Add(ui.SuccessStyle.Render("Device rooted successfully!"))
		return s, nil
	}

	return s, nil
}

func (s *RootScreen) showError(msg string) {
	s.errorMsg = msg
	s.state = rootError
	s.statusLine = ""
}

// startPreparePipeline launches Phase 1: check root, prepare Magisk, push boot.img.
func (s *RootScreen) startPreparePipeline() tea.Cmd {
	s.state = rootRunning
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
					p.Send(rootProgressMsg(msg))
				}
			}

			// Check if already rooted
			rooted, err := adb.CheckRoot()
			if err == nil && rooted {
				send(commands.ProgressMsg{Text: "Device is already rooted", Percent: 1.0})
				if p != nil {
					p.Send(rootPipelineDoneMsg{})
				}
				return
			}

			cacheDir, err := commands.CacheDir()
			if err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}

			rootCmd := commands.RootCommand{}

			// Verify firmware matches our boot.img before proceeding
			send(commands.ProgressMsg{Text: "Checking firmware version", Percent: -1})
			if err := rootCmd.CheckFirmware(); err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}
			send(commands.ProgressMsg{Text: "Firmware version verified", Percent: 1.0})

			if err := rootCmd.PrepareMagisk(ctx, cacheDir, send); err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}

			if err := rootCmd.PushBootImage(ctx, cacheDir, send); err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}

			if p != nil {
				p.Send(rootPipelineDoneMsg{})
			}
		}()
		return nil
	}
	return tea.Batch(pipelineCmd, s.spinTick())
}

// startFlashPipeline launches Phase 2: pull patched image, flash, reboot.
func (s *RootScreen) startFlashPipeline() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	p := s.program
	pipelineCmd := func() tea.Msg {
		go func() {
			send := func(msg commands.ProgressMsg) {
				if p != nil {
					p.Send(rootProgressMsg(msg))
				}
			}

			cacheDir, err := commands.CacheDir()
			if err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}

			rootCmd := commands.RootCommand{}

			send(commands.ProgressMsg{Text: "Pulling patched boot image", Percent: 0})
			imagePath, err := rootCmd.PullPatchedImage(cacheDir)
			if err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}
			send(commands.ProgressMsg{Text: "Patched image pulled", Percent: 1.0})

			flashCmd := commands.FlashCommand{}
			if err := flashCmd.Flash(ctx, imagePath, send); err != nil {
				if p != nil {
					p.Send(rootPipelineDoneMsg{err: err})
				}
				return
			}

			if p != nil {
				p.Send(rootPipelineDoneMsg{})
			}
		}()
		return nil
	}
	return tea.Batch(pipelineCmd, s.spinTick())
}

// Spinner

func (s *RootScreen) advanceSpinner() {
	s.spinnerIdx += s.spinnerDir
	if s.spinnerIdx >= len(spinnerFrames)-1 {
		s.spinnerDir = -1
	} else if s.spinnerIdx <= 0 {
		s.spinnerDir = 1
	}
}

func (s *RootScreen) spinnerFrame() string {
	idx := s.spinnerIdx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(spinnerFrames) {
		idx = len(spinnerFrames) - 1
	}
	return spinnerFrames[idx]
}

func (s *RootScreen) spinTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return rootSpinTickMsg{}
	})
}

// View renders the root screen content within the given dimensions.
func (s *RootScreen) View(width, height int) string {
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
	if s.state == rootPatchWait {
		b.WriteString("\n")
		b.WriteString(s.renderPatchWaitOverlay())
		b.WriteString("\n")
	}
	if s.state == rootError {
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
	if s.statusLine != "" && (s.state == rootRunning || s.state == rootPatchWait || s.state == rootFlashing) {
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
func (s *RootScreen) FooterHelp() string {
	switch s.state {
	case rootIdle:
		return "enter start  r refresh"
	case rootRunning, rootFlashing:
		return "q cancel"
	case rootPatchWait:
		return "c cancel"
	case rootError:
		return "any key dismiss"
	default:
		return ""
	}
}

// Rendering helpers

func (s *RootScreen) renderButton() string {
	switch s.state {
	case rootIdle:
		if s.device.Connected {
			return ui.ButtonStyle.Render("  Root Device  ")
		}
		return ui.ButtonDimStyle.Render("  Connect Device to Start  ")
	case rootRunning:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(s.spinnerFrame()) + " Preparing  ")
	case rootPatchWait:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(s.spinnerFrame()) + " Waiting for Patch  ")
	case rootFlashing:
		return ui.ButtonDimStyle.Render("  " + ui.SpinnerStyle.Render(s.spinnerFrame()) + " Flashing  ")
	case rootError:
		return ui.ButtonDimStyle.Render("  Error  ")
	case rootDone:
		return ui.SuccessStyle.Render("  Rooted!  ")
	}
	return ""
}

func (s *RootScreen) renderErrorPopup() string {
	wrapped := wordWrap(s.errorMsg, 36)
	body := lipgloss.PlaceHorizontal(36, lipgloss.Center, wrapped)
	popup := ui.BoxStyle.Render("\n" + body + "\n")
	return lipgloss.PlaceHorizontal(innerWidth(), lipgloss.Center, popup)
}

func (s *RootScreen) renderPatchWaitOverlay() string {
	frame := s.spinnerFrame()

	content := "\n" +
		"  1. Open the Magisk app on the device\n" +
		"  2. Tap \"Install\" next to Magisk\n" +
		"  3. Select \"Select and Patch a File\"\n" +
		"  4. Choose boot.img from /sdcard/\n" +
		"  5. Wait for \"All done!\"\n" +
		"\n" +
		"  Polling for patched image...  " + ui.SpinnerStyle.Render(frame)

	title := " " + ui.SpinnerStyle.Render(frame) + "  " + ui.TitleStyle.Render("Patch boot.img in Magisk")
	return ui.BoxStyle.Render(title + content)
}

// Compile-time check that RootScreen implements Screen.
var _ Screen = (*RootScreen)(nil)

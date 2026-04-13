package main

import (
	"context"
	"errors"
	"strings"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// baseScreen holds the fields and methods shared by every pipeline-style
// Screen implementation (GPU, Overclock, ...). Screens embed it and only
// implement the state-machine parts that differ.
type baseScreen struct {
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

// SetProgram wires the tea.Program reference so pipeline goroutines can
// push messages back into the update loop.
func (b *baseScreen) SetProgram(p *tea.Program) {
	b.program = p
}

// advanceSpinner bounces the spinner frame index back and forth.
func (b *baseScreen) advanceSpinner() {
	b.spinnerIdx += b.spinnerDir
	if b.spinnerIdx >= len(spinnerFrames)-1 {
		b.spinnerDir = -1
	} else if b.spinnerIdx <= 0 {
		b.spinnerDir = 1
	}
}

// spinnerFrame returns the current spinner glyph, clamped to valid range.
func (b *baseScreen) spinnerFrame() string {
	idx := b.spinnerIdx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(spinnerFrames) {
		idx = len(spinnerFrames) - 1
	}
	return spinnerFrames[idx]
}

// resetForRun clears transient run state before launching a pipeline.
func (b *baseScreen) resetForRun() {
	b.log.Clear()
	b.statusLine = ""
	b.statusPct = -1
	b.spinnerIdx = 0
	b.spinnerDir = 1
}

// applyProgress routes a ProgressMsg into either the log (on completion)
// or the live status line. Returns true if the message was a completion
// step (Percent == 1.0) so the caller can perform additional actions.
func (b *baseScreen) applyProgress(msg commands.ProgressMsg) bool {
	if msg.Percent == 1.0 {
		entry := ui.SuccessStyle.Render("■") + " " + ui.ProgressStyle.Render(msg.Text)
		if len(b.log.Lines) == 0 || b.log.Lines[len(b.log.Lines)-1] != entry {
			b.log.Add(entry)
		}
		b.statusLine = ""
		b.statusPct = -1
		return true
	}
	b.statusLine = msg.Text
	b.statusPct = msg.Percent
	return false
}

// finalizePipeline handles the common cleanup for a finished pipeline.
// Returns canceled=true if the error is a user-initiated cancel (caller
// should return to idle), errored=true if it is a real failure (caller
// should transition to its error state — errorMsg is already populated).
// If both are false, the pipeline succeeded and the caller owns the
// success transition.
func (b *baseScreen) finalizePipeline(err error) (canceled, errored bool) {
	b.statusLine = ""
	b.statusPct = -1
	if err == nil {
		return false, false
	}
	if errors.Is(err, context.Canceled) {
		return true, false
	}
	b.errorMsg = shortenError(err)
	return false, true
}

// renderHeader renders the logo + device info block shared by all screens
// and returns the content plus the number of lines it occupies (used for
// mouse-click button-row tracking).
func (b *baseScreen) renderHeader(w int) (string, int) {
	var sb strings.Builder
	art := center(ui.TitleStyle.Render(ui.LogoArt))
	subtitle := center(ui.TitleStyle.Render(ui.LogoSubtitle))
	sb.WriteString(art + "\n" + subtitle)
	sb.WriteString("\n\n")
	sb.WriteString(ui.RenderDeviceInfo(b.device, w))
	sb.WriteString("\n\n")
	out := sb.String()
	return out, strings.Count(out, "\n")
}

// renderErrorPopup renders the centered error popup using b.errorMsg.
func (b *baseScreen) renderErrorPopup() string {
	wrapped := wordWrap(b.errorMsg, 36)
	body := lipgloss.PlaceHorizontal(36, lipgloss.Center, wrapped)
	popup := ui.BoxStyle.Render("\n" + body + "\n")
	return lipgloss.PlaceHorizontal(innerWidth(), lipgloss.Center, popup)
}

// renderStatusLine returns the live status line and progress bar (if any),
// or an empty string when there is no active status line.
func (b *baseScreen) renderStatusLine(w int) string {
	if b.statusLine == "" {
		return ""
	}
	out := "\n" + ui.ProgressStyle.Render(b.statusLine)
	if b.statusPct >= 0 && b.statusPct < 1.0 {
		out += "\n" + renderProgressBar(b.statusPct, w)
	}
	return out
}

// handleRebootYes executes the common "user confirmed reboot" flow. The
// caller is responsible for transitioning to its own Done state before
// calling this.
func (b *baseScreen) handleRebootYes() tea.Cmd {
	b.log.Add(ui.ProgressStyle.Render("Rebooting device..."))
	_ = adb.Reboot()
	b.log.Add(ui.SuccessStyle.Render("Reboot command sent. Device will restart."))
	return tea.Quit
}

// handleRebootNo logs that the user skipped the reboot prompt. The caller
// is responsible for the state transition and whatever tea.Cmd to return.
func (b *baseScreen) handleRebootNo() {
	b.log.Add(ui.SuccessStyle.Render("Skipped reboot. Reboot manually to apply changes."))
}

// clampContentWidth returns the smaller of the provided width and
// contentWidth, falling back to contentWidth when width is zero.
func clampContentWidth(width int) int {
	if width > 0 && width < contentWidth {
		return width
	}
	return contentWidth
}

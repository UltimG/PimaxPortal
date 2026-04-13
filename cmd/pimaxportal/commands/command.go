package commands

// ProgressMsg reports pipeline step progress to the TUI.
type ProgressMsg struct {
	Text    string
	Percent float64 // 0.0-1.0 for progress, -1 for indeterminate
}

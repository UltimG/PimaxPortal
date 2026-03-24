package main

import (
	"fmt"
	"os"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := commands.CheckPreflight(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n\n", err)
		os.Exit(1)
	}

	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

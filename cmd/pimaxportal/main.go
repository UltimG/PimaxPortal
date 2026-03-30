package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Pass program reference to screens that need it for goroutine communication
	for _, s := range m.screens {
		switch screen := s.(type) {
		case *GPUScreen:
			screen.SetProgram(p)
		case *RootScreen:
			screen.SetProgram(p)
		}
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

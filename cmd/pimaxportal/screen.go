package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Screen represents a navigable content panel.
type Screen interface {
	// Key returns the unique identifier for this screen.
	Key() string
	// Title returns the display title.
	Title() string
	// Init is called when the screen becomes active.
	Init() tea.Cmd
	// Update handles messages when the screen is active.
	Update(msg tea.Msg) (Screen, tea.Cmd)
	// View renders the screen content within the given dimensions.
	View(width, height int) string
	// FooterHelp returns the footer key hints for the current state.
	FooterHelp() string
}

// ProgramSetter is implemented by screens that need a reference to the
// tea.Program in order to push messages from background goroutines.
// main() passes the program to every screen that implements it.
type ProgramSetter interface {
	SetProgram(*tea.Program)
}

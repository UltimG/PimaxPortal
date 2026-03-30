package main

import (
	"strings"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// SidebarEntry represents a single item in the sidebar tree menu.
type SidebarEntry struct {
	Key      string
	Label    string
	IsHeader bool
}

// ScreenSelectedMsg is sent when the user selects a screen in the sidebar.
type ScreenSelectedMsg struct {
	Key string
}

// Sidebar is a tree-style navigation menu component.
type Sidebar struct {
	entries []SidebarEntry
	cursor  int
	active  int
	focused bool
	height  int
}

// NewSidebar creates a new sidebar with the given entries.
// The cursor and active index are set to the first selectable entry.
func NewSidebar(entries []SidebarEntry) Sidebar {
	first := 0
	for i, e := range entries {
		if !e.IsHeader {
			first = i
			break
		}
	}
	return Sidebar{
		entries: entries,
		cursor:  first,
		active:  first,
	}
}

// SetHeight sets the available height for rendering.
func (s *Sidebar) SetHeight(h int) {
	s.height = h
}

// SetFocused sets whether the sidebar has keyboard focus.
func (s *Sidebar) SetFocused(f bool) {
	s.focused = f
}

// Focused returns whether the sidebar has keyboard focus.
func (s Sidebar) Focused() bool {
	return s.focused
}

// ActiveKey returns the key of the currently active entry.
func (s Sidebar) ActiveKey() string {
	if s.active >= 0 && s.active < len(s.entries) {
		return s.entries[s.active].Key
	}
	return ""
}

// SetActive sets the active entry by key.
func (s *Sidebar) SetActive(key string) {
	for i, e := range s.entries {
		if e.Key == key {
			s.active = i
			s.cursor = i
			return
		}
	}
}

// Update handles keyboard input when the sidebar is focused.
func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	if !s.focused {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			s.moveCursor(-1)
		case "down", "j":
			s.moveCursor(1)
		case "enter":
			if s.cursor >= 0 && s.cursor < len(s.entries) && !s.entries[s.cursor].IsHeader {
				s.active = s.cursor
				return s, func() tea.Msg {
					return ScreenSelectedMsg{Key: s.entries[s.active].Key}
				}
			}
		}
	}

	return s, nil
}

// moveCursor moves the cursor by dir (+1 or -1), skipping headers.
func (s *Sidebar) moveCursor(dir int) {
	n := len(s.entries)
	if n == 0 {
		return
	}

	next := s.cursor
	for {
		next += dir
		if next < 0 || next >= n {
			return // hit boundary, don't wrap
		}
		if !s.entries[next].IsHeader {
			s.cursor = next
			return
		}
	}
}

// isLastInGroup returns true if the entry at idx is the last selectable
// entry before the next header or end of entries.
func (s Sidebar) isLastInGroup(idx int) bool {
	for i := idx + 1; i < len(s.entries); i++ {
		if s.entries[i].IsHeader {
			return true
		}
		if !s.entries[i].IsHeader {
			return false
		}
	}
	return true
}

// EntryKeyAtY returns the key of the sidebar entry at the given Y coordinate,
// accounting for padding. Returns "" if no selectable entry at that position.
func (s Sidebar) EntryKeyAtY(y int) string {
	// SidebarStyle has Padding(1, 1) — 1 row top padding
	row := y - 1
	if row < 0 || row >= len(s.entries) {
		return ""
	}
	// Headers have an extra MarginTop(1) line except the first
	idx := 0
	for i, e := range s.entries {
		if e.IsHeader && i > 0 {
			row-- // skip the margin line
		}
		if row == 0 {
			if e.IsHeader {
				return ""
			}
			return e.Key
		}
		row--
		idx = i
		_ = idx
	}
	return ""
}

// View renders the sidebar tree menu.
func (s Sidebar) View() string {
	var b strings.Builder
	firstHeader := true

	for i, entry := range s.entries {
		if entry.IsHeader {
			style := ui.SidebarHeaderStyle
			if firstHeader {
				style = style.MarginTop(0)
				firstHeader = false
			}
			b.WriteString(style.Render(entry.Label))
			b.WriteString("\n")
			continue
		}

		// Tree connector
		var connector string
		if s.isLastInGroup(i) {
			connector = ui.TreeConnectorStyle.Render("└")
		} else {
			connector = ui.TreeConnectorStyle.Render("│")
		}

		// Active indicator
		var indicator string
		if i == s.active {
			indicator = ui.SidebarActiveStyle.Render("■")
		} else {
			indicator = ui.TreeConnectorStyle.Render("□")
		}

		// Label styling
		label := entry.Label
		if i == s.active {
			label = ui.SidebarActiveStyle.Render(label)
		} else if s.focused && i == s.cursor {
			label = ui.SidebarCursorStyle.Render(label)
		} else {
			label = ui.SidebarItemStyle.Render(label)
		}

		b.WriteString(connector + " " + indicator + " " + label)
		b.WriteString("\n")
	}

	return ui.SidebarStyle.Render(b.String())
}

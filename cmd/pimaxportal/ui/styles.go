package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7C3AED")
	ColorSecondary = lipgloss.Color("#06B6D4")
	ColorSuccess   = lipgloss.Color("#22C55E")
	ColorWarning   = lipgloss.Color("#EAB308")
	ColorError     = lipgloss.Color("#EF4444")
	ColorDim       = lipgloss.Color("#6B7280")

	TitleStyle         = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DeviceConnected    = lipgloss.NewStyle().Foreground(ColorSuccess)
	DeviceDisconnected = lipgloss.NewStyle().Foreground(ColorError)
	DeviceLabelStyle   = lipgloss.NewStyle().Foreground(ColorDim).Width(10)
	DeviceValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	ButtonStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(ColorPrimary).Padding(0, 2).Bold(true)
	ButtonDimStyle     = lipgloss.NewStyle().Foreground(ColorDim).Background(lipgloss.Color("#374151")).Padding(0, 2)
	ProgressStyle      = lipgloss.NewStyle().Foreground(ColorSecondary)
	ErrorStyle         = lipgloss.NewStyle().Foreground(ColorError)
	SuccessStyle       = lipgloss.NewStyle().Foreground(ColorSuccess)
	BoxStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
)

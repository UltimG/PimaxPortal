package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorSuccess = lipgloss.Color("#22C55E")
	ColorOrange  = lipgloss.Color("#F97316")
	ColorDim     = lipgloss.Color("#6B7280")
	ColorBorder  = lipgloss.Color("#555555")

	SpinnerStyle = lipgloss.NewStyle().Foreground(ColorOrange)

	TitleStyle         = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	DeviceConnected    = lipgloss.NewStyle().Foreground(ColorSuccess)
	DeviceDisconnected = lipgloss.NewStyle().Foreground(ColorDim)
	DeviceLabelStyle   = lipgloss.NewStyle().Foreground(ColorDim).Width(10)
	DeviceValueStyle   = lipgloss.NewStyle().Foreground(ColorWhite)
	ButtonStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(ColorWhite).Padding(0, 2).Bold(true)
	ButtonDimStyle     = lipgloss.NewStyle().Foreground(ColorDim).Padding(0, 2)
	ProgressStyle      = lipgloss.NewStyle().Foreground(ColorWhite)
	ErrorStyle         = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	SuccessStyle       = lipgloss.NewStyle().Foreground(ColorSuccess)
	BoxStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	FrameStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 3)
	SeparatorStyle     = lipgloss.NewStyle().Foreground(ColorBorder)
	FooterStyle        = lipgloss.NewStyle().Foreground(ColorDim)

	// Sidebar styles
	SidebarWidth = 22

	SidebarStyle       = lipgloss.NewStyle().Width(SidebarWidth).Padding(1, 1)
	SidebarActiveStyle = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	SidebarItemStyle   = lipgloss.NewStyle().Foreground(ColorDim)
	SidebarCursorStyle = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	SidebarHeaderStyle = lipgloss.NewStyle().Foreground(ColorWhite).Bold(true).MarginTop(1)
	TreeConnectorStyle = lipgloss.NewStyle().Foreground(ColorDim)
)

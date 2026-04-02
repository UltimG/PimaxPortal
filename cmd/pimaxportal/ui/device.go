package ui

import (
	"fmt"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/charmbracelet/lipgloss"
)

var (
	rowEvenBg = lipgloss.Color("#1a1a1a")
	rowOddBg  = lipgloss.Color("#252525")
)

func RenderDeviceInfo(info adb.DeviceInfo, width int) string {
	if !info.Connected {
		status := " " + DeviceDisconnected.Render("● Disconnected")
		msg := "No device connected"
		if info.MultipleDevices {
			msg = "Multiple devices connected — disconnect all but one"
		}
		return lipgloss.JoinVertical(lipgloss.Left, status, DeviceLabelStyle.Render(msg))
	}

	status := " " + DeviceConnected.Render("● Connected")

	rows := []struct{ label, value string }{
		{"Device", fmt.Sprintf("%s (%s)", info.Model, info.Variant)},
		{"GPU", fmt.Sprintf("%s %s", info.GPU, info.DriverVersion)},
		{"Panel", panelDescription(info.PanelType)},
	}

	var rendered []string
	rendered = append(rendered, status)

	for i, row := range rows {
		bg := rowEvenBg
		if i%2 == 1 {
			bg = rowOddBg
		}
		label := lipgloss.NewStyle().Foreground(ColorDim).Background(bg).Width(10).PaddingLeft(1).PaddingRight(1).Render(row.label)
		value := lipgloss.NewStyle().Foreground(ColorWhite).Background(bg).Width(width - 10).PaddingLeft(1).PaddingRight(1).Render(row.value)
		rendered = append(rendered, label+value)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func panelDescription(panelType string) string {
	switch panelType {
	case "3":
		return "2K QHD (type 3)"
	case "2":
		return "1080p (type 2)"
	case "1":
		return "1080p (type 1)"
	case "":
		return "Unknown"
	default:
		return fmt.Sprintf("Type %s", panelType)
	}
}

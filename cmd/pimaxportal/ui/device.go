package ui

import (
	"fmt"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
	"github.com/charmbracelet/lipgloss"
)

func RenderDeviceInfo(info adb.DeviceInfo) string {
	if !info.Connected {
		status := DeviceDisconnected.Render("● Disconnected")
		msg := "No device connected"
		if info.MultipleDevices {
			msg = "Multiple devices connected — disconnect all but one"
		}
		return lipgloss.JoinVertical(lipgloss.Left, status, DeviceLabelStyle.Render(msg))
	}

	status := DeviceConnected.Render("● Connected")
	model := fmt.Sprintf("%s (%s)", info.Model, info.Variant)
	lines := []string{
		status,
		DeviceLabelStyle.Render("Device:") + "  " + DeviceValueStyle.Render(model),
		DeviceLabelStyle.Render("GPU:") + "     " + DeviceValueStyle.Render(fmt.Sprintf("%s %s", info.GPU, info.DriverVersion)),
		DeviceLabelStyle.Render("Panel:") + "   " + DeviceValueStyle.Render(fmt.Sprintf("Type %s", info.PanelType)),
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const maxLogLines = 15

type ProgressLog struct {
	Lines []string
}

func (p *ProgressLog) Add(line string) {
	p.Lines = append(p.Lines, line)
	if len(p.Lines) > maxLogLines {
		p.Lines = p.Lines[len(p.Lines)-maxLogLines:]
	}
}

func (p *ProgressLog) Clear() {
	p.Lines = nil
}

func (p *ProgressLog) Render() string {
	if len(p.Lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range p.Lines {
		connector := "├"
		if i == len(p.Lines)-1 {
			connector = "└"
		}
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
		b.WriteString(dim.Render(connector) + " " + line)
		if i < len(p.Lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

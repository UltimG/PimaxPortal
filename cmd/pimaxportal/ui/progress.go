package ui

import "strings"

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
	return strings.Join(p.Lines, "\n")
}

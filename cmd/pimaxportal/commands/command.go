package commands

import "context"

type Command interface {
	Name() string
	Description() string
	Run(ctx context.Context, send func(ProgressMsg)) error
}

type ProgressMsg struct {
	Text    string
	Percent float64 // 0.0-1.0 for progress, -1 for indeterminate
}

var registry []Command

func Register(cmd Command) {
	registry = append(registry, cmd)
}

func All() []Command {
	return registry
}

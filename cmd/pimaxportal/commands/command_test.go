package commands

import (
	"context"
	"testing"
)

type mockCommand struct{}

func (m *mockCommand) Name() string        { return "mock" }
func (m *mockCommand) Description() string { return "A mock command" }
func (m *mockCommand) Run(ctx context.Context, send func(ProgressMsg)) error {
	send(ProgressMsg{Text: "running"})
	return nil
}

func TestRegisterAndGet(t *testing.T) {
	registry = nil
	Register(&mockCommand{})
	cmds := All()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name() != "mock" {
		t.Fatalf("expected name 'mock', got '%s'", cmds[0].Name())
	}
}

func TestProgressMsg(t *testing.T) {
	var received []ProgressMsg
	send := func(msg ProgressMsg) { received = append(received, msg) }
	cmd := &mockCommand{}
	err := cmd.Run(context.Background(), send)
	if err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Text != "running" {
		t.Fatalf("unexpected messages: %v", received)
	}
}

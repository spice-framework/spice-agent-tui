package composition_test

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	agenttui "github.com/spice-framework/spice-agent-tui"
	spicegen "github.com/spice-framework/spice-agent-tui/internal/spicegen/compositionproof"
	"github.com/spice-framework/spice/bean"
)

func TestGeneratedApplicationConstructsCompletePublicTerminalGraph(t *testing.T) {
	var output bytes.Buffer
	streams, err := agenttui.NewTerminalIO(bytes.NewBufferString("\x03"), &output)
	if err != nil {
		t.Fatal(err)
	}
	application, err := spicegen.NewApplicationWithOptions(t.Context(), spicegen.ApplicationOptions{
		Overrides: spicegen.BeanOverrides{OsTerminalIO: bean.Replace(streams)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	components := application.Components()
	if components.TerminalShell == nil || components.FixedRenderer == nil ||
		components.DarkTheme == nil || components.AcceptanceSession == nil {
		t.Fatalf("generated components = %#v", components)
	}
	if components.DarkTheme.Name() != "spice-dark" ||
		components.ConnectingView.Status().Level() != agenttui.StatusReconnecting {
		t.Fatalf("generated presentation defaults = %#v", components)
	}
	wantOrder := []agenttui.Action{
		agenttui.ActionSubmit, agenttui.ActionCancelActiveRun, agenttui.ActionRespond, agenttui.ActionQuit,
		agenttui.ActionCursorLeft, agenttui.ActionCursorRight, agenttui.ActionCursorStart, agenttui.ActionCursorEnd,
		agenttui.ActionHistoryPrevious, agenttui.ActionHistoryNext, agenttui.ActionBackspace,
	}
	if order := components.BindingOrder.Actions(); !slices.Equal(order, wantOrder) {
		t.Fatalf("generated binding order = %v, want %v", order, wantOrder)
	}
	runContext, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := components.TerminalShell.Run(runContext); err != nil {
		t.Fatalf("generated terminal shell normal exit: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("generated terminal shell produced no lifecycle output")
	}
	if err := application.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}

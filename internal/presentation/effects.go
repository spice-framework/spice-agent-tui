package presentation

import (
	"context"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

// OperationToken identifies one presentation-issued asynchronous operation.
// It is local process state, not a daemon or protocol operation identifier.
type OperationToken uint64

// Effects is the adapter seam for asynchronous presentation work. Implementors
// may later delegate to an adopted high-level client, but the model only calls
// these methods from Bubble Tea commands.
type Effects interface {
	Receive(context.Context, OperationToken) (tea.Msg, error)
	Perform(context.Context, OperationToken, agenttui.Intent) (agenttui.CommandResult, error)
}

type receiveCompletedMsg struct {
	token   OperationToken
	message tea.Msg
	err     error
}

type effectCompletedMsg struct {
	token  OperationToken
	result agenttui.CommandResult
	err    error
}

func receiveCommand(ctx context.Context, effects Effects, token OperationToken) tea.Cmd {
	return func() tea.Msg {
		message, err := effects.Receive(ctx, token)
		return receiveCompletedMsg{token: token, message: message, err: err}
	}
}

func effectCommand(ctx context.Context, effects Effects, token OperationToken, intent agenttui.Intent) tea.Cmd {
	return func() tea.Msg {
		result, err := effects.Perform(ctx, token, intent)
		return effectCompletedMsg{token: token, result: result, err: err}
	}
}

func effectErrorStatus(err error) agenttui.StatusState {
	message, textErr := agenttui.NewText("operation failed; inspect application diagnostics")
	if textErr != nil || err == nil {
		return agenttui.StatusState{}
	}
	status, statusErr := agenttui.NewStatus(agenttui.StatusError, message, nil)
	if statusErr != nil {
		return agenttui.StatusState{}
	}
	return status
}

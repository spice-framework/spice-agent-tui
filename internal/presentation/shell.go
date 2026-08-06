package presentation

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

type bubbleShell struct {
	model  Model
	input  io.Reader
	output io.Writer
}

// NewShell constructs a Bubble Tea shell around an already validated model.
// Input may be nil for non-interactive execution; output must be injected.
func NewShell(model Model, input io.Reader, output io.Writer) (agenttui.Shell, error) {
	if model.renderer == nil {
		return nil, errors.New("shell model must be initialized")
	}
	if output == nil {
		return nil, errors.New("shell output must not be nil")
	}
	return &bubbleShell{model: model, input: input, output: output}, nil
}

func (shell *bubbleShell) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("shell context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	program := tea.NewProgram(
		shell.model,
		tea.WithContext(ctx),
		tea.WithInput(shell.input),
		tea.WithOutput(shell.output),
	)
	_, err := program.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

var _ agenttui.Shell = (*bubbleShell)(nil)

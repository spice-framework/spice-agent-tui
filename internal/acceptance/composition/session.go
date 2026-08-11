package composition

import (
	"context"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

type session struct{}

func (session) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	<-ctx.Done()
	return agenttui.SessionUpdate{}, context.Cause(ctx)
}

func (session) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	return agenttui.CommandResult{}, context.Canceled
}

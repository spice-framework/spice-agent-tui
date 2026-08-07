package composition

import (
	"context"
	"slices"

	agenttui "github.com/spice-framework/spice-agent-tui"
	_ "github.com/spice-framework/spice-agent-tui/autoconfigure"
)

// @import { Application, Bean } from "github.com/spice-framework/spice/annotation/core"

// NewSession is the application-owned exact session bean required to activate
// the terminal-shell fallback.
//
// @Bean(name="acceptanceSession")
func NewSession() agenttui.Session { return session{} }

// BindingOrder captures the exact collection order selected by Spice.
type BindingOrder struct{ actions []agenttui.Action }

// Actions returns the generated binding order as a defensive copy.
func (order BindingOrder) Actions() []agenttui.Action { return slices.Clone(order.actions) }

// NewBindingOrder proves ordered collection injection through an ordinary bean.
//
// @Bean(name="bindingOrder")
func NewBindingOrder(bindings []agenttui.KeyBinding) BindingOrder {
	actions := make([]agenttui.Action, len(bindings))
	for index, binding := range bindings {
		actions[index] = binding.Action()
	}
	return BindingOrder{actions: actions}
}

// CompositionProof proves that one explicit blank import plus an
// application-owned Session resolves the public Shell root. Spice never
// executes this marker.
//
// @Application
func CompositionProof(agenttui.Shell, BindingOrder) {
	panic("Spice must never execute an application marker")
}

type session struct{}

func (session) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	<-ctx.Done()
	return agenttui.SessionUpdate{}, context.Cause(ctx)
}

func (session) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	return agenttui.CommandResult{}, context.Canceled
}

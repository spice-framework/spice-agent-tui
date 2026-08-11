package composition

import (
	"slices"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

// BindingOrder captures the exact collection order selected by Spice.
type BindingOrder struct{ actions []agenttui.Action }

// Actions returns the generated binding order as a defensive copy.
func (order BindingOrder) Actions() []agenttui.Action { return slices.Clone(order.actions) }

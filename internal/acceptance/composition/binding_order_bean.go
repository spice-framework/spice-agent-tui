package composition

import agenttui "github.com/spice-framework/spice-agent-tui"

// @import { Bean, Singleton } from "github.com/spice-framework/spice/annotation/core"

// NewBindingOrder proves ordered collection injection through an ordinary bean.
//
// @Bean(name="bindingOrder")
// @Singleton
func NewBindingOrder(bindings []agenttui.KeyBinding) BindingOrder {
	actions := make([]agenttui.Action, len(bindings))
	for index, binding := range bindings {
		actions[index] = binding.Action()
	}
	return BindingOrder{actions: actions}
}

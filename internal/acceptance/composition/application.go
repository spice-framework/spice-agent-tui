package composition

import (
	agenttui "github.com/spice-framework/spice-agent-tui"
	_ "github.com/spice-framework/spice-agent-tui/autoconfigure"
)

// @import { Application } from "github.com/spice-framework/spice/annotation/core"

// CompositionProof proves that one explicit blank import plus an
// application-owned Session resolves the public Shell root. Spice never
// executes this marker.
//
// @Application
func CompositionProof(agenttui.Shell, BindingOrder) {}

// Package wrapper is a negative fixture for defined interface wrappers.
package wrapper

import agenttui "github.com/spice-framework/spice-agent-tui"

// @import { UIShell } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

type Shell interface {
	agenttui.Shell
}

// NewShell returns a defined wrapper that must not impersonate Shell.
// @UIShell(name="wrapper-shell")
func NewShell() Shell { return nil }

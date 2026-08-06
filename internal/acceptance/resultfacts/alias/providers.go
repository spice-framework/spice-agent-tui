// Package alias proves that canonical result identity preserves Go aliases.
package alias

import agenttui "github.com/spice-framework/spice-agent-tui"

// @import { UIRenderer, UIShell } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

type (
	Shell    = agenttui.Shell
	Renderer = agenttui.Renderer
)

// NewShell returns an alias of the exact public Shell contract.
// @UIShell(name="alias-shell")
func NewShell() Shell { return nil }

// NewRenderer returns an alias of the exact public Renderer contract.
// @UIRenderer(name="alias-renderer")
func NewRenderer() Renderer { return nil }

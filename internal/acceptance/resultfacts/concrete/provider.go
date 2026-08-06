// Package concrete is a negative fixture for concrete provider results.
package concrete

import agenttui "github.com/spice-framework/spice-agent-tui"

// @import { UIRenderer } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

type Renderer struct{}

func (Renderer) Render(
	agenttui.ViewData,
	agenttui.Size,
	agenttui.Theme,
) (agenttui.Frame, error) {
	return agenttui.Frame{}, nil
}

// NewRenderer returns a concrete type that must not impersonate Renderer.
// @UIRenderer(name="concrete-renderer")
func NewRenderer() Renderer { return Renderer{} }

package ui

import (
	"context"

	"github.com/spice-framework/spice-agent-tui/annotation/uitool"
	"github.com/spice-framework/spice/annotation/sdk"
)

// UIRenderer marks an exported package-level factory whose first exact output
// is agenttui.Renderer or a valid alias selected by the generic typed compiler.
// It preserves ordinary Spice cleanup/error result forms, derives dependencies
// from Go parameters, and produces direct generated calls without reflection.
//
// Every selection property is source-visible: name is required, and aliases,
// qualifiers, primary, fallback, and order are opt-in. The annotation does not
// select the internal FixedRenderer, synthesize a model, or auto-configure a
// shell. Its trusted native handler emits only generic provider and bean
// metadata. Exact descriptor-result enforcement remains pending on the shared
// compiler's generic Invocation.Facts type-domain support; the handler does not
// parse TypeID or introduce TUI-specific compiler behavior. Callers must return
// the documented exact interface until that generic contract is available.
//
//	// @import { UIRenderer } from "github.com/spice-framework/spice-agent-tui/annotation/ui"
//	// @UIRenderer(name="fixed", aliases=["default-renderer"], fallback=true, order=100)
//	func NewFixedRenderer(config RenderConfig) agenttui.Renderer
func UIRenderer() sdk.Definition {
	return sdk.Definition{
		Name:    "ui.UIRenderer",
		Summary: "Declares an exact agenttui.Renderer provider with explicit bean metadata.",
		Targets: []sdk.Target{sdk.TargetFunction},
		Arguments: []sdk.Argument{
			{Name: "name", Kinds: []sdk.Kind{sdk.KindString}, Description: "Required canonical static renderer bean name.", Required: true},
			{Name: "aliases", Kinds: []sdk.Kind{sdk.KindList}, ListElementKinds: []sdk.Kind{sdk.KindString}, Description: "Optional unique canonical alternate bean names."},
			{Name: "qualifiers", Kinds: []sdk.Kind{sdk.KindList}, ListElementKinds: []sdk.Kind{sdk.KindString}, Description: "Optional unique canonical DI qualifiers."},
			{Name: "primary", Kinds: []sdk.Kind{sdk.KindBoolean}, Description: "Explicitly marks the preferred normal renderer candidate.", Default: "false"},
			{Name: "fallback", Kinds: []sdk.Kind{sdk.KindBoolean}, Description: "Explicitly marks a replaceable default renderer candidate.", Default: "false"},
			{Name: "order", Kinds: []sdk.Kind{sdk.KindInteger}, Description: "Deterministic order from -1000000 through 1000000.", Default: "0"},
		},
		Examples: []sdk.Example{{
			Title: "Explicit fallback renderer",
			Code:  "// @UIRenderer(name=\"fixed\", fallback=true, order=100)\nfunc NewFixedRenderer(config RenderConfig) agenttui.Renderer",
		}},
		Compatibility: sdk.Compatibility{Since: "0.1.0-preview.1", MinimumSpice: "0.1.0-preview.1"},
		Implementation: sdk.Implementation{
			Tool:     uitool.Path,
			Handler:  UIRendererHandler,
			Protocol: sdk.ProtocolV1Alpha2,
		},
	}
}

// UIRendererHandler validates invocation and bean metadata and contributes only
// generic provider records. The compiler validates provider shape.
func UIRendererHandler(ctx context.Context, invocation sdk.Invocation) (sdk.Result, error) {
	return providerMetadata(ctx, invocation, "UIRenderer")
}

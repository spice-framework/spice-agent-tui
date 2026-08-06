package ui

import (
	"context"

	"github.com/spice-framework/spice-agent-tui/annotation/uitool"
	"github.com/spice-framework/spice/annotation/sdk"
)

// UIShell marks an exported package-level factory whose first exact output is
// agenttui.Shell or a valid alias selected by the generic typed compiler. The
// factory may additionally return lifecycle.Cleanup and/or error using the
// ordinary Spice provider contract. Constructor parameters remain ordinary Go
// dependencies and generated code calls the factory directly.
//
// The required name and optional aliases, qualifiers, primary/fallback choice,
// and order are explicit static bean metadata. This annotation does not create
// a default shell, discover a daemon, infer terminal ownership, or register a
// runtime container. The trusted tool handler never guesses interface identity
// from strings. Exact descriptor-result enforcement is reserved for the shared
// compiler's upcoming generic Invocation.Facts type-domain support; this handler
// deliberately does not parse TypeID or add TUI-specific compiler behavior.
// Until that support lands, callers must return the documented exact interface
// and ordinary typed injection still fails closed when no Shell candidate exists.
//
//	// @import { UIShell } from "github.com/spice-framework/spice-agent-tui/annotation/ui"
//	// @UIShell(name="terminal", aliases=["interactive"], qualifiers=["primary-ui"], primary=true)
//	func NewTerminalShell(model Model, streams Streams) (agenttui.Shell, lifecycle.Cleanup, error)
func UIShell() sdk.Definition {
	return sdk.Definition{
		Name:    "ui.UIShell",
		Summary: "Declares an exact agenttui.Shell provider with explicit bean metadata.",
		Targets: []sdk.Target{sdk.TargetFunction},
		Arguments: []sdk.Argument{
			{Name: "name", Kinds: []sdk.Kind{sdk.KindString}, Description: "Required canonical static shell bean name.", Required: true},
			{Name: "aliases", Kinds: []sdk.Kind{sdk.KindList}, ListElementKinds: []sdk.Kind{sdk.KindString}, Description: "Optional unique canonical alternate bean names."},
			{Name: "qualifiers", Kinds: []sdk.Kind{sdk.KindList}, ListElementKinds: []sdk.Kind{sdk.KindString}, Description: "Optional unique canonical DI qualifiers."},
			{Name: "primary", Kinds: []sdk.Kind{sdk.KindBoolean}, Description: "Explicitly marks the preferred normal shell candidate.", Default: "false"},
			{Name: "fallback", Kinds: []sdk.Kind{sdk.KindBoolean}, Description: "Explicitly marks a replaceable default shell candidate.", Default: "false"},
			{Name: "order", Kinds: []sdk.Kind{sdk.KindInteger}, Description: "Deterministic order from -1000000 through 1000000.", Default: "0"},
		},
		Examples: []sdk.Example{{
			Title: "Explicit terminal shell",
			Code:  "// @UIShell(name=\"terminal\", qualifiers=[\"primary-ui\"], primary=true)\nfunc NewTerminalShell(model Model, streams Streams) agenttui.Shell",
		}},
		Compatibility: sdk.Compatibility{Since: "0.1.0-preview.1", MinimumSpice: "0.1.0-preview.1"},
		Implementation: sdk.Implementation{
			Tool:     uitool.Path,
			Handler:  UIShellHandler,
			Protocol: sdk.ProtocolV1Alpha2,
		},
	}
}

// UIShellHandler validates invocation and bean metadata and contributes only
// generic provider records. The compiler validates provider shape.
func UIShellHandler(ctx context.Context, invocation sdk.Invocation) (sdk.Result, error) {
	return providerMetadata(ctx, invocation, "UIShell")
}

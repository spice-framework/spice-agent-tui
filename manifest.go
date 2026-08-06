package agenttui

import "github.com/spice-framework/spice/starter"

// Manifest identifies the UI-contract and annotation SDK without activating a
// shell. Applications must explicitly construct semantic view data and declare
// their own @UIShell and @UIRenderer factories; dependency presence performs no
// auto-configuration.
var Manifest = starter.Must(starter.Spec{
	Schema:    starter.Schema,
	ID:        "github.com/spice-framework/spice-agent-tui",
	Version:   "0.1.0-dev",
	Module:    "github.com/spice-framework/spice-agent-tui",
	SpiceAPI:  starter.APIVersion,
	MinimumGo: "1.26.5",
	License:   "Apache-2.0",
	Review:    "docs/dependency-review.md",
	Activation: starter.Activation{
		Mode: starter.ActivationExplicitConstructor,
		EntryPoints: []starter.EntryPoint{{
			Package: "github.com/spice-framework/spice-agent-tui",
			Symbol:  "NewViewData",
		}},
	},
	Capabilities: []string{
		"agent.tui.annotations",
		"agent.tui.contracts",
		"agent.tui.presentation",
	},
	Dependencies: []starter.Dependency{
		{Module: "charm.land/bubbletea/v2", Version: "v2.0.8", License: "MIT"},
		{Module: "github.com/charmbracelet/x/ansi", Version: "v0.11.7", License: "MIT"},
	},
})

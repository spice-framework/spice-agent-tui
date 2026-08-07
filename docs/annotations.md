# TUI provider annotations

Spice Agent TUI publishes two explicit provider annotations:

- `@UIShell` for an exact `agenttui.Shell` provider or valid Go alias;
- `@UIRenderer` for an exact `agenttui.Renderer` provider or valid Go alias.

Both target exported package-level factory functions and require a canonical
`name`. They also accept `aliases`, `qualifiers`, `primary`, `fallback`, and
`order`. A bean cannot be both primary and fallback; identities must be unique,
lowercase canonical values, and order is bounded from -1,000,000 through
1,000,000.

```go
// @import { UIShell, UIRenderer } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

// @UIShell(name="terminal", aliases=["interactive"], qualifiers=["primary-ui"], primary=true)
func NewTerminalShell(model Model, streams Streams) (
    agenttui.Shell,
    lifecycle.Cleanup,
    error,
)

// @UIRenderer(name="fixed", fallback=true, order=100)
func NewFixedRenderer(config RenderConfig) agenttui.Renderer
```

The consuming application's root `go.mod` selects and authorizes the trusted
native handler with standard Go tooling:

```text
go get -tool github.com/spice-framework/spice-agent-tui/cmd/spice-agent-tui-annotations@<version>
```

The annotation tool path is
`github.com/spice-framework/spice-agent-tui/cmd/spice-agent-tui-annotations`.
It runs only as `go tool <path> --spice-stdio` and implements the public Spice
v1alpha2 framed protocol. Normal analysis uses the already selected module graph
and must not download or install anything in the background.

There is no type-name shortcut in either handler. The shared typed compiler
publishes bounded facts for every function result: readable identity, canonical
identity after alias removal, effective Go kind, and named origin. The handler
requires result zero to be the exact public `agenttui.Shell` or
`agenttui.Renderer` interface and accepts aliases through canonical identity.
Defined wrapper interfaces, anonymous interfaces, concrete results, missing or
malformed facts, and forged origins are rejected at the annotation position.

Supported output layouts are `T`, `(T, error)`, `(T, lifecycle.Cleanup)`, and
`(T, lifecycle.Cleanup, error)`. The handler validates the auxiliary identities
but never executes the function or parses `Declaration.TypeID`. Constructor
parameters and direct generated calls remain generic compiler responsibilities.
Real-tool acceptance executes the pinned Spice CLI against alias-positive and
wrapper/anonymous/concrete negative fixtures.

These annotations deliberately do not create a model, client session, input,
output, shell, or renderer. They do not expose the internal presentation
package, scan at runtime, mutate a container, or activate through dependency
presence. Applications that want the library defaults must separately
blank-import `github.com/spice-framework/spice-agent-tui/autoconfigure` and
supply an exact `agenttui.Session` bean. The committed composition-proof target
demonstrates that path through ordinary generated Go.

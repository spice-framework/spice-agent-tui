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

There is no type-name shortcut in either handler. A handler confirms only that
the declaration is an exported package-level factory and validates its explicit
selection metadata, then returns generic provider and bean metadata. The shared typed compiler validates
ordinary provider signatures such as `(T, error)`, `(T, lifecycle.Cleanup)`, and
`(T, lifecycle.Cleanup, error)`, and generated code calls factories directly.

One contract is deliberately pending: Spice's public SDK does not yet expose the
generic `Invocation.Facts` type-domain information needed for a descriptor to
require that its first result is exactly `Shell` or `Renderer` while preserving
Go alias identity. Factories must currently follow the documented result type;
ordinary typed injection still fails closed when no matching interface bean
exists, but the descriptor-specific source diagnostic will arrive with that
shared SDK/toolchain feature. The TUI tool will not parse `Declaration.TypeID`,
guess assignability from a string, or introduce annotation-name switches.

These annotations deliberately do not create a model, client session, input,
output, shell, or renderer. They do not expose the internal presentation
package, scan at runtime, mutate a container, or activate through dependency
presence. Shell auto-configuration and a generated application remain deferred
until the client/model/stream ownership contracts are stable.

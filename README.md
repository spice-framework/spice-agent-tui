# Spice Agent TUI

This repository provides the UI foundation for the standalone Spice Agent
terminal client. Its first production slice includes:

- bounded immutable semantic view, status, prompt, keybinding, and theme values;
- a deterministic fixed-size renderer with distinct light and dark palettes;
- a Bubble Tea v2 model with resize, Unicode editing, and quit handling; and
- a cancellation-aware shell with injected input and output;
- canonical `@UIShell` and `@UIRenderer` provider annotations; and
- an authorized v1alpha2 stdio annotation tool selected through Go modules.

The source intentionally does **not** pretend that the agent connection exists.
There is no daemon discovery, transport, client/session adapter, shell
auto-configuration, generated Spice application, or terminal binary yet. The next slice must
adopt the reviewed high-level client contract and inject it through an
application composition root. The presentation package will never import the
agent kernel, generated gRPC, daemon supervision, or operating-system IPC.

The public package is renderer-neutral. Bubble Tea is confined to
`internal/presentation`, so semantic data and commands remain straightforward to
test and reuse. All semantic text rejects terminal control sequences, aggregate
views and frames are bounded, snapshots clone caller-owned slices, and every
render has an exact visible width and height.

Applications opt into annotation metadata explicitly:

```go
// @import { UIShell, UIRenderer } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

// @UIShell(name="terminal", primary=true)
func NewTerminalShell(model Model, streams Streams) agenttui.Shell

// @UIRenderer(name="fixed", fallback=true)
func NewFixedRenderer(config RenderConfig) agenttui.Renderer
```

The handlers contribute only generic Spice provider and bean metadata.
Constructor parameters, cleanup, error forms, and all interface identity logic
remain owned by the shared typed compiler. Descriptor-specific enforcement that
an annotated result is exactly `Shell` or `Renderer` awaits the compiler's
generic `Invocation.Facts` type-domain support; no string parser or TUI-specific
compiler switch substitutes for it. See `docs/annotations.md`.

Go 1.26.5 is exact. On a fresh clone, run `make tools-bootstrap` once to
populate the exact product and tools module graphs without changing tracked
module files. All ordinary quality targets remain offline. Use `make fast`
while editing, `make check` for the broader loop, and `make verify` before every
commit. See `docs/verification.md` for the complete contract.

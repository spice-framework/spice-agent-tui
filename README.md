# Spice Agent TUI

This repository provides the UI foundation for the standalone Spice Agent
terminal client. Its first production slice includes:

- bounded immutable semantic view, status, prompt, keybinding, and theme values;
- a deterministic fixed-size renderer with distinct light and dark palettes,
  explicit textual status labels, and an accessible unstyled mode;
- a Bubble Tea v2 model with deterministic resize, revisioned streaming
  snapshots, bounded rolling activity and prompt history, and keyboard
  navigation;
- constructor-injected ordered key bindings with deterministic duplicate-action
  and duplicate-key rejection, including separate submit, cancel-run, respond,
  and explicit Ctrl-C quit actions;
- bounded immutable terminal configuration, terminal I/O, command invocation,
  command result, and semantic intent values;
- a command-owned asynchronous effects seam with run-context cancellation,
  one active receive, stale-token rejection, no optimistic prompt loss, and a
  no-I/O fallback;
- grapheme-safe Unicode editing with a display-cell-positioned cursor; and
- a cancellation-aware shell with injected input/output and clean Ctrl-C exit;
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
render has an exact visible width and height. Presentation updates use positive,
monotonically increasing revisions; stale updates are ignored and rolling
activity evicts its oldest entries before exceeding view limits. This is a
client-neutral Bubble Tea message boundary, not a session or transport API.

`StatusDisconnected`, `StatusReconnecting`, and `StatusError` are rendered as
literal bracketed labels, so status never depends on color alone. Accessible
mode emits line-oriented semantic text without ANSI styling, alternate-screen
presentation, cursor control, fixed-canvas padding, or resize-only replay.
Home/End and Ctrl+A/Ctrl+E
move by prompt boundaries; Up/Down navigate a maximum of 64 injected history
entries. Left/Right and Backspace operate on Unicode grapheme clusters rather
than splitting combining text or joined emoji.

Key policy is explicit application composition. `StandardKeyBindings` is a
convenience bean candidate, never an implicit model fallback: Ctrl-C/Ctrl-Q
quit, Escape/Ctrl-X cancel the active run, Enter submits, and Alt-Enter responds
to a pending interaction. The model copies and validates injected bindings in
order. An application may replace the entire set without modifying presentation
code.

`TerminalConfig` selects an exact server-owned definition ID and revision plus
accessibility and bounded shutdown policy. `TerminalIO` contains caller-owned
streams. Neither value discovers, starts, or attaches to a daemon. Likewise,
commands accept only an immutable bounded invocation and return a typed bounded
result with an optional semantic intent; injected services never travel through
an invocation or registry.

Applications opt into annotation metadata explicitly:

```go
// @import { UIShell, UIRenderer } from "github.com/spice-framework/spice-agent-tui/annotation/ui"

// @UIShell(name="terminal", primary=true)
func NewTerminalShell(model Model, terminal agenttui.TerminalIO, config agenttui.TerminalConfig) agenttui.Shell

// @UIRenderer(name="fixed", fallback=true)
func NewFixedRenderer(config RenderConfig) agenttui.Renderer
```

The handlers contribute only generic Spice provider and bean metadata. They
decode the shared compiler's generic result facts and require exact canonical
`Shell` or `Renderer` identity with interface kind and public named origin.
Real Go aliases work; defined wrappers, anonymous interfaces, concrete results,
missing facts, and malformed facts fail closed. The handlers never parse
`Declaration.TypeID`. Constructor parameters and generated calls remain owned
by the shared typed compiler. See `docs/annotations.md`.

The module pins the exact Spice core and toolchain revisions that define and
produce these facts. Repository acceptance runs the real pinned `spice verify`
tool against alias-positive and source-positioned negative fixtures. This is a
compiler-development dependency only; no client, daemon, or transport wiring
has been added.

Go 1.26.5 is exact. On a fresh clone, run `make tools-bootstrap` once to
populate the exact product and tools module graphs without changing tracked
module files. All ordinary quality targets remain offline. Use `make fast`
while editing, `make check` for the broader loop, and `make verify` before every
commit. See `docs/verification.md` for the complete contract.

# Spice Agent TUI

This repository provides the UI foundation for the standalone Spice Agent
terminal client. Its first production slice includes:

- bounded immutable semantic view, status, prompt, keybinding, and theme values;
- a deterministic fixed-size renderer with distinct light and dark palettes;
- a Bubble Tea v2 model with resize, Unicode editing, and quit handling; and
- a cancellation-aware shell with injected input and output.

The source intentionally does **not** pretend that the agent connection exists.
There is no daemon discovery, transport, client/session adapter, annotation
integration, generated Spice application, or binary yet. The next slice must
adopt the reviewed high-level client contract and inject it through an
application composition root. The presentation package will never import the
agent kernel, generated gRPC, daemon supervision, or operating-system IPC.

The public package is renderer-neutral. Bubble Tea is confined to
`internal/presentation`, so semantic data and commands remain straightforward to
test and reuse. All semantic text rejects terminal control sequences, aggregate
views and frames are bounded, snapshots clone caller-owned slices, and every
render has an exact visible width and height.

Go 1.26.5 is exact. Use `make fast` while editing, `make check` for the broader
loop, and `make verify` before every commit.

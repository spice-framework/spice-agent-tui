# Architecture

## Ownership

This repository owns the terminal product, not an agent daemon, transport,
or framework runtime. Its product surface is the Bubble Tea v2 shell and the
renderers, editor, commands, keybindings, terminal input, and accessibility
behavior composed within that shell.

The intended dependency direction is:

```text
public immutable values --> Bubble Tea v2 presentation
                                      |
                                      +--> later application composition
                                                   |
                                                   +--> adopted client/session
```

The root package owns library-neutral contracts and immutable semantic values.
It has no Bubble Tea dependency in its public signatures. Text rejects terminal
controls, slices are defensively copied, render work is bounded, and prompt
cursor positions are Unicode rune boundaries.

`internal/presentation` owns deterministic terminal rendering, input translation,
and the Bubble Tea program lifecycle. The renderer is pure: a semantic snapshot,
bounded size, and theme always produce the same fixed-size frame. Bubble Tea
v2.0.8 is pinned through its canonical `charm.land/bubbletea/v2` module path.

`internal/application` owns terminal-product composition and the lifecycle of
the injected client session. Neither package may import the agent kernel,
generated gRPC packages, daemon hosting or supervision, or operating-system IPC.
Those responsibilities remain behind the adopted high-level client/session
contract.

There is deliberately no command entrypoint yet. A no-op or disconnected binary
would create a misleading executable contract. The current shell is a tested
library boundary; constructing a real model remains the later composition root's
responsibility.

## Compatibility

`compatibility.json` records Go 1.26.5, the local `v0.1.0-dev` UI-value contract,
and explicit null values for the high-level client/session, Spice core, and
toolchain contracts that do not exist here yet. Replacing a null requires an
adopted contract, immutable version selection, and executable compatibility
tests.

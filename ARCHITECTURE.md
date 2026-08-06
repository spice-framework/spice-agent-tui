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
           +--> public annotation descriptors --> v1alpha2 tool --> generic Spice compiler
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

`annotation/ui` is the only public descendant annotation package and is the
module's named `annotations` interface. Each annotation has one canonical file
containing rich GoDoc, its statically decoded descriptor, and its typed handler.
`internal/annotationtool` owns isolated sorted protocol dispatch, and
`cmd/spice-agent-tui-annotations` is only its `go tool` stdio boundary. The tool
returns generic `ProviderContribution` and `BeanMetadataContribution` records;
it never decides whether a result type implements `Shell` or `Renderer`.
Interface and alias identity, dependency inputs, cleanup, and error forms stay
in the shared Spice `go/types` pipeline. The SDK does not yet expose the generic
`Invocation.Facts` type-domain contract needed to assert that these two
descriptor results are exactly the documented interfaces. That follow-up belongs
to Spice/toolchain; this module does not parse `TypeID` or add name-based
compiler logic as a workaround.

`internal/application` owns terminal-product composition and the lifecycle of
the injected client session. Neither package may import the agent kernel,
generated gRPC packages, daemon hosting or supervision, or operating-system IPC.
Those responsibilities remain behind the adopted high-level client/session
contract.

There is deliberately no terminal-product command entrypoint yet. The annotation
tool is compiler protocol infrastructure, not a user-facing TUI. A no-op or disconnected binary
would create a misleading executable contract. The current shell is a tested
library boundary; constructing a real model remains the later composition root's
responsibility.

## Compatibility

`compatibility.json` records Go 1.26.5, the local `v0.1.0-dev` UI-value contract,
and exact Spice core `v0.1.0-preview.1`. The high-level client/session and
toolchain remain explicit null contracts. Replacing a null requires an
adopted contract, immutable version selection, and executable compatibility
tests.

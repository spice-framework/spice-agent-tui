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
bounded size, and theme always produce the same fixed-size frame. Its terminal
cursor is a validated display-cell coordinate, while editing moves only across
Unicode grapheme boundaries. Accessible mode emits the same explicit status
semantics without ANSI or alternate-screen presentation. Bubble Tea v2.0.8 is
pinned through its canonical `charm.land/bubbletea/v2` module path.

Presentation state accepts validated, monotonically revisioned snapshot and
activity messages. Stale revisions are ignored, activity is a rolling
oldest-first-evicted window within the public item and byte bounds, and injected
prompt history is capped at 64 entries. Ordered key bindings are constructor
injected, copied into immutable bindings, and rejected deterministically when
actions or keystrokes collide. Ctrl-C is explicit shell shutdown; submit,
cancel-active-run, and interaction response are distinct semantic actions.

The model performs no I/O in `Update`. Its internal `Effects` seam returns
Bubble Tea commands that own blocking receives and semantic operations. Shell
startup binds those commands to the caller-owned run context. A single receive
is armed at a time, operation tokens reject stale completions, and submitted
prompts enter history only after generic success. Definite failures retain the
draft and are never retried by presentation code. With no injected effects the
same model is a tested no-I/O fallback. These contracts do not define a client,
daemon, transport, reconnect timer, retry policy, or session ownership; an
adopted session may translate immutable events into this boundary later.

Accessible presentation is a semantic line stream rather than a styled,
fixed-size canvas. It never requests the alternate screen or cursor, never
contains ANSI sequences, and does not change merely because the terminal is
resized. The normal renderer retains exact fixed-cell light/dark output.

`annotation/ui` is the only public descendant annotation package and is the
module's named `annotations` interface. Each annotation has one canonical file
containing rich GoDoc, its statically decoded descriptor, and its typed handler.
`internal/annotationtool` owns isolated sorted protocol dispatch, and
`cmd/spice-agent-tui-annotations` is only its `go tool` stdio boundary. The tool
returns generic `ProviderContribution` and `BeanMetadataContribution` records.

The shared Spice `go/types` pipeline produces bounded v1alpha2 function-result
facts. TUI handlers decode those facts and require the first provider result to
have interface kind, exact canonical public `Shell` or `Renderer` identity, and
the matching named origin. Aliases preserve a readable source identity while
canonicalizing to the public contract. Defined wrappers, anonymous interfaces,
concrete results, malformed metadata, and unsupported cleanup/error layouts are
rejected. The handler never parses `Declaration.TypeID`, performs assignability,
or adds TUI-specific behavior to the compiler.

Public terminal startup values contain explicit caller-owned input/output,
accessibility selection, exact server-owned definition ID and revision, and a
bounded shutdown timeout. They perform no discovery or acquisition. Public
commands receive only a bounded immutable invocation and return a bounded typed
result/intent; services remain constructor dependencies and command IDs are
validated for deterministic collisions.

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
and exact result-facts Spice core and toolchain pseudo-versions. The toolchain is
a compiler/development tool, not a runtime dependency. The high-level
client/session remains an explicit null contract; replacing it requires an
adopted contract, immutable version selection, and executable compatibility
tests.

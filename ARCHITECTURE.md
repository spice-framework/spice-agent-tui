# Architecture

## Ownership

This repository will own the terminal product, not an agent daemon, transport,
or framework runtime. Its product surface is the Bubble Tea v2 shell and the
renderers, editor, commands, keybindings, terminal input, and accessibility
behavior composed within that shell.

The intended dependency direction is:

```text
Bubble Tea v2 presentation --+
                             +--> application --> adopted high-level client/session
UI-neutral values -----------+
```

`internal/presentation` owns terminal rendering and input translation.
`internal/application` owns terminal-product composition and the lifecycle of
the injected client session. Neither package may import the agent kernel,
generated gRPC packages, daemon hosting or supervision, or operating-system IPC.
Those responsibilities remain behind the adopted high-level client/session
contract.

There is deliberately no command entrypoint in the repository foundation. A
no-op binary would create a misleading executable contract. Bubble Tea v2 is the
accepted presentation architecture, but it remains unpinned until real shell
code and its cancellation, accessibility, Windows, and Linux tests exist.

## Compatibility

`compatibility.json` records Go 1.26.5 and explicit null values for the
high-level client/session and UI-neutral value contracts that do not exist yet.
Replacing a null requires an adopted contract, immutable version selection, and
executable compatibility tests.

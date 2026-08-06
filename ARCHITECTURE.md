# Architecture

## Ownership

This repository will own a terminal client, not an agent daemon or a framework
runtime. The intended dependency direction is:

```text
presentation -> application -> adopted client/transport contract
```

`internal/presentation` owns terminal rendering and input translation.
`internal/application` owns client lifecycle and composition.
`internal/transport` will adapt an externally owned, versioned Spice Agent
protocol. It must not define a private competing protocol.

There is deliberately no command entrypoint in Phase 0. A no-op binary would
create a misleading executable contract. There is also no Bubble Tea
dependency: a UI library will be selected only when real presentation code and
its cancellation, accessibility, Windows, and Linux tests exist.

## Compatibility

`compatibility.json` records Go 1.26.5 and explicit null values for contracts
that do not exist yet. Replacing a null requires an architecture decision,
immutable version selection, and executable compatibility tests.

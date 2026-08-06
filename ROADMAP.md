# Roadmap

Spice Agent has one authoritative sequence of implementation outcomes: the
[canonical implementation ledger](https://github.com/spice-framework/spice-agent/tree/main/docs/implementation).
This repository does not assign parallel phase numbers. It delivers the TUI
portion of that ledger while the owning repositories deliver the client,
protocol, daemon, and distribution contracts.

## Repository foundation

- [x] Establish Apache-2.0 licensing and governance.
- [x] Enforce Go 1.26.5 with cross-platform fast/check/verify gates.
- [x] Reserve package ownership without publishing speculative APIs.
- [x] Record explicit unselected compatibility boundaries.
- [ ] With the first real generated TUI slice, pin exact compatible Spice core
      and toolchain versions, publish the TUI starter manifest, and register its
      compatibility and verification gates in the development catalog.

The checked items describe this repository's scaffold only. Its portion of the
canonical multi-repository foundation is not complete until the pending pin,
manifest, and catalog work above can be proved by executable product code.

## Adopted dependencies

- [ ] Adopt the separately reviewed high-level Spice Agent client/session API.
- [ ] Adopt UI-neutral interaction and semantic-view values.
- [ ] Define cancellation, reconnect, backpressure, and transcript-redaction behavior.
- [ ] Add a deterministic fake client/session and lifecycle tests.
- [ ] Keep the kernel, generated gRPC, daemon supervision, and OS IPC outside
      this module's dependency graph.

## Terminal product

- [ ] Review and pin Bubble Tea v2 with the first production shell code.
- [ ] Implement the shell, renderers, editor, commands, and keybindings.
- [ ] Implement accessible keyboard navigation, resizing, streaming, and errors.
- [ ] Verify packaged Windows and Linux executables against a real daemon.

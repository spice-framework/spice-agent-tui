# Roadmap

Spice Agent has one authoritative sequence of implementation outcomes: the
[canonical implementation ledger](https://github.com/spice-framework/spice-agent/tree/main/docs/implementation).
This repository does not assign parallel phase numbers. It delivers the TUI
portion of that ledger while the owning repositories deliver the client,
protocol, daemon, and distribution contracts.

## Repository foundation

- [x] Establish Apache-2.0 licensing and governance.
- [x] Enforce Go 1.26.5 with cross-platform fast/check/verify gates.
- [x] Publish the bounded UI-neutral values needed by real presentation code.
- [x] Record explicit unselected compatibility boundaries.
- [ ] With the first real generated TUI slice, pin exact compatible Spice core
      and toolchain versions, publish the TUI starter manifest, and register its
      compatibility and verification gates in the development catalog.

The checked items describe this repository's scaffold only. Its portion of the
canonical multi-repository foundation is not complete until the pending pin,
manifest, and catalog work above can be proved by executable product code.

## Adopted dependencies

- [ ] Adopt the separately reviewed high-level Spice Agent client/session API.
- [x] Define UI-neutral interaction and semantic-view values locally.
- [ ] Define cancellation, reconnect, backpressure, and transcript-redaction behavior.
- [ ] Add a deterministic fake client/session and lifecycle tests.
- [ ] Keep the kernel, generated gRPC, daemon supervision, and OS IPC outside
      this module's dependency graph.

## Terminal product

- [x] Review and pin Bubble Tea v2.0.8 with the first production shell code.
- [x] Implement the deterministic shell/model skeleton, renderer, editor, and
      keybindings without faking application integration.
- [ ] Adopt application commands only after the client/session contract exists.
- [ ] Implement accessible keyboard navigation, resizing, streaming, and errors.
- [ ] Verify packaged Windows and Linux executables against a real daemon.

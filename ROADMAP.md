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
- [x] Record explicit selected and unselected compatibility boundaries.
- [x] Pin the exact result-facts Spice core and toolchain revisions and publish the explicit-constructor TUI
      starter manifest without claiming shell auto-configuration.
- [x] Prove generic result facts through the real tool protocol with alias-positive
      and source-positioned negative compiler fixtures.
- [ ] Register the compatible revisions in the development catalog with the
      first generated application slice.

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
- [x] Publish `@UIShell` and `@UIRenderer` through a typed v1alpha2 Go tool;
      preserve generic compiler ownership of exact interface identity.
- [x] Adopt generic `Invocation.Facts` type-domain support from Spice/toolchain
      to enforce exact `Shell` and `Renderer` annotation results, including aliases.
- [ ] Add shell auto-configuration only after model, client, input, and output
      ownership contracts are stable.
- [ ] Adopt application commands only after the client/session contract exists.
- [ ] Implement accessible keyboard navigation, resizing, streaming, and errors.
- [ ] Verify packaged Windows and Linux executables against a real daemon.

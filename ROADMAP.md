# Roadmap

## Phase 0 — repository foundation

- [x] Establish Apache-2.0 licensing and governance.
- [x] Enforce Go 1.26.5 with cross-platform fast/check/verify gates.
- [x] Reserve package ownership without publishing speculative APIs.
- [x] Record explicit unselected compatibility boundaries.

## Phase 1 — adopted client contract

- [ ] Adopt the separately reviewed Spice Agent protocol and client API.
- [ ] Define cancellation, reconnect, backpressure, and transcript-redaction behavior.
- [ ] Add deterministic fake transport and lifecycle tests.

## Phase 2 — real terminal product

- [ ] Review and, if selected, pin Bubble Tea for production presentation code.
- [ ] Implement accessible keyboard navigation, resizing, streaming, and errors.
- [ ] Verify packaged Windows and Linux executables against a real daemon.

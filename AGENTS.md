# Spice Agent TUI implementation contract

## Mission

Build the standalone terminal experience for Spice Agent after the protocol
and host contracts have explicit owners. Phase 0 establishes repository and
quality boundaries; it does not invent those contracts.

## Invariants

- Go 1.26.5 is mandatory.
- Terminal presentation, input handling, and client application composition
  belong here. Daemon behavior, coding-agent policy, compiler behavior, and
  protocol ownership do not.
- Do not add a transport or session API until its owning contract is adopted.
- Do not introduce Bubble Tea merely as a placeholder. Pin it only with real,
  tested presentation code and a recorded dependency review.
- Commands use discrete arguments without a shell. Ordinary verification after
  dependency preparation is offline and workspace-isolated.
- Do not commit credentials, transcripts, prompts, model output, local state,
  generated binaries, or terminal recordings containing user data.

## Delivery

Work directly on local `main` in bounded commits. Fetch before work and again
before push. Every commit must pass `make verify`; use `make fast` and
`make check` for shorter feedback. Stop if `origin/main` moves unexpectedly.


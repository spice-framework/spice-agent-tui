# Alternate semantic shell evidence

Phase 7 asks whether a second client can consume Spice Agent TUI semantics
without inheriting the Bubble Tea implementation. The nested
[`experiments/semantic-shell`](../experiments/semantic-shell) module is that
stress prototype.

It pins the published `spice-agent-tui v0.1.0-preview.1` module with no local
replacement and imports only its public, UI-neutral `Session`, `SessionUpdate`,
and `Intent` values. The implementation is a standard-library line shell and
deterministic JSONL projector. It has no Bubble Tea, terminal-plugin, daemon,
transport, or network dependency.

The proof covers:

- one receive owner, one serial ordinary-operation lane, and a separately
  available cancel lane;
- strictly increasing semantic revisions and output sequences;
- fixed secret-safe errors, panic containment, caller cancellation, no retry,
  and bounded command, queue, state, and output sizes;
- invalid/stale update rejection, concurrent lane use, and cancel while submit
  is blocked; and
- an immutable compatibility manifest, committed vendor graph, deterministic
  benchmarks, and a complete deletion path.

Repository `fast`, `check`, dependency bootstrap, and `verify` gates enter the
nested module explicitly. Full verification recreates its vendor tree, runs
shuffled and race tests, enforces 85% statement coverage, and builds offline.
This is experimental evidence, not a production command or a compatibility
freeze; authoritative phase status remains in the core Agent implementation
ledger.

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

A separate conformance-only adapter pins Agent commit
`b205307d3b5fb262401c77d1af902b1ce926d49a` through its exact pseudo-version.
On Linux and Windows the test binary launches independent 1.2-capped legacy and
current 1.3 exact-replay child peers, authenticates over a private Unix socket
or Windows named pipe, and drives the unchanged JSONL submit, respond, cancel,
and quit workflow through Agent's public client and wire validators. The child
peers are source-built test fixtures. No released daemon or client binary is
executed, so this is explicitly not N/N-1 binary compatibility evidence.

The proof covers:

- one receive owner, one serial ordinary-operation lane, and a separately
  available cancel lane;
- strictly increasing semantic revisions and output sequences;
- fixed secret-safe errors, panic containment, caller cancellation, no retry,
  and bounded command, queue, state, and output sizes;
- invalid/stale update rejection, concurrent lane use, and cancel while submit
  is blocked; and
- an immutable compatibility manifest, committed vendor graph, deterministic
  benchmarks, and a complete deletion path; and
- exact legacy-1.2 versus current-1.3 initialization semantics, authenticated
  real local IPC, mutation delivery, child cleanup, and socket removal.

Repository `fast`, `check`, dependency bootstrap, and `verify` gates enter the
nested module explicitly. Full verification recreates its vendor tree, runs
shuffled and race tests, enforces 85% statement coverage, and builds offline.
This is experimental evidence, not a production command or a compatibility
freeze; authoritative phase status remains in the core Agent implementation
ledger.

## Released public-module version skew

The repository also owns a distinct, explicit hosted matrix at
[`compatibility/released-client-matrix.json`](../compatibility/released-client-matrix.json).
It source-builds one semantic adapter against each exact public TUI generation
(`v0.1.0-preview.1` and `v0.1.0-preview.2`) and each exact public Agent peer
generation (`v0.1.0-preview.5` and `v0.1.0-preview.6`). The four old/current
client-peer lanes run on both Linux and Windows.

Each job creates a fresh temporary module, sets `GOWORK=off`, uses fresh module,
GOPATH, and build caches, downloads only through the public Go proxy and SumDB,
and verifies the published module sums and origin commits before compiling. The
runner imports only public TUI, Agent client, protocol, endpoint, and local-IPC
packages. It has no `replace` directive and no dependency on the checked-out
production module.

The semantic contract authenticates initialization, refuses a wrong token,
then performs submit, respond, and cancel through the released TUI `Session`
surface and released Agent client. Every lane is bounded and proves that the
session, transport connection, peer, listener, and Unix socket are cleaned up.
This is a released **Go module** source-build matrix; the manifest deliberately
does not claim a prebuilt-executable matrix. Ordinary repository gates validate
the manifest, runner digest, workflow, and public-only boundary offline. Only
the dedicated `released-version-skew` mode and hosted matrix enable network
access.

The earlier source-built 1.2/1.3 peer proof and the specialized snapshot,
deterministic replay, lifecycle, and accessibility tests remain in their
original gates; the released-module matrix supplements rather than replaces
them.

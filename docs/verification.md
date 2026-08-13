# Verification

On a fresh clone, explicitly populate the exact product and tools module graphs:

```text
make tools-bootstrap
```

This is the ordinary dependency-bootstrap network mode. It requires Go 1.26.5, validates
the repository identity and exact tool pins, downloads `all` from private
temporary copies of the product, tools, and semantic-shell experiment module
graphs, disables Go authentication,
and permits only the public checksum database and module proxy. It verifies that
the repository is byte-for-byte unchanged even when a download fails. A
repository without a tools module is valid. No API keys, tokens, passwords, or
secrets are passed to the Go subprocess.

The hosted released-client matrix uses a separate explicit network-enabled
mode, one lane at a time:

```text
go run ./internal/qualitygate -mode=released-version-skew -lane=previous-client-current-peer
```

That mode creates fresh temporary caches and a temporary module containing the
digest-pinned semantic runner. It downloads exact public TUI and Agent versions
through `proxy.golang.org` and `sum.golang.org`, verifies sums and origin
commits, then runs the authenticated semantic contract. The four lanes run on
both Linux and Windows in `.github/workflows/released-version-skew.yml`.
Ordinary `fast`, `check`, and `verify` remain offline and validate only the
matrix manifest, runner source digest, and workflow boundary.

Every child Go command uses the selected Go 1.26.5 binary from `runtime.GOROOT`,
not an older `go` that may appear first on `PATH`.

- `make fast` validates repository identity and runs shuffled root and
  semantic-shell tests.
- `make check` adds formatting, module/vendor consistency, vet, and shuffled
  tests, including the exact schema-2 application-style boundary and a
  byte-current nested experiment vendor proof.
- `make benchmark` runs the six adopted deterministic runtime benchmarks and
  two semantic-shell experiment benchmarks as five fixed 500-iteration,
  single-CPU samples with their offline vendor graphs.
- `make verify` adds lint, NilAway, gosec, govulncheck, race tests, coverage, and
  vendor-offline tests/builds. It also runs one second each of the canonical
  trace replay, VT chunk-boundary, and accessible-Unicode/control fuzz targets
  with a single worker,
  including the annotation tool smoke path, real
  pinned Spice compiler fixtures for alias acceptance and invalid result types,
  byte-current generated public auto-configuration composition, and the pinned
  Toolchain preview4 `spicestyle` verifier over only the composition fixture and
  its exact generated ownership root.
  All applicable style rules are errors. `moduleOwnership` alone is documented
  as inapplicable because schema-2 roots cannot select `.`, and the independent
  composition command always loads both root and fixture to retain the exact
  Modulith dependency proof.
  Generated `internal/spicegen` packages remain compilation and execution
  inputs, but are excluded from the handwritten-product coverage denominator.
  Presentation acceptance includes fixed light/dark goldens at normal, compact
  Unicode, and 1x1 boundary sizes; exact row/column and cursor-cell assertions;
  revision/stale-update, rolling-bound, history-navigation, resize-sequence,
  accessible-mode and resize-stability, injected-binding collision, semantic
  action, command/effect ownership, stale-operation, prompt-commit,
  concurrent-render, Ctrl-C, blocked-receive cancellation, Session panic,
  one-shot operation, late-cancellation result precedence, concurrent Session
  lanes, cancel control-lane availability, tagged-update, facade, Theme snapshot,
  public-facade prompt/submit/cancel/Ctrl-Q terminal interaction,
  deterministic `tuittest` scripts, strict canonical JSON parsing, independent
  double replay, per-event full-screen digests and reference invariants,
  committed lifecycle interaction goldens, grapheme input, injected key maps,
  command timeout cancellation, exact styled/plain/report goldens, missing-fixture
  refusal, ScriptSession queue/close behavior, and bounded VT cell, cursor,
  alternate-screen, resize, transcript, chunking, and wait behavior, plus a
  real current-test-binary Unix PTY/Windows ConPTY handshake covering TTY
  identity, input, output, resize, terminal modes, exit, and cleanup,
  exact auto-configuration order, and external generated-shell normal-exit tests.
  The nested semantic-shell module additionally runs offline shuffled and race
  tests, enforces 85% statement coverage, and builds from its committed vendor
  graph. See [Alternate semantic shell evidence](semantic-shell-experiment.md).

Repository identity also validates `.github/workflows/release.yml` as a
single-job, secret-free caller of the organization keyless Go-module release
workflow at exact audited commit
`a56c451168aae0f2b3075782156d204d75fb7f69`. The caller must deny permissions
at the workflow level and may grant only `contents`, `id-token`,
`attestations`, and `artifact-metadata` writes to the reusable release job.
Extra permissions, local steps, additional jobs, legacy workflows, module
drift, and either named or inherited secrets fail every verification mode
before product tests run.

The repository-owned verifier is cross-platform. `make fast`, `make check`, and
`make verify` force `GOPROXY=off`; missing cache entries fail instead of causing
hidden downloads. `make fmt` is the only target that rewrites Go source.

See [Runtime benchmarks](benchmarks.md) for the deterministic, offline,
threshold-free session and presentation baselines produced by the
repository-owned benchmark gate.

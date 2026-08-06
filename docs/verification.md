# Verification

On a fresh clone, explicitly populate the exact product and tools module graphs:

```text
make tools-bootstrap
```

This is the only network-enabled quality mode. It requires Go 1.26.5, validates
the repository identity and exact tool pins, downloads `all` from private
temporary copies of both `go.mod`/`go.sum` pairs, disables Go authentication,
and permits only the public checksum database and module proxy. It verifies that
the repository is byte-for-byte unchanged even when a download fails. A
repository without a tools module is valid. No API keys, tokens, passwords, or
secrets are passed to the Go subprocess.

Every child Go command uses the selected Go 1.26.5 binary from `runtime.GOROOT`,
not an older `go` that may appear first on `PATH`.

- `make fast` validates repository identity and runs shuffled tests.
- `make check` adds formatting, module/vendor consistency, vet, and shuffled tests.
- `make verify` adds lint, NilAway, gosec, govulncheck, race tests, coverage, and
  vendor-offline tests/builds, including the annotation tool smoke path and real
  pinned Spice compiler fixtures for alias acceptance and invalid result types.
  Presentation acceptance includes fixed light/dark goldens at normal, compact
  Unicode, and 1x1 boundary sizes; exact row/column and cursor-cell assertions;
  revision/stale-update, rolling-bound, history-navigation, resize-sequence,
  accessible-mode, concurrent-render, Ctrl-C, and cancellation tests.

The repository-owned verifier is cross-platform. `make fast`, `make check`, and
`make verify` force `GOPROXY=off`; missing cache entries fail instead of causing
hidden downloads. `make fmt` is the only target that rewrites Go source.

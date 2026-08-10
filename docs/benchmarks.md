# Runtime benchmarks

These deterministic, offline microbenchmarks provide provisional feedback for
the TUI's UI-neutral session, presentation, and virtual-terminal paths. They do
not create a child process or PTY, start a Bubble Tea event loop, contact a
daemon, or use the network.

The suite measures:

- a complete `SessionSnapshot` passing through `ScriptSession`, the public
  `tuittest.Driver`, model ingestion, and a fixed-size screen capture;
- pure public screen rendering;
- the pre-canceled `ScriptSession.Receive` hot path;
- one bounded VT output frame interpretation and cell/cursor capture;
- private Bubble Tea model update plus view rendering through the same
  deterministic hooks used by `tuittest`; and
- the fixed production renderer in isolation.

Run the complete suite through the repository-owned offline gate:

```text
make benchmark
```

The gate selects exactly the six adopted benchmarks, runs five 500-iteration
samples with `-cpu=1`, records allocations, forces `GOWORK=off`, and uses only
the committed vendor graph with the proxy and checksum database disabled.
Missing dependencies therefore fail instead of causing a hidden download.

## Provisional baseline

The current baseline is intentionally descriptive, not a performance gate.
Microbenchmark results vary with operating system scheduling, CPU power policy,
and Go runtime changes. Record medians from a clean five-sample run alongside
the exact Go version, platform, CPU, commit, and benchmark source hashes. Do not
convert these observations into thresholds until representative end-to-end TUI
profiles establish stable budgets.

Initial observation on Windows 11 amd64, AMD Ryzen 9 5900X, Go 1.26.5, from
base commit `0e6cfb1a58b8bb2cf711fd36dfa66a8cd4e9867f`:

| Benchmark | Median ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `SessionEventIngestionAndScreen` | 118,365 | 26,927 | 241 |
| `RenderScreen` | 34,935 | 14,896 | 90 |
| `ScriptSessionReceiveCanceled` | 33.80 | 0 | 0 |
| `VirtualTerminalFrameCapture` | 540,000 | 4,795,768 | 468 |
| `ModelSnapshotUpdateAndView` | 62,894 | 9,643 | 127 |
| `FixedRendererRender` | 25,887 | 6,063 | 65 |

The observation predates the repository-owned command and used the equivalent
five-sample benchmark selection. The virtual-terminal row was added from the
repository-owned five-sample, 500-iteration, CPU-1 command on the same machine
and Go version; its 527,802–561,765 ns/op range is intentionally descriptive.
Future observations use `make benchmark`.
The source identities were:

| File | SHA-256 |
| --- | --- |
| `session.go` | `d43146fdd640e04d41652a1b6108abf435b50e5625dea8eca8b324d9ae708658` |
| `tuittest/driver.go` | `f4fa6d9555c613f044d8a5c97166ba3832b1753b660bf599f8fd09a5a919d3e6` |
| `tuittest/session.go` | `cb1f7ecc905378843ed512ab3b82069c1905885f14930fbf0b468d76d85ef5e2` |
| `tuittest/benchmark_test.go` | `8812e3017766563f5c663e8f18514005978c509213d6f226b0b25d4cb4ea7fb7` |
| `internal/presentation/model.go` | `430d1e81e1d0b1b05ae7201a109fcb8fcbc5f904d2a15472adad1891c7df7f05` |
| `internal/presentation/renderer.go` | `eb21b11772f67d863eab429aff88a9a016a3b7f67fb57fd4f2df5ba352fe8306` |
| `internal/presentation/benchmark_test.go` | `1aff1fe21c5efd8af1986d5de484fd1a9b31df2d6aa5948455d927b460d73995` |

The virtual-terminal observation was produced from the following exact source
identities on top of base commit
`37ded601f90f047659cd680bffce00071031ff60`:

| File | SHA-256 |
| --- | --- |
| `tuittest/virtual_terminal.go` | `746e123fe9d2275255196241673754766d0b54c12aa8cc6ab8a35651defe719f` |
| `tuittest/virtual_terminal_options.go` | `d865b480bcc53158c95e9eb5d2ccd93e142513f52cfb7a147e25908d1fd1fe8b` |
| `tuittest/virtual_terminal_segment.go` | `320993bf4dadd3fb991ac3ed90b59a865e7fdf781aae61d6295a1513717e5193` |
| `tuittest/benchmark_test.go` | `54ca983e6f35e760cb0996755572e66e636e840903a11674c1724cd105f5d3b4` |

# Dependency review

The shell directly requires
`github.com/spice-framework/spice-agent-tui v0.1.0-preview.1` and imports only
that module's public root package. A separate conformance-only package requires
`github.com/spice-framework/spice-agent` at exact commit `b205307d...` and
imports its public client, gRPC wire, endpoint, and local-IPC packages. There is
no `replace`, tool directive, Bubble Tea, terminal plugin, database, telemetry,
or remote-network dependency.

The published root package selects this exact vendored graph:

| Module | Version | License | Reason in the public TUI package |
| --- | --- | --- | --- |
| `github.com/spice-framework/spice-agent-tui` | `v0.1.0-preview.1` | Apache-2.0 | Session and immutable semantic values |
| `github.com/spice-framework/spice-agent` | `v0.1.0-preview.5.0.20260810055539-b205307d3b5f` | Apache-2.0 | Conformance-only public protocol/client/local-IPC contracts |
| `github.com/Microsoft/go-winio` | `v0.6.2` | MIT | Windows named-pipe local IPC |
| `github.com/spice-framework/spice` | `v0.1.0-preview.2` | Apache-2.0 | Published starter/annotation metadata referenced by the root package |
| `github.com/charmbracelet/x/ansi` | `v0.11.7` | MIT | Bounded terminal-safe public text validation |
| `github.com/clipperhouse/displaywidth` | `v0.11.0` | MIT | Transitive Unicode display width |
| `github.com/clipperhouse/uax29/v2` | `v2.7.0` | MIT | Transitive grapheme segmentation |
| `github.com/lucasb-eyer/go-colorful` | `v1.4.0` | MIT | Transitive color value support |
| `github.com/mattn/go-runewidth` | `v0.0.23` | MIT | Transitive Unicode display width |
| `github.com/rivo/uniseg` | `v0.4.7` | MIT | Transitive grapheme segmentation |
| `golang.org/x/net` | `v0.57.0` | BSD-3-Clause | gRPC HTTP/2 transport internals |
| `golang.org/x/sys` | `v0.47.0` | BSD-3-Clause | OS-specific local IPC and transport support |
| `golang.org/x/text` | `v0.40.0` | BSD-3-Clause | gRPC transitive text processing |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20260526163538-3dc84a4a5aaa` | Apache-2.0 | gRPC status details |
| `google.golang.org/grpc` | `v1.83.0` | Apache-2.0 | Conformance-only Agent client/peer transport |
| `google.golang.org/protobuf` | `v1.36.11` | BSD-3-Clause | Engine protocol messages and validators |

This is a subset of the released TUI graph already reviewed in the repository
[`docs/dependency-review.md`](../../docs/dependency-review.md). The nested
`go.sum` verifies module content and the mechanically generated vendor tree
contains source and license files for offline builds. Root quality gates fail
when that tree differs byte-for-byte from `go mod vendor`.

The shell performs no dependency discovery or background update. Compatibility
tests start only the exact source-built test executable, use authenticated
current-user local IPC, bound child startup/shutdown, and assert endpoint
cleanup. They make no released-binary compatibility claim. Changes to
the published TUI version or any selected transitive version require a new
checksum, provenance, license, maintenance, cancellation, vulnerability, and
offline-vendor review.

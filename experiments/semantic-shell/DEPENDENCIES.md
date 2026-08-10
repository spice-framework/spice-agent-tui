# Dependency review

The experiment directly requires only
`github.com/spice-framework/spice-agent-tui v0.1.0-preview.1` and imports only
that module's public root package. It has no `replace`, tool, executable,
Bubble Tea, terminal-plugin, database, telemetry, process, or network
dependency.

The published root package selects this exact vendored graph:

| Module | Version | License | Reason in the public TUI package |
| --- | --- | --- | --- |
| `github.com/spice-framework/spice-agent-tui` | `v0.1.0-preview.1` | Apache-2.0 | Session and immutable semantic values |
| `github.com/spice-framework/spice` | `v0.1.0-preview.2` | Apache-2.0 | Published starter/annotation metadata referenced by the root package |
| `github.com/charmbracelet/x/ansi` | `v0.11.7` | MIT | Bounded terminal-safe public text validation |
| `github.com/clipperhouse/displaywidth` | `v0.11.0` | MIT | Transitive Unicode display width |
| `github.com/clipperhouse/uax29/v2` | `v2.7.0` | MIT | Transitive grapheme segmentation |
| `github.com/lucasb-eyer/go-colorful` | `v1.4.0` | MIT | Transitive color value support |
| `github.com/mattn/go-runewidth` | `v0.0.23` | MIT | Transitive Unicode display width |
| `github.com/rivo/uniseg` | `v0.4.7` | MIT | Transitive grapheme segmentation |

This is a subset of the released TUI graph already reviewed in the repository
[`docs/dependency-review.md`](../../docs/dependency-review.md). The nested
`go.sum` verifies module content and the mechanically generated vendor tree
contains source and license files for offline builds. Root quality gates fail
when that tree differs byte-for-byte from `go mod vendor`.

The shell performs no dependency discovery or background update. Changes to
the published TUI version or any selected transitive version require a new
checksum, provenance, license, maintenance, cancellation, vulnerability, and
offline-vendor review.

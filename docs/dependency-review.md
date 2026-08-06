# Dependency and security review

## Product graph

### Spice core v0.1.0-preview.1.0.20260806200749-524424a04df0

`github.com/spice-framework/spice` is pinned exactly at
v0.1.0-preview.1.0.20260806200749-524424a04df0 and is
Apache-2.0 licensed. This module uses its public annotation SDK v1alpha2,
framed protocol server, starter manifest, lifecycle provider contract metadata,
and Modulith declaration annotations. It does not import Spice compiler,
toolchain, CLI, generated transport, or internal packages.

The annotation SDK carries immutable typed contributions and bounded generic
function-result facts. TUI handlers use canonical identity, effective kind, and
named origin to validate exact interfaces while retaining alias support. The stdio
server receives caller-owned streams and cancellation and performs no network,
filesystem, logging, telemetry, discovery, or background update work.

The dependency is pre-1.0 and therefore intentionally exact. Upgrading requires
descriptor decode, protocol framing, starter compatibility, contribution wire,
vendor-offline, and full generated-compiler compatibility review.

### Spice toolchain v0.1.0-preview.1.0.20260806203056-d0b9ac086bd6

`github.com/spice-framework/toolchain` is selected through standard Go `tool`
directives for the Spice CLI and official core annotation tool. It supplies the
single `go/types` result-fact producer and real offline acceptance compiler. It
is a development/tool dependency only: public TUI packages and runtime code do
not import compiler, CLI, or internal toolchain packages.

The exact pin is intentionally coupled to the core result-facts revision.
`go.sum` and committed vendor contents provide integrity and offline operation;
ordinary verification never downloads or updates either module.

### Bubble Tea v2.0.8

The presentation package imports `charm.land/bubbletea/v2` at exactly v2.0.8.
That canonical Go module is maintained from the upstream Charmbracelet Bubble
Tea repository and is licensed MIT, which is compatible with this repository's
Apache-2.0 license. It provides the terminal event loop, Windows and Unix input,
renderer lifecycle, resize messages, and context-aware program cancellation.

Bubble Tea is confined to `internal/presentation`; public contracts do not expose
its types. The shell injects input/output and passes caller cancellation through
`tea.WithContext`. It enables no logging, telemetry, persistence, process launch,
or network access. The application still bounds all semantic data and terminal
dimensions before passing them to the renderer.

The canonical module path is important: `github.com/charmbracelet/bubbletea/v2`
is not an interchangeable import. The repository gate requires exactly
`charm.land/bubbletea/v2 v2.0.8`, rejects replacements, and `go.sum` plus the
committed vendor tree preserve source integrity and offline builds.

### Charmbracelet x/ansi v0.11.7

`github.com/charmbracelet/x/ansi` is pinned directly at v0.11.7 and is MIT
licensed. The deterministic renderer uses only its terminal display-width and
ANSI-aware truncation operations, which are required for correct Unicode cell
widths without splitting style sequences. It performs no I/O, logging,
telemetry, persistence, or network access.

### Rivo uniseg v0.4.7

`github.com/rivo/uniseg` is pinned directly at v0.4.7 and is MIT licensed. The
immutable prompt editor uses its Unicode grapheme segmentation so navigation,
deletion, and insertion never leave a cursor inside combining text, emoji ZWJ
sequences, regional indicators, or variation-selector clusters. Work remains
bounded by the 4 KiB prompt limit. The package performs no I/O, persistence,
telemetry, network access, or background work.

### Transitive terminal dependencies

Bubble Tea's selected graph includes terminal capability, input cancellation,
display width, color, synchronization, and OS syscall packages. Their exact
versions are recorded by `go.mod`, `go.sum`, and
`vendor/modules.txt`. Product code does not import these transitive packages.
Their platform files are exercised by Windows tests and Linux compile/test
coverage in the release workflow. Any change to the graph requires a fresh
license, maintenance, checksum, vulnerability, cancellation, and platform
review rather than an automatic version range update.

`govulncheck`, `gosec`, vet, race tests, module-tidy comparison, reproducible
vendor comparison, and vendor-only build/test are mandatory gates. Verification
runs analysis offline after the explicit, source-preserving
`make tools-bootstrap` target has populated the cache.

## Verification tools

The isolated `tools` module pins golangci-lint 2.12.2, gofumpt 0.10.0,
goimports/x-tools 0.48.0, gosec 2.28.0, govulncheck 1.1.4, and NilAway at
`f4f8ac24c032`. They are build-time-only dependencies. The explicit bootstrap
downloads the complete product and tools graphs through private alternate
module files, then every ordinary gate runs with `GOPROXY=off`, `GOWORK=off`,
and the selected exact Go 1.26.5 executable.

# Dependency and security review

## Product graph

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

### Transitive terminal dependencies

Bubble Tea's selected graph includes terminal capability, input cancellation,
Unicode segmentation/display width, color, synchronization, and OS syscall
packages. Their exact versions are recorded by `go.mod`, `go.sum`, and
`vendor/modules.txt`. Product code does not import these transitive packages.
Their platform files are exercised by Windows tests and Linux compile/test
coverage in the release workflow. Any change to the graph requires a fresh
license, maintenance, checksum, vulnerability, cancellation, and platform
review rather than an automatic version range update.

`govulncheck`, `gosec`, vet, race tests, module-tidy comparison, reproducible
vendor comparison, and vendor-only build/test are mandatory gates. Verification
runs analysis offline after explicit dependency preparation.

## Verification tools

The isolated `tools` module pins golangci-lint 2.12.2, gofumpt 0.10.0,
goimports/x-tools 0.48.0, gosec 2.28.0, govulncheck 1.1.4, and NilAway at
`f4f8ac24c032`. They are build-time-only dependencies. The quality gate prepares
them explicitly, then runs analysis with `GOPROXY=off`, `GOWORK=off`, and the
local Go 1.26.5 toolchain.

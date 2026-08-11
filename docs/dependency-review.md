# Dependency and security review

## Product graph

### Spice core v0.1.0-preview.4

`github.com/spice-framework/spice` is pinned exactly at
v0.1.0-preview.4 (module sum
`h1:jfUSUquq9rQN/FMI6zvBEJZYX12ZVeJAQmusvjy/3T8=`, go.mod sum
`h1:dBZV5UZcbY6pzhfGNtvAwQIJ8YsFna+jf1SAlmukJfk=`) and is Apache-2.0
licensed. This immutable public preview
replaces the development pseudo-version and is governed by Spice's protected
keyless release contract. This module uses its public annotation SDK v1alpha2,
framed protocol server, starter manifest, lifecycle provider contract metadata,
and Modulith declaration annotations. It does not import Spice compiler,
toolchain, CLI, generated transport, or internal packages.

The annotation SDK carries immutable typed contributions and bounded generic
function-result facts. TUI handlers use canonical identity, effective kind, and
named origin to validate exact interfaces while retaining alias support. The stdio
server receives caller-owned streams and cancellation and performs no network,
filesystem, logging, telemetry, discovery, or background update work.

The dependency remains pre-1.0 and therefore intentionally exact. Upgrading
requires descriptor decode, protocol framing, starter compatibility,
contribution wire, vendor-offline, and full generated-compiler compatibility
review.

### Spice toolchain v0.1.0-preview.4

`github.com/spice-framework/toolchain` is selected through standard Go `tool`
directives for the Spice CLI, official core annotation tool, and through the
isolated tools module for `spicestyle`. The exact public release has module sum
`h1:mpHAsOdPSUQTSa2GE891VJg5bXmzML0T2N9c5QU4yJg=` and go.mod sum
`h1:nezzFkAq9TDdavVL5sYJm2nOKNWAu1p9VTz3XFihgUg=`. It supplies the single
`go/types` result-fact producer, real offline acceptance compiler, and exact
schema-2 style analyzer. It is a development/tool dependency only: public TUI
packages and runtime code do not import compiler, CLI, or internal toolchain
packages.

The exact pin is intentionally coupled to the core result-facts revision.
`go.sum` and committed vendor contents provide integrity and offline operation;
ordinary verification never downloads or updates either module.

### Bubble Tea v2.0.8

The presentation package imports `charm.land/bubbletea/v2` at exactly v2.0.8.
That canonical Go module is maintained from the upstream Charmbracelet Bubble
Tea repository and is licensed MIT, which is compatible with this repository's
Apache-2.0 license. It provides the terminal event loop, Windows and Unix input,
renderer lifecycle, resize messages, and context-aware program cancellation.

Bubble Tea is confined to `internal/presentation`; root and `terminal` public
signatures do not expose its types. The shell injects input/output and passes
caller cancellation through `tea.WithContext`. The private Session adapter
maps validated UI-neutral updates to Bubble Tea messages, invokes each operation
once, and contains panics. It enables no logging, telemetry, persistence,
process launch, or network access. The application still bounds all semantic
data and terminal dimensions before passing them to the renderer.

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

### Charmbracelet x/vt 3755ebad01b1

`github.com/charmbracelet/x/vt` is pinned at the exact upstream commit
`3755ebad01b1366a9eeb5e4e80d664b404ab6eff` (module pseudo-version
`v0.0.0-20260803091719-3755ebad01b1`, sum
`h1:iS4IE3G9sjxjESAxhtIe2xxzrlLct83b7De0KlJDunE=`) and is MIT licensed. It is
maintained by Charmbracelet and reuses the already selected ANSI, Unicode, and
Ultraviolet terminal graph. The public `tuittest.VirtualTerminal` uses it only
as an in-memory, output-only terminal interpreter; no child process, PTY,
filesystem, environment, logging, telemetry, or network operation is added.

Output and control-string memory are bounded by the harness transcript limit
and upstream parser bounds. Spice wraps all access with one ownership mutex,
captures defensive immutable values, rejects overflow before interpretation,
and contains caller predicate panics. The dependency is pre-1.0, so any update
requires exact cell/cursor/alternate-screen/resize, chunking, fuzz, race,
license, checksum, and offline-vendor review. Native PTY/ConPTY ownership
remains outside this dependency and outside the TUI library runtime.

### CrossPTY v1.1.0

`github.com/Kodecable/crosspty` is pinned exactly at v1.1.0 (upstream tag
commit `0dbd5253c95baefd8c3e53cb0be1d44200884448`, sum
`h1:DlhZUGRB5kWJv20CLyol1ye/s5rOOXy0qNsgdcmKBFY=`) and is MIT licensed. It is
used only by the repository's native terminal acceptance package: no public or
production TUI package imports it. It supplies classic Unix PTYs and Windows
ConPTY plus bounded process-group/Job Object cleanup. The fixture launches only
its exact current Go test executable with a fixed helper selector, no shell,
network, downloaded executable, or user command. The child receives only a
fixed allowlist of locale, temporary-directory, and home-directory variables;
API keys and all other application environment variables are excluded and a
secret-canary regression locks that boundary.

The parent owns a 30-second test context, a 5-second cleanup budget, exact
80x24→100x30 resize, bounded output capture, and an explicit quit handshake
before process exit. This handshake also avoids the documented Darwin kernel
boundary where unread output from a very short-lived PTY process can be
discarded during teardown. Any update requires real Windows, Linux, and macOS
TTY identity, alternate-screen, cursor, Unicode/ANSI, resize, input, exit,
descendant-cleanup, race, checksum, license, vulnerability, and offline-vendor
evidence. CrossPTY is process-test infrastructure, not a sandbox or production
launcher.

### Rivo uniseg v0.4.7

`github.com/rivo/uniseg` is pinned directly at v0.4.7 and is MIT licensed. The
immutable prompt editor uses its Unicode grapheme segmentation so navigation,
deletion, and insertion never leave a cursor inside combining text, emoji ZWJ
sequences, regional indicators, or variation-selector clusters. Work remains
bounded by the 4 KiB prompt limit. The package performs no I/O, persistence,
telemetry, network access, or background work.

### Go x/image v0.39.0 and x/text v0.36.0

`golang.org/x/image` is pinned directly at v0.39.0 and brings
`golang.org/x/text` v0.36.0 transitively. Both are maintained by the Go project
and use the Go BSD-3-Clause license. Only the `tuittest` human visual-QA helper
imports x/image: it parses the package's embedded Go Mono TTF, performs
in-memory glyph drawing, and encodes PNG bytes. x/text supplies the pinned
font/parser graph. Neither dependency adds filesystem discovery, process
launch, logging, telemetry, persistence, network access, or background work.
The v0.39.0 minimum includes the upstream fix for GO-2026-4962 in malicious
SFNT decoding; the initially evaluated v0.25.0 is intentionally rejected by
the repository vulnerability gate.

The renderer accepts only an already bounded immutable `Screen`, caps scale at
2, uses the product's existing terminal dimension bounds, adds no PNG metadata,
and substitutes a deterministic outlined cell for glyphs absent from Go Mono.
Its output is explicitly non-authoritative; Screen/VT evidence and the native
PTY/ConPTY suite remain release gates. Any upgrade requires font-byte and PNG
digest review, Unicode fallback, concurrency/race, license, vulnerability,
offline-vendor, and Linux/Windows determinism evidence.

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

## Semantic-shell experiment graph

The removable Phase 7 module at `experiments/semantic-shell` directly pins the
published `spice-agent-tui v0.1.0-preview.1` module with no replacement. It
imports only the public root Session/value package in its shell. A removable
conformance package additionally pins Agent commit `b205307d...` and its public
gRPC/local-IPC client contracts; this selects gRPC, Protobuf, go-winio, and the
reviewed `x/net`, `x/sys`, and `x/text` transitive graph. It still does not
select or import Bubble Tea. Its checksum file, vendor tree, compatibility
manifest, and [dependency review](../experiments/semantic-shell/DEPENDENCIES.md)
are verified from the repository root. Product execution performs no runtime
network or dependency discovery; compatibility tests use only private local
IPC.

## Verification tools

The isolated `tools` module pins golangci-lint 2.12.2, gofumpt 0.10.0,
goimports/x-tools 0.48.0, gosec 2.28.0, govulncheck 1.1.4, and NilAway at
`f4f8ac24c032`, plus Toolchain preview4 `spicestyle`. They are build-time-only
dependencies. The style tool is restricted to the reviewed schema-2
composition boundary and runs without network access. The explicit bootstrap
downloads the complete product and tools graphs through private alternate
module files, then every ordinary gate runs with `GOPROXY=off`, `GOWORK=off`,
and the selected exact Go 1.26.5 executable.

The CI-only `actions/upload-artifact` action is pinned to v7.0.1 commit
`043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` and is MIT licensed. It uploads
only repository-generated synthetic lifecycle PNGs and their deterministic
manifest, receives no secrets, errors when output is absent, and retains the
artifact for 14 days. The repository identity gate rejects a floating or
changed action reference.

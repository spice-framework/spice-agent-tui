# Dependency and security review

## Product graph

The repository foundation uses only the Go standard library and exports no
runtime behavior. Bubble Tea v2 is the accepted shell architecture but is
deliberately not present: adding it before real presentation code would create
maintenance and supply-chain cost without product value.

Any future product dependency must document:

- maintained version and release cadence;
- license compatibility with Apache-2.0;
- checksum and vulnerability status;
- context cancellation and bounded resource behavior;
- logging, telemetry, and secret-redaction implications; and
- Windows, Linux, terminal, and accessibility behavior.

## Verification tools

The isolated `tools` module pins golangci-lint 2.12.2, gofumpt 0.10.0,
goimports/x-tools 0.48.0, gosec 2.28.0, govulncheck 1.1.4, and NilAway at
`f4f8ac24c032`. They are build-time-only dependencies. The quality gate prepares
them explicitly, then runs analysis with `GOPROXY=off`, `GOWORK=off`, and the
local Go 1.26.5 toolchain.

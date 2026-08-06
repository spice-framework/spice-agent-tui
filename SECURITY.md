# Security policy

Report vulnerabilities privately through GitHub Security Advisories for this
repository. Do not open a public issue containing exploit details, credentials,
prompts, transcripts, or user data.

The terminal client will treat daemon and model output as untrusted text. It
must not execute rendered content, interpolate it into a shell, or persist it
without an explicit user-owned destination. Authentication material must be
instance-owned, redacted, and excluded from logs and crash reports.

Phase 0 has no runtime or third-party product dependencies. Build tools are
pinned in `tools/go.mod`, execute only during local verification, and receive
the same vulnerability and checksum review as product dependencies.

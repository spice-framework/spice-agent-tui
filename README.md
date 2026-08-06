# Spice Agent TUI

This repository will provide the standalone terminal client for Spice Agent.
It is currently a Phase 0 foundation: repository ownership, governance,
compatibility metadata, and the complete Go quality gate are real; a protocol,
session API, command, and terminal implementation are intentionally absent.

This prevents a placeholder client from becoming an accidental compatibility
contract. See `ARCHITECTURE.md` and `ROADMAP.md` for the adoption sequence.

Go 1.26.5 is exact. Use `make fast` while editing, `make check` for the broader
loop, and `make verify` before every commit.


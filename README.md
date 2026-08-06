# Spice Agent TUI

This repository will provide the standalone terminal client for Spice Agent.
It currently contains the repository foundation: ownership, governance,
compatibility metadata, and the complete Go quality gate are real; the adopted
client/session API, UI-neutral values, command, and terminal implementation are
intentionally absent.

This prevents a placeholder client from becoming an accidental compatibility
contract. The finished product will own the Bubble Tea v2 shell, renderers,
editor, commands, and keybindings. It will consume only a high-level
client/session API and UI-neutral values, never the agent kernel, generated gRPC,
daemon supervision, or OS IPC. See `ARCHITECTURE.md` and `ROADMAP.md` for the
adoption sequence.

Go 1.26.5 is exact. Use `make fast` while editing, `make check` for the broader
loop, and `make verify` before every commit.

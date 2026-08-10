# TUI testing framework (`tuittest`)

`github.com/spice-framework/spice-agent-tui/tuittest` is the agent-friendly
harness for pixel-perfect Spice Agent terminal verification.

It drives the **real presentation model and FixedRenderer** and includes a
bounded modern virtual terminal for output conformance, without making the
public package own:

- a child process, PTY, or ConPTY;
- Bubble Tea's async event loop;
- daemon discovery, gRPC, or network I/O.

## Why this exists

Coding agents cannot reliably “see” an interactive TUI. This package makes the
UI:

1. **scriptable** (keys, typing, session updates);
2. **inspectable** (`Screen.AgentReport()` dumps plain + styled + semantics);
3. **pixel-comparable** (fixed-size frames, golden files, cursor metadata);
4. **terminal-verifiable** (interpreted VT cells, Unicode width, cursor,
   alternate screen, resize, bounded raw transcript, and event-driven waits).

## Quick start

```go
package mytui_test

import (
    "testing"

    "github.com/spice-framework/spice-agent-tui/tuittest"
)

func TestPromptChrome(t *testing.T) {
    driver, err := tuittest.NewDriver(tuittest.Options{
        Width:  48,
        Height: 12,
    })
    if err != nil {
        t.Fatal(err)
    }
    defer driver.Close()
    if err := driver.Type("list owners"); err != nil {
        t.Fatal(err)
    }
    screen, err := driver.Snapshot("list-owners")
    if err != nil {
        t.Fatal(err)
    }

    // Agent-readable dump for logs / CI artifacts
    t.Log("\n" + screen.AgentReport())

    // Pixel-perfect goldens (styled + plain)
    screen.AssertGolden(t, "testdata", "list-owners")
}
```

Refresh goldens:

```text
UPDATE_GOLDEN=1 go test ./tuittest -run TestPromptChrome
```

## Driver API

| Method | Purpose |
| --- | --- |
| `NewDriver(Options)` | Build deterministic interactive harness |
| `Close()` | Cancel in-flight Session work; safe to call repeatedly |
| `Type(text)` | Insert prompt text |
| `Key(stroke[, text])` | Press `enter`, `esc`, `left`, `ctrl+c`, … |
| `Action(action)` | Semantic action via standard bindings |
| `InjectUpdate(update)` | Apply a `SessionUpdate` as if received |
| `Resize(w,h)` | Change canvas |
| `Screen(name)` / `Snapshot(name)` | Capture frame |
| `RunScenario(steps...)` | Ordered script with named snapshots |

### Useful keystrokes

`enter`, `esc`, `backspace`, `left`, `right`, `up`, `down`, `home`, `end`,
`ctrl+c`, `ctrl+q`, `ctrl+x`, `ctrl+a`, `ctrl+e`, `alt+enter`, `text`.

## Scripted sessions

```go
session := tuittest.NewScriptSession()
_ = session.SetPerformResult(result)
driver, _ := tuittest.NewDriver(tuittest.Options{Session: session})
defer driver.Close()

_ = driver.InjectUpdate(snapshotUpdate)
_ = driver.Type("hello")
_ = driver.Key("enter")
// session.Intents() contains IntentSubmit{"hello"}
```

`Driver` deliberately does not arm the model's blocking Receive loop. Use
`InjectUpdate` for deterministic driver state. `ScriptSession.PushUpdate` is a
real cancellation-aware Receive queue for facade/shell tests that run Bubble
Tea, or for direct Session contract tests. This separation prevents abandoned
receive goroutines and lost updates in a synchronous test driver.

Perform commands are bounded by `Options.DrainTimeout`. A timeout is returned,
the driver closes, and its lifecycle context cancels the in-flight Session call;
timeouts are never reported as successful interaction.

## Pure renderer captures

When you only need chrome/layout without keyboard state:

```go
screen, err := tuittest.RenderScreen(viewData, tuittest.RenderOptions{
    Width:  48,
    Height: 10,
    Name:   "disconnected",
})
```

## Virtual-terminal conformance

Use `VirtualTerminal` when the subject is raw output from a real Bubble Tea
program or another terminal producer rather than a direct semantic frame:

```go
terminal, err := tuittest.NewVirtualTerminal(tuittest.VirtualTerminalOptions{
    Width: 80,
    Height: 24,
})
if err != nil {
    t.Fatal(err)
}
defer terminal.Close()

_, _ = terminal.WriteString("\x1b[?1049h\x1b[2J\x1b[Hready")
screen, err := terminal.WaitFor(t.Context(), "ready", func(screen tuittest.Screen) bool {
    return screen.Contains("ready")
})
```

Writes are atomic against a configured transcript limit. Captures expose the
same `Screen` contract as the semantic driver, including exact dimensions,
plain cells, normalized styling, cursor coordinates/visibility, and
alternate-screen state. `WaitFor` is notification-driven and has no polling or
sleep interval; caller context owns its deadline. Predicate panics become a
fixed diagnostic without reflecting panic text.

The virtual terminal deliberately does not launch a process. The repository's
separate `internal/nativeacceptance` suite launches its own exact test binary
under a real Unix PTY or Windows ConPTY, feeds output into `VirtualTerminal`,
and proves TTY identity, alternate-screen/cursor control, input, Unicode/ANSI
rendering, 80x24→100x30 resize propagation, bounded capture, clean exit, and
process-group/Job Object cleanup on every hosted platform. This keeps arbitrary
process launch out of the public library while testing the same composition an
application uses with its own trusted process fixture. Released-distribution
acceptance remains responsible for the actual shipped terminal executable.

## Screen inspection for agents

```go
screen.Plain()          // fixed-size plain text grid
screen.Styled()         // ANSI with <ESC> tokens for goldens
screen.Lines()          // plain lines
screen.Contains("...")  // substring check
screen.AlternateScreen() // virtual-terminal alternate-buffer state
screen.Prompt()         // editor value
screen.StatusLevel()    // ready/error/...
screen.Activity()       // activity strings
screen.AgentReport()    // multi-section dump
screen.Diff(other)      // mismatch explanation
```

### Pixel contract

For normal (non-accessible) mode:

- height equals requested canvas height;
- every plain line has display width equal to canvas width;
- styled golden comparison uses the same normalization as presentation goldens
  (`ESC` → `<ESC>`, right-trim decorative padding spaces).
- the report golden is required and verifies cursor, dimensions, semantic state,
  accessibility mode, alternate-screen policy, and revision metadata.

Normal comparison never creates missing fixtures. Set `UPDATE_GOLDEN=1`
explicitly to create or replace all three artifacts.

Accessible mode emits a dense semantic transcript (no fixed padding, no ANSI).

## Scenario helpers

```go
err := driver.RunScenario(
    tuittest.UpdateStep(readyUpdate),
    tuittest.SnapshotStep("ready"),
    tuittest.TypeStep("show orders"),
    tuittest.KeyStep("enter"),
    tuittest.SnapshotStep("submitted"),
)
```

## Design boundaries

| In scope | Out of scope |
| --- | --- |
| Presentation model + FixedRenderer | Daemon / gRPC / tools |
| Scripted Session SPI | Real OpenAI / OpenRouter |
| Golden styled/plain frames | PTY screenshot OCR |
| VT output/cursor/alternate-screen/resize | Arbitrary process launch in public API |
| Repository-owned Unix PTY/Windows ConPTY acceptance | Production process ownership |
| Agent text dumps | Visual font rasterization |

This package imports `internal/presentation` (same module). External modules
should depend only on `tuittest` and public `agenttui` contracts.

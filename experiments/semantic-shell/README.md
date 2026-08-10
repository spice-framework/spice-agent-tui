# Semantic shell experiment

This nested module proves that the released, UI-neutral Spice Agent TUI
contracts can support a second client without importing Bubble Tea or a
terminal plugin. It is deliberately experimental and is not distributed as a
command.

The shell consumes line-oriented commands and emits one deterministic JSON
Lines record for every accepted semantic view, command result, or fixed error.
It depends only on the Go standard library and
`github.com/spice-framework/spice-agent-tui v0.1.0-preview.1`.

The removable conformance test package separately pins Agent commit
`b205307d3b5fb262401c77d1af902b1ce926d49a`. On Linux and Windows it
source-builds exact legacy-1.2 and current-1.3 peers, connects over private
authenticated local IPC, and drives this unchanged JSONL shell. The adapter is
not imported by the shell package and is not a production transport.

## Commands

| Input | Public intent |
| --- | --- |
| `submit <text>` | `IntentSubmit` |
| `respond <text>` | `IntentRespond` |
| `cancel` | `IntentCancelActiveRun` |
| `quit` | Stops this shell only |

One goroutine owns `Session.Receive`. Ordinary commands use one serial perform
lane. Cancellation has a separate serial perform lane so a blocked submission
cannot prevent cancellation. An operation is attempted once and is never
retried. Every output record has the schema
`spice.agent.semantic-shell/v1alpha1`, a strictly increasing sequence, and a
bounded payload. Semantic view revisions must also increase strictly.

The shell owns neither a daemon nor a network connection. Its caller supplies
the `Session`, input, and output and retains transport ownership. See
[`SECURITY.md`](SECURITY.md) for the threat boundary and deletion plan.

The compatibility matrix is source-to-source evidence. It invokes no prior
released binary and therefore makes no N/N-1 binary compatibility claim.

## Verification

Run with Go 1.26.5:

```text
make fast
make check
make race
make verify
make benchmark
```

The Makefile always uses the committed vendor tree with workspace and network
resolution disabled. The repository root gates run the same experiment so a
clean hosted cache is sufficient on the first push.

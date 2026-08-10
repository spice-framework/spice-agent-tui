# Security and deletion boundary

This experiment accepts already-authorized semantic values. It is not a
permission system, transport, sandbox, daemon, terminal emulator, or secret
store.

- Input lines, prompts, queues, records, and writes are bounded.
- Invalid commands, session errors, output errors, and recovered panics emit
  fixed messages. Underlying error and panic text is never serialized.
- Receive has one owner. Ordinary actions and cancellation have independent
  bounded serial lanes. Cancellation wins through the caller context and no
  action is replayed.
- State is copied into portable strings and slices. It contains no executable
  UI code, transport handle, or plugin object.
- The shell performs no network access and discovers no ambient service.

Command text and semantic state are intentionally visible in JSONL output; the
caller must treat that stream according to the sensitivity of its session.
This experiment does not claim to redact user-authored prompts or model output.

If the public seam does not survive the stress prototype, delete this entire
module and remove its root quality and documentation references. No root
product API, release metadata, generated output, or runtime migration is
required.

package tuittest_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

func TestLifecycleTraceIsCanonicalDeterministicAndGolden(t *testing.T) {
	trace := lifecycleTrace(t, false)
	path := "testdata/lifecycle.trace.json"
	if tuittest.UpdateGolden() {
		if err := os.WriteFile(path, trace.CanonicalJSON(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := tuittest.ParseTrace(content)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed.CanonicalJSON(), trace.CanonicalJSON()) || parsed.Digest() != trace.Digest() {
		t.Fatal("parsed lifecycle trace differs from constructed reference")
	}
	replay, err := parsed.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if replay.TraceDigest() != parsed.Digest() || len(replay.Steps()) != len(parsed.Events()) {
		t.Fatalf("replay evidence = digest %q, steps %d", replay.TraceDigest(), len(replay.Steps()))
	}
	for _, name := range []string{"lifecycle-ready", "lifecycle-typed", "lifecycle-streaming", "lifecycle-history"} {
		screen, exists := replay.Screen(name)
		if !exists {
			t.Fatalf("replay is missing snapshot %q", name)
		}
		if err := screen.CompareGolden("testdata", name); err != nil {
			t.Fatal(err)
		}
	}
	intents := replay.Intents()
	if len(intents) != 1 || intents[0].Kind() != agenttui.IntentSubmit ||
		intents[0].Values()[0].String() != "show orders" {
		t.Fatalf("performed intents = %#v", intents)
	}
	for index, step := range replay.Steps() {
		if len(step.Digest()) != 64 || step.Digest() != step.Screen().Digest() || step.Index() != index {
			t.Fatalf("invalid per-step evidence at %d: %#v", index, step)
		}
	}
}

func TestTraceReplayReferenceInvariantsCoverAccessibleMode(t *testing.T) {
	t.Parallel()
	trace := lifecycleTrace(t, true)
	replay, err := trace.Replay()
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range replay.Steps() {
		screen := step.Screen()
		if !screen.Accessible() || screen.AlternateScreen() || strings.Contains(screen.Styled(), "<ESC>") {
			t.Fatalf("accessible invariant failed at %d\n%s", step.Index(), screen.AgentReport())
		}
	}
}

func TestParseTraceRejectsNoncanonicalAndUnknownJSON(t *testing.T) {
	t.Parallel()
	canonical := lifecycleTrace(t, false).CanonicalJSON()
	tests := map[string][]byte{
		"missing final LF": bytes.TrimSuffix(canonical, []byte{'\n'}),
		"leading space":    append([]byte{' '}, canonical...),
		"unknown field": bytes.Replace(
			canonical, []byte(`"accessible":false`), []byte(`"accessible":false,"unknown":true`), 1,
		),
		"duplicate field": bytes.Replace(
			canonical, []byte(`"width":48`), []byte(`"width":48,"width":48`), 1,
		),
		"extra union field": bytes.Replace(
			canonical, []byte(`{"type":"type","text":"show orders"}`),
			[]byte(`{"type":"type","text":"show orders","name":"bad"}`), 1,
		),
		"trailing value": append(append([]byte(nil), canonical...), []byte("{}\n")...),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := tuittest.ParseTrace(content); err == nil {
				t.Fatal("expected strict parse error")
			}
		})
	}
}

func TestTraceValidationRejectsInvalidOrderingAndPayloads(t *testing.T) {
	t.Parallel()
	first := mustUpdateEvent(t, lifecycleSnapshot(t, 2, agenttui.StatusReady, "ready", nil, nil))
	stale := mustUpdateEvent(t, lifecycleSnapshot(t, 1, agenttui.StatusReady, "stale", nil, nil))
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, []tuittest.TraceEvent{first, stale}); err == nil ||
		!strings.Contains(err.Error(), "not greater") {
		t.Fatalf("revision ordering error = %v", err)
	}
	if _, err := tuittest.NewTypeEvent(""); err == nil {
		t.Fatal("expected empty type event error")
	}
	if _, err := tuittest.NewResizeEvent(0, 1); err == nil {
		t.Fatal("expected invalid resize error")
	}
	if _, err := tuittest.NewSnapshotEvent("../escape"); err == nil {
		t.Fatal("expected unsafe snapshot name error")
	}
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, nil); err == nil {
		t.Fatal("expected empty trace error")
	}
}

func TestTraceRoundTripCoversEveryTaggedEventVariant(t *testing.T) {
	t.Parallel()
	initial := lifecycleSnapshot(t, 1, agenttui.StatusReady, "ready", []string{"connected"}, nil)
	activity, err := agenttui.NewActivityUpdate(2, mustText(t, "indexed owners"))
	if err != nil {
		t.Fatal(err)
	}
	history, err := agenttui.NewPromptHistoryUpdate(3, []agenttui.Text{mustText(t, "owners")})
	if err != nil {
		t.Fatal(err)
	}
	action, err := tuittest.NewActionEvent(agenttui.ActionCursorStart)
	if err != nil {
		t.Fatal(err)
	}
	printable, err := tuittest.NewKeyEvent("text", "!")
	if err != nil {
		t.Fatal(err)
	}
	events := []tuittest.TraceEvent{
		mustUpdateEvent(t, initial),
		mustUpdateEvent(t, activity),
		mustUpdateEvent(t, history),
		mustTypeEvent(t, "owners"),
		action,
		printable,
		mustResizeEvent(t, 44, 11),
		mustSnapshotEvent(t, "tagged-union"),
	}
	trace, err := tuittest.NewTrace(tuittest.TraceOptions{
		Width: 36, Height: 9, ThemeMode: agenttui.ThemeLight,
	}, events)
	if err != nil {
		t.Fatal(err)
	}
	if options := trace.Options(); options.Width != 36 || options.Height != 9 ||
		options.ThemeMode != agenttui.ThemeLight {
		t.Fatalf("trace options = %#v", options)
	}
	parsed, err := tuittest.ParseTrace(trace.CanonicalJSON())
	if err != nil {
		t.Fatal(err)
	}
	replay, err := parsed.Replay()
	if err != nil {
		t.Fatal(err)
	}
	for index, event := range parsed.Events() {
		if event.Kind() != replay.Steps()[index].Kind() {
			t.Fatalf("step %d kind = %q, want %q", index, replay.Steps()[index].Kind(), event.Kind())
		}
	}
	if screen, exists := replay.Screen("tagged-union"); !exists || screen.Prompt() != "!owners" ||
		screen.Revision() != 3 || screen.Width() != 44 || screen.Height() != 11 {
		t.Fatalf("tagged-union screen = %#v, exists %t", screen, exists)
	}
	if _, exists := replay.Screen("missing"); exists {
		t.Fatal("unexpected missing snapshot")
	}
	returned := parsed.Events()
	returned[0] = tuittest.TraceEvent{}
	if parsed.Events()[0].Kind() != tuittest.TraceEventUpdate {
		t.Fatal("Events did not return a defensive copy")
	}
}

func TestTraceValidationRejectsUnexecutableEventsAndBounds(t *testing.T) {
	t.Parallel()
	if _, err := tuittest.NewActionEvent(agenttui.Action("unknown")); err == nil {
		t.Fatal("expected unsupported action error")
	}
	if _, err := tuittest.NewKeyEvent("enter", "one", "two"); err == nil {
		t.Fatal("expected extra key text error")
	}
	if _, err := tuittest.NewUpdateEvent(agenttui.SessionUpdate{}); err == nil {
		t.Fatal("expected empty update error")
	}
	if _, err := tuittest.NewTypeEvent(strings.Repeat("x", agenttui.MaximumPromptBytes+1)); err == nil {
		t.Fatal("expected bounded type error")
	}
	nested, err := agenttui.NewIntent(agenttui.IntentSubmit, []agenttui.Text{mustText(t, "nested")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := agenttui.NewCommandResult(mustText(t, "bad"), &nested)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tuittest.NewPerformResultEvent(result); err == nil {
		t.Fatal("expected nested result error")
	}
	typed := mustTypeEvent(t, "submit")
	enter := mustKeyEvent(t, "enter")
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, []tuittest.TraceEvent{typed, enter}); err == nil ||
		!strings.Contains(err.Error(), "perform-result") {
		t.Fatalf("unconfigured effect error = %v", err)
	}
	snapshot := mustSnapshotEvent(t, "duplicate")
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, []tuittest.TraceEvent{snapshot, snapshot}); err == nil ||
		!strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate snapshot error = %v", err)
	}
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{Width: agenttui.MaximumWidth + 1}, []tuittest.TraceEvent{typed}); err == nil {
		t.Fatal("expected options size error")
	}
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{ThemeMode: agenttui.ThemeMode("sepia")}, []tuittest.TraceEvent{typed}); err == nil {
		t.Fatal("expected options theme error")
	}
	tooMany := make([]tuittest.TraceEvent, 257)
	for index := range tooMany {
		tooMany[index] = typed
	}
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, tooMany); err == nil ||
		!strings.Contains(err.Error(), "exceed") {
		t.Fatalf("event bound error = %v", err)
	}
	if _, err := tuittest.NewTrace(tuittest.TraceOptions{}, []tuittest.TraceEvent{{}}); err == nil {
		t.Fatal("expected zero event error")
	}
}

func lifecycleTrace(t *testing.T, accessible bool) tuittest.Trace {
	t.Helper()
	ready := lifecycleSnapshot(
		t, 1, agenttui.StatusReady, "ready for prompts",
		[]string{"connected"}, []string{"previous query"},
	)
	busy := lifecycleSnapshot(
		t, 2, agenttui.StatusBusy, "loading orders",
		[]string{"connected", "loading orders"}, []string{"previous query", "show orders"},
	)
	complete := lifecycleSnapshot(
		t, 3, agenttui.StatusReady, "orders loaded",
		[]string{"connected", "orders loaded"}, []string{"previous query", "show orders"},
	)
	result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
	if err != nil {
		t.Fatal(err)
	}
	events := []tuittest.TraceEvent{
		mustUpdateEvent(t, ready),
		mustSnapshotEvent(t, "lifecycle-ready"),
		mustTypeEvent(t, "show orders"),
		mustSnapshotEvent(t, "lifecycle-typed"),
		mustPerformResultEvent(t, result),
		mustKeyEvent(t, "enter"),
		mustUpdateEvent(t, busy),
		mustResizeEvent(t, 52, 13),
		mustSnapshotEvent(t, "lifecycle-streaming"),
		mustUpdateEvent(t, complete),
		mustKeyEvent(t, "up"),
		mustSnapshotEvent(t, "lifecycle-history"),
	}
	trace, err := tuittest.NewTrace(tuittest.TraceOptions{
		Width: 48, Height: 12, Accessible: accessible, ThemeMode: agenttui.ThemeDark,
	}, events)
	if err != nil {
		t.Fatal(err)
	}
	return trace
}

func lifecycleSnapshot(
	t *testing.T,
	revision uint64,
	level agenttui.StatusLevel,
	message string,
	activityValues, historyValues []string,
) agenttui.SessionUpdate {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(mustText(t, "Commerce"), []agenttui.Section{
		mustSection(t, "Module", "orders\npayments"),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(level, mustText(t, message), []agenttui.Text{mustText(t, "enter submit")})
	if err != nil {
		t.Fatal(err)
	}
	activity := traceTexts(t, activityValues)
	history := traceTexts(t, historyValues)
	snapshot, err := agenttui.NewSessionSnapshot(revision, workspace, status, activity, history)
	if err != nil {
		t.Fatal(err)
	}
	update, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func traceTexts(t *testing.T, values []string) []agenttui.Text {
	t.Helper()
	result := make([]agenttui.Text, 0, len(values))
	for _, value := range values {
		result = append(result, mustText(t, value))
	}
	return result
}

func mustTypeEvent(t *testing.T, text string) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewTypeEvent(text)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustKeyEvent(t *testing.T, stroke string) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewKeyEvent(stroke)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustUpdateEvent(t *testing.T, update agenttui.SessionUpdate) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewUpdateEvent(update)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustResizeEvent(t *testing.T, width, height int) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewResizeEvent(width, height)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustSnapshotEvent(t *testing.T, name string) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewSnapshotEvent(name)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustPerformResultEvent(t *testing.T, result agenttui.CommandResult) tuittest.TraceEvent {
	t.Helper()
	event, err := tuittest.NewPerformResultEvent(result)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

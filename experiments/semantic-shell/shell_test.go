package semanticshell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

const testTimeout = 3 * time.Second

func TestShellProjectsPortableStateAndCommands(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	session.updates <- mustSnapshotUpdate(t, 1)
	session.perform = func(_ context.Context, intent agenttui.Intent) (agenttui.CommandResult, error) {
		message := mustText(t, "accepted "+string(intent.Kind()))
		return agenttui.NewCommandResult(message, nil)
	}
	harness := startShell(t, session)

	view := harness.nextRecord(t)
	if view.Type != "view" || view.Sequence != 1 || view.Revision != 1 || view.View == nil ||
		view.View.Workspace.Title != "workspace" || view.View.Status.Level != "ready" ||
		len(view.View.Activity) != 1 || view.View.Activity[0] != "ready" {
		t.Fatalf("initial record = %#v", view)
	}
	for _, command := range []string{"submit hello", "respond approved", "cancel"} {
		harness.write(t, command+"\n")
		result := harness.nextRecord(t)
		if result.Type != "result" || result.Command != strings.Fields(command)[0] ||
			result.Operation == 0 || result.Message == "" || result.View != nil {
			t.Fatalf("%s result = %#v", command, result)
		}
	}
	harness.write(t, "quit\n")
	quit := harness.nextRecord(t)
	if quit.Type != "result" || quit.Command != "quit" || quit.Message != "semantic shell stopped" {
		t.Fatalf("quit record = %#v", quit)
	}
	if err := harness.wait(t); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	harness.assertSequences(t)
}

func TestCancelLaneRemainsAvailableWhileSubmitIsBlocked(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	session.updates <- mustSnapshotUpdate(t, 1)
	submitStarted := make(chan struct{})
	cancelStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	var ordinaryActive atomic.Int32
	var cancelActive atomic.Int32
	var maximumTotal atomic.Int32
	session.perform = func(ctx context.Context, intent agenttui.Intent) (agenttui.CommandResult, error) {
		switch intent.Kind() {
		case agenttui.IntentSubmit:
			active := ordinaryActive.Add(1) + cancelActive.Load()
			maximumTotal.Store(max(maximumTotal.Load(), active))
			defer ordinaryActive.Add(-1)
			close(submitStarted)
			select {
			case <-releaseSubmit:
			case <-ctx.Done():
				return agenttui.CommandResult{}, ctx.Err()
			}
		case agenttui.IntentCancelActiveRun:
			active := cancelActive.Add(1) + ordinaryActive.Load()
			maximumTotal.Store(max(maximumTotal.Load(), active))
			defer cancelActive.Add(-1)
			close(cancelStarted)
			close(releaseSubmit)
		}
		return agenttui.NewCommandResult(mustText(t, "done"), nil)
	}
	harness := startShell(t, session)
	_ = harness.nextRecord(t)
	harness.write(t, "submit blocked\n")
	waitSignal(t, submitStarted, "submit start")
	harness.write(t, "cancel\n")
	waitSignal(t, cancelStarted, "cancel start")

	seen := map[string]bool{}
	for range 2 {
		result := harness.nextRecord(t)
		if result.Type != "result" {
			t.Fatalf("operation record = %#v", result)
		}
		seen[result.Command] = true
	}
	if !seen["submit"] || !seen["cancel"] || maximumTotal.Load() != 2 {
		t.Fatalf("lane proof = seen %v maximum %d", seen, maximumTotal.Load())
	}
	harness.write(t, "quit\n")
	_ = harness.nextRecord(t)
	if err := harness.wait(t); err != nil {
		t.Fatal(err)
	}
}

func TestShellRejectsInvalidAndStaleUpdates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		updates []agenttui.SessionUpdate
	}{
		{name: "zero", updates: []agenttui.SessionUpdate{{}}},
		{name: "activity before snapshot", updates: []agenttui.SessionUpdate{mustActivityUpdate(t, 1, "early")}},
		{name: "stale", updates: []agenttui.SessionUpdate{mustSnapshotUpdate(t, 2), mustActivityUpdate(t, 2, "stale")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := newTestSession()
			for _, update := range test.updates {
				session.updates <- update
			}
			harness := startShell(t, session)
			var last wireRecord
			for range len(test.updates) {
				last = harness.nextRecord(t)
				if last.Type == "error" {
					break
				}
			}
			if last.Type != "error" || last.Code != "invalid_update" ||
				last.Message != "session update is invalid" {
				t.Fatalf("terminal record = %#v", last)
			}
			if err := harness.wait(t); !errors.Is(err, ErrSessionReceive) {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}

func TestSessionFailuresAndPanicsAreSecretSafe(t *testing.T) {
	t.Parallel()
	const secret = "secret-session-canary"
	t.Run("receive error", func(t *testing.T) {
		session := newTestSession()
		session.receiveErr = errors.New(secret)
		harness := startShell(t, session)
		record := harness.nextRecord(t)
		if record.Code != "receive_failed" || strings.Contains(harness.output.String(), secret) {
			t.Fatalf("record/output = %#v / %q", record, harness.output.String())
		}
		if err := harness.wait(t); !errors.Is(err, ErrSessionReceive) || strings.Contains(err.Error(), secret) {
			t.Fatalf("Run() error = %v", err)
		}
	})
	t.Run("receive panic", func(t *testing.T) {
		session := newTestSession()
		session.receivePanic = secret
		harness := startShell(t, session)
		if record := harness.nextRecord(t); record.Code != "receive_failed" {
			t.Fatalf("record = %#v", record)
		}
		if err := harness.wait(t); !errors.Is(err, ErrSessionReceive) || strings.Contains(err.Error(), secret) {
			t.Fatalf("Run() error = %v", err)
		}
	})
	t.Run("perform panic", func(t *testing.T) {
		session := newTestSession()
		session.updates <- mustSnapshotUpdate(t, 1)
		session.perform = func(context.Context, agenttui.Intent) (agenttui.CommandResult, error) { panic(secret) }
		harness := startShell(t, session)
		_ = harness.nextRecord(t)
		harness.write(t, "submit value\n")
		if record := harness.nextRecord(t); record.Code != "perform_failed" || record.Message != "session command failed" {
			t.Fatalf("record = %#v", record)
		}
		if strings.Contains(harness.output.String(), secret) {
			t.Fatal("panic value reached JSONL output")
		}
		harness.write(t, "quit\n")
		_ = harness.nextRecord(t)
		if err := harness.wait(t); err != nil {
			t.Fatal(err)
		}
	})
}

func TestShellBoundsCommandQueues(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	session.updates <- mustSnapshotUpdate(t, 1)
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	session.perform = func(ctx context.Context, _ agenttui.Intent) (agenttui.CommandResult, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-block:
			return agenttui.NewCommandResult(mustText(t, "done"), nil)
		case <-ctx.Done():
			return agenttui.CommandResult{}, ctx.Err()
		}
	}
	harness := startShell(t, session)
	_ = harness.nextRecord(t)
	harness.write(t, "submit first\n")
	waitSignal(t, started, "first operation")
	for index := 0; index < ordinaryQueueDepth+2; index++ {
		harness.write(t, "submit queued\n")
	}
	var full wireRecord
	for {
		full = harness.nextRecord(t)
		if full.Code == "queue_full" {
			break
		}
	}
	if full.Message != "command queue is full" {
		t.Fatalf("queue record = %#v", full)
	}
	close(block)
	harness.write(t, "quit\n")
	for record := harness.nextRecord(t); record.Command != "quit"; record = harness.nextRecord(t) {
	}
	if err := harness.wait(t); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestShellBoundsCancelQueue(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	session.updates <- mustSnapshotUpdate(t, 1)
	started := make(chan struct{}, 1)
	session.perform = func(ctx context.Context, _ agenttui.Intent) (agenttui.CommandResult, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return agenttui.CommandResult{}, ctx.Err()
	}
	harness := startShell(t, session)
	_ = harness.nextRecord(t)
	harness.write(t, "cancel\n")
	waitSignal(t, started, "first cancel")
	for range cancelQueueDepth + 1 {
		harness.write(t, "cancel\n")
	}
	full := harness.nextRecord(t)
	if full.Code != "queue_full" || full.Command != "cancel" || full.Message != "command queue is full" {
		t.Fatalf("cancel queue record = %#v", full)
	}
	harness.write(t, "quit\n")
	if quit := harness.nextRecord(t); quit.Command != "quit" {
		t.Fatalf("quit record = %#v", quit)
	}
	if err := harness.wait(t); err != nil {
		t.Fatal(err)
	}
}

func TestShellBoundsInput(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	input := io.NopCloser(strings.NewReader(strings.Repeat("x", MaximumInputBytes+1) + "\n"))
	output := newCaptureWriter()
	shell, err := New(session, input, output)
	if err != nil {
		t.Fatal(err)
	}
	if err = shell.Run(context.Background()); !errors.Is(err, ErrInput) {
		t.Fatalf("Run() error = %v", err)
	}
	record := decodeWireRecord(t, output.next(t))
	if record.Code != "input_failed" || record.Message != "command input failed" {
		t.Fatalf("input failure record = %#v", record)
	}
}

func TestShellCancellationInterruptsAllLanes(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	session.updates <- mustSnapshotUpdate(t, 1)
	var active atomic.Int32
	session.perform = func(ctx context.Context, _ agenttui.Intent) (agenttui.CommandResult, error) {
		active.Add(1)
		defer active.Add(-1)
		<-ctx.Done()
		return agenttui.CommandResult{}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	harness := startShellContext(t, ctx, session)
	_ = harness.nextRecord(t)
	harness.write(t, "submit blocked\ncancel\n")
	deadline := time.Now().Add(testTimeout)
	for active.Load() != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if active.Load() != 2 {
		t.Fatalf("active lanes = %d", active.Load())
	}
	cancel()
	if err := harness.wait(t); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if active.Load() != 0 {
		t.Fatalf("active lanes after cancellation = %d", active.Load())
	}
}

func TestParserAndOutputBounds(t *testing.T) {
	t.Parallel()
	for _, invalid := range []string{"", "unknown", "cancel value", "quit value", "submit", "respond", "submit \x00"} {
		if _, _, err := parseCommand(invalid); err == nil {
			t.Fatalf("parseCommand(%q) error = nil", invalid)
		}
	}
	command, value, err := parseCommand("submit hello world")
	if err != nil || command != "submit" || value != "hello world" {
		t.Fatalf("parseCommand() = %q, %q, %v", command, value, err)
	}
	if err = writeAll(shortWriter{}, []byte("complete")); err != nil {
		t.Fatalf("writeAll() error = %v", err)
	}
	if err = writeAll(zeroWriter{}, []byte("fail")); !errors.Is(err, ErrOutput) {
		t.Fatalf("zero write error = %v", err)
	}
}

func TestShellConfigurationAndSingleUse(t *testing.T) {
	t.Parallel()
	session := newTestSession()
	for _, test := range []struct {
		name    string
		session agenttui.Session
		input   io.ReadCloser
		output  io.Writer
	}{
		{name: "session", input: io.NopCloser(strings.NewReader("")), output: io.Discard},
		{name: "input", session: session, output: io.Discard},
		{name: "output", session: session, input: io.NopCloser(strings.NewReader(""))},
	} {
		if _, err := New(test.session, test.input, test.output); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("New(%s) error = %v", test.name, err)
		}
	}
	shell, err := New(session, io.NopCloser(strings.NewReader("quit\n")), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = shell.Run(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil context Run() error = %v", err)
	}
	if err = shell.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = shell.Run(context.Background()); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("second Run() error = %v", err)
	}
}

func TestInputPanicIsContainedAndRedacted(t *testing.T) {
	t.Parallel()
	const secret = "private input panic"
	output := newCaptureWriter()
	shell, err := New(newTestSession(), panicReadCloser{value: secret}, output)
	if err != nil {
		t.Fatal(err)
	}
	if err = shell.Run(context.Background()); !errors.Is(err, ErrInput) || strings.Contains(err.Error(), secret) {
		t.Fatalf("Run() error = %v", err)
	}
	record := decodeWireRecord(t, output.next(t))
	if record.Code != "input_failed" || strings.Contains(output.String(), secret) {
		t.Fatalf("record/output = %#v / %q", record, output.String())
	}
}

func TestPerformFailuresAndInvalidResultsRemainNonTerminal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		call func(context.Context, agenttui.Intent) (agenttui.CommandResult, error)
		code string
	}{
		{name: "error", call: func(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
			return agenttui.CommandResult{}, errors.New("private failure")
		}, code: "perform_failed"},
		{name: "invalid result", call: func(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
			result, _ := agenttui.NewCommandResult(agenttui.Text{}, &agenttui.Intent{})
			return result, nil
		}, code: "invalid_result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := newTestSession()
			session.updates <- mustSnapshotUpdate(t, 1)
			var calls atomic.Int32
			session.perform = func(ctx context.Context, intent agenttui.Intent) (agenttui.CommandResult, error) {
				calls.Add(1)
				return test.call(ctx, intent)
			}
			harness := startShell(t, session)
			_ = harness.nextRecord(t)
			harness.write(t, "submit value\n")
			if record := harness.nextRecord(t); record.Code != test.code {
				t.Fatalf("record = %#v", record)
			}
			harness.write(t, "quit\n")
			_ = harness.nextRecord(t)
			if err := harness.wait(t); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Fatalf("Perform calls = %d, want exactly one", calls.Load())
			}
		})
	}
}

func TestIntentAndEmitterFailureBoundaries(t *testing.T) {
	t.Parallel()
	if _, err := commandIntent("unknown", ""); !errors.Is(err, ErrInput) {
		t.Fatalf("unknown intent error = %v", err)
	}
	if _, err := textIntent(agenttui.IntentSubmit, strings.Repeat("x", agenttui.MaximumPromptBytes+1)); !errors.Is(err, ErrInput) {
		t.Fatalf("oversized intent error = %v", err)
	}
	if _, err := textIntent(agenttui.IntentSubmit, "invalid\x00text"); !errors.Is(err, ErrInput) {
		t.Fatalf("invalid text intent error = %v", err)
	}

	for _, output := range []io.Writer{zeroWriter{}, excessiveWriter{}, panicWriter{}} {
		ctx, cancel := context.WithCancel(context.Background())
		emitter := newEmitter(output, cancel)
		emitter.result("submit", 1, "result")
		if !errors.Is(emitter.err(), ErrOutput) || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("output failure = %v, context = %v", emitter.err(), ctx.Err())
		}
		emitter.issue("ignored", "ignored", "", 0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	emitter := newEmitter(io.Discard, cancel)
	emitter.result("submit", 1, strings.Repeat("x", MaximumOutputBytes))
	if !errors.Is(emitter.err(), ErrOutput) || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("oversized output failure = %v, context = %v", emitter.err(), ctx.Err())
	}
}

type testSession struct {
	updates      chan agenttui.SessionUpdate
	receiveErr   error
	receivePanic any
	perform      func(context.Context, agenttui.Intent) (agenttui.CommandResult, error)
}

func newTestSession() *testSession {
	return &testSession{updates: make(chan agenttui.SessionUpdate, 32)}
}

func (session *testSession) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	if session.receivePanic != nil {
		panic(session.receivePanic)
	}
	if session.receiveErr != nil {
		return agenttui.SessionUpdate{}, session.receiveErr
	}
	select {
	case update := <-session.updates:
		return update, nil
	case <-ctx.Done():
		return agenttui.SessionUpdate{}, ctx.Err()
	}
}

func (session *testSession) Perform(ctx context.Context, intent agenttui.Intent) (agenttui.CommandResult, error) {
	if session.perform != nil {
		return session.perform(ctx, intent)
	}
	return agenttui.NewCommandResult(mustTextForSession("ok"), nil)
}

type shellHarness struct {
	input  *io.PipeWriter
	output *captureWriter
	done   <-chan error
}

func startShell(t *testing.T, session agenttui.Session) *shellHarness {
	t.Helper()
	return startShellContext(t, context.Background(), session)
}

func startShellContext(t *testing.T, ctx context.Context, session agenttui.Session) *shellHarness {
	t.Helper()
	reader, writer := io.Pipe()
	output := newCaptureWriter()
	shell, err := New(session, reader, output)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- shell.Run(ctx) }()
	return &shellHarness{input: writer, output: output, done: done}
}

func (harness *shellHarness) write(t *testing.T, content string) {
	t.Helper()
	if _, err := io.WriteString(harness.input, content); err != nil {
		t.Fatalf("write input: %v", err)
	}
}

func (harness *shellHarness) nextRecord(t *testing.T) wireRecord {
	t.Helper()
	return decodeWireRecord(t, harness.output.next(t))
}

func decodeWireRecord(t *testing.T, line []byte) wireRecord {
	t.Helper()
	var record wireRecord
	if err := json.Unmarshal(line, &record); err != nil {
		t.Fatalf("decode record %q: %v", line, err)
	}
	if record.Schema != recordSchema {
		t.Fatalf("record schema = %q", record.Schema)
	}
	return record
}

func (harness *shellHarness) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-harness.done:
		_ = harness.input.Close()
		return err
	case <-time.After(testTimeout):
		t.Fatal("semantic shell did not stop")
		return nil
	}
}

func (harness *shellHarness) assertSequences(t *testing.T) {
	t.Helper()
	records := harness.output.records()
	for index, line := range records {
		var value wireRecord
		if err := json.Unmarshal(line, &value); err != nil || value.Sequence != uint64(index+1) {
			t.Fatalf("record %d sequence = %d, error %v", index, value.Sequence, err)
		}
	}
}

type captureWriter struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	queue  chan []byte
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{queue: make(chan []byte, 128)}
}

func (writer *captureWriter) Write(content []byte) (int, error) {
	copyOfContent := append([]byte(nil), content...)
	writer.mu.Lock()
	_, _ = writer.buffer.Write(copyOfContent)
	writer.mu.Unlock()
	writer.queue <- bytes.TrimSuffix(copyOfContent, []byte{'\n'})
	return len(content), nil
}

func (writer *captureWriter) next(t *testing.T) []byte {
	t.Helper()
	select {
	case record := <-writer.queue:
		return record
	case <-time.After(testTimeout):
		t.Fatal("semantic shell produced no record")
		return nil
	}
}

func (writer *captureWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.buffer.String()
}

func (writer *captureWriter) recordsSnapshot() [][]byte {
	content := []byte(writer.String())
	lines := bytes.Split(bytes.TrimSpace(content), []byte{'\n'})
	return lines
}

func (writer *captureWriter) records() [][]byte { return writer.recordsSnapshot() }

type wireRecord struct {
	Schema    string        `json:"schema"`
	Sequence  uint64        `json:"sequence"`
	Type      string        `json:"type"`
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Command   string        `json:"command"`
	Operation uint64        `json:"operation"`
	Revision  uint64        `json:"revision"`
	View      *semanticView `json:"view"`
}

func mustSnapshotUpdate(t *testing.T, revision uint64) agenttui.SessionUpdate {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(mustText(t, "workspace"), nil)
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusReady, mustText(t, "connected"), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenttui.NewSessionSnapshot(
		revision, workspace, status, []agenttui.Text{mustText(t, "ready")}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	update, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func mustActivityUpdate(t *testing.T, revision uint64, value string) agenttui.SessionUpdate {
	t.Helper()
	update, err := agenttui.NewActivityUpdate(revision, mustText(t, value))
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func mustText(t *testing.T, value string) agenttui.Text {
	t.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func mustTextForSession(value string) agenttui.Text {
	text, err := agenttui.NewText(value)
	if err != nil {
		panic(err)
	}
	return text
}

func waitSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(testTimeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}

type shortWriter struct{}

func (shortWriter) Write(content []byte) (int, error) { return min(2, len(content)), nil }

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type excessiveWriter struct{}

func (excessiveWriter) Write(content []byte) (int, error) { return len(content) + 1, nil }

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("private output panic") }

type panicReadCloser struct{ value string }

func (reader panicReadCloser) Read([]byte) (int, error) { panic(reader.value) }

func (reader panicReadCloser) Close() error { panic(reader.value) }

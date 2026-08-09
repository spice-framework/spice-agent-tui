package tuittest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestKeyTranslationCoversNamedModifiedAndTextInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stroke string
		text   string
		want   string
	}{
		{stroke: "enter", want: "enter"},
		{stroke: "return", want: "enter"},
		{stroke: "esc", want: "esc"},
		{stroke: "escape", want: "esc"},
		{stroke: "backspace", want: "backspace"},
		{stroke: "left", want: "left"},
		{stroke: "right", want: "right"},
		{stroke: "up", want: "up"},
		{stroke: "down", want: "down"},
		{stroke: "home", want: "home"},
		{stroke: "end", want: "end"},
		{stroke: "tab", want: "tab"},
		{stroke: "space", want: "space"},
		{stroke: "alt+enter", want: "alt+enter"},
		{stroke: "ctrl+p", want: "ctrl+p"},
		{stroke: "alt+x", want: "alt+x"},
		{stroke: "text", text: "é", want: "é"},
		{stroke: "runes", text: "ab", want: "ab"},
		{stroke: "z", want: "z"},
		{stroke: "named", text: "x", want: "x"},
	}
	for _, test := range tests {
		t.Run(test.stroke+test.text, func(t *testing.T) {
			t.Parallel()
			message, err := keyPress(test.stroke, test.text)
			if err != nil {
				t.Fatal(err)
			}
			if got := message.Keystroke(); got != test.want {
				t.Fatalf("keystroke = %q, want %q", got, test.want)
			}
		})
	}
}

func TestKeyTranslationRejectsUnsafeOrUnsupportedInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stroke string
		text   string
	}{
		{name: "empty", stroke: " "},
		{name: "missing text", stroke: "text"},
		{name: "long control", stroke: "ctrl+ab"},
		{name: "long alt", stroke: "alt+ab"},
		{name: "newline", stroke: "text", text: "a\nb"},
		{name: "tab", stroke: "text", text: "a\tb"},
		{name: "escape", stroke: "text", text: "\x1b"},
		{name: "invalid utf8", stroke: "text", text: string([]byte{0xff})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := keyPress(test.stroke, test.text); err == nil {
				t.Fatal("expected key translation error")
			}
		})
	}
	if _, err := modifiedKey("ctrl+\x1f", "ctrl+", tea.ModCtrl); err == nil {
		t.Fatal("expected control-rune modifier error")
	}
}

func TestScenarioStepsCoverEveryOperationAndFailure(t *testing.T) {
	t.Parallel()
	driver, err := NewDriver(Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	update := sessionUpdate(t, 1)
	steps := []Step{
		UpdateStep(update),
		ResizeStep(36, 9),
		TypeStep("abc"),
		KeyStep("left"),
		KeyStep("text", "!"),
		ActionStep(agenttui.ActionCursorEnd),
		SnapshotStep("all-steps"),
	}
	if err := driver.RunScenario(steps...); err != nil {
		t.Fatal(err)
	}
	if driver.Prompt() != "ab!c" || len(driver.History()) != 1 {
		t.Fatalf("prompt/history = %q/%d", driver.Prompt(), len(driver.History()))
	}
	wantLabels := []string{"update", "resize", "type", "key:left", "key:text", "action:cursor-end", "snapshot:all-steps"}
	for index, step := range steps {
		if got := step.label(); got != wantLabels[index] {
			t.Fatalf("label %d = %q, want %q", index, got, wantLabels[index])
		}
	}
	invalid := []Step{
		{kind: stepUpdate},
		{kind: stepResize},
		{kind: stepKind(99)},
	}
	for _, step := range invalid {
		if err := step.apply(driver); err == nil {
			t.Fatalf("step %d unexpectedly succeeded", step.kind)
		}
	}
	if got := (Step{}).label(); got != "unknown" {
		t.Fatalf("unknown label = %q", got)
	}
	if err := driver.RunScenario(KeyStep("text")); err == nil || !strings.Contains(err.Error(), "scenario step 0") {
		t.Fatalf("scenario error = %v", err)
	}
}

func TestDriverValidationAccessorsAndRenderFailures(t *testing.T) {
	t.Parallel()
	invalid := agenttui.ViewData{}
	if _, err := NewDriver(Options{Initial: &invalid}); err == nil {
		t.Fatal("expected invalid initial view error")
	}
	if _, err := NewDriver(Options{Bindings: []agenttui.KeyBinding{nil}}); err == nil {
		t.Fatal("expected invalid binding error")
	}
	driver, err := NewDriver(Options{Renderer: failingRenderer{}})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if _, err := driver.Screen("failure-view"); err != nil {
		t.Fatalf("fallback failure screen: %v", err)
	}
	if driver.StatusLevel() != agenttui.StatusReconnecting || driver.Revision() != 0 {
		t.Fatalf("initial status/revision = %q/%d", driver.StatusLevel(), driver.Revision())
	}
	if err := driver.InjectUpdate(sessionUpdate(t, 2)); err != nil {
		t.Fatal(err)
	}
	if driver.StatusLevel() != agenttui.StatusReady || driver.Revision() != 2 {
		t.Fatalf("updated status/revision = %q/%d", driver.StatusLevel(), driver.Revision())
	}
	if err := driver.Key("text", "x", "y"); err == nil {
		t.Fatal("expected multiple text argument error")
	}
	if err := driver.Type("bad\ntext"); err == nil {
		t.Fatal("expected typed control error")
	}
	if err := driver.Action(agenttui.Action("unknown")); err == nil {
		t.Fatal("expected unbound action error")
	}
	if err := driver.InjectUpdate(agenttui.SessionUpdate{}); err == nil {
		t.Fatal("expected invalid update error")
	}
	driver.Close()
	if err := driver.Action(agenttui.ActionSubmit); !errors.Is(err, ErrDriverClosed) {
		t.Fatalf("closed action error = %v", err)
	}
}

func TestDefaultConnectingViewAndPureRenderBoundaries(t *testing.T) {
	t.Parallel()
	view, err := DefaultConnectingView("  ")
	if err != nil {
		t.Fatal(err)
	}
	if view.Workspace().Title().String() != "Spice Agent" {
		t.Fatalf("default title = %q", view.Workspace().Title().String())
	}
	screen, err := RenderScreen(view, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if screen.Width() != 48 || screen.Height() != 12 {
		t.Fatalf("default render size = %dx%d", screen.Width(), screen.Height())
	}
	if _, err := RenderScreen(agenttui.ViewData{}, RenderOptions{}); err == nil {
		t.Fatal("expected invalid view error")
	}
	if _, err := RenderScreen(view, RenderOptions{Renderer: failingRenderer{}}); err == nil {
		t.Fatal("expected renderer error")
	}
}

func TestScreenInspectionDiffAndNormalization(t *testing.T) {
	t.Parallel()
	frame, err := agenttui.NewFrame("one\ntwo", agenttui.BoundedSize(8, 2))
	if err != nil {
		t.Fatal(err)
	}
	frame, err = frame.WithCursor(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	screen, err := FromFrame(
		frame,
		WithName("inspect"),
		WithSemantic("prompt", "ready", "ok", []string{"one event"}, 7, false, true),
	)
	if err != nil {
		t.Fatal(err)
	}
	if screen.Name() != "inspect" || screen.Prompt() != "prompt" || screen.Status() != "ok" ||
		screen.StatusLevel() != "ready" || screen.Revision() != 7 || screen.String() != screen.Plain() {
		t.Fatalf("screen accessors disagree:\n%s", screen.AgentReport())
	}
	activity := screen.Activity()
	activity[0] = "changed"
	if screen.Activity()[0] != "one event" {
		t.Fatal("activity accessor was not defensive")
	}
	if line, err := screen.Line(1); err != nil || line != "two" {
		t.Fatalf("line = %q, %v", line, err)
	}
	if _, err := screen.Line(-1); err == nil {
		t.Fatal("expected negative line error")
	}
	if _, err := screen.Line(2); err == nil {
		t.Fatal("expected high line error")
	}
	duplicate := screen
	if !screen.EqualStyled(duplicate) || !screen.EqualPlain(duplicate) || screen.Diff(duplicate) != "" {
		t.Fatal("identical screen comparison failed")
	}
	other := screen
	other.width++
	other.height++
	other.cursorX++
	other.cursorY = 0
	other.cursorVisible = false
	other.styled = "different"
	other.plain = "different"
	diff := screen.Diff(other)
	for _, field := range []string{"width", "height", "cursorX", "cursorY", "cursorVisible", "got styled", "got plain"} {
		if !strings.Contains(diff, field) {
			t.Fatalf("diff missing %q:\n%s", field, diff)
		}
	}
	normalized := NormalizeStyled("\x1b[1mvalue   ")
	if normalized != "<ESC>[1mvalue" || DenormalizeStyled(normalized) != "\x1b[1mvalue" {
		t.Fatalf("normalized = %q", normalized)
	}
	if _, err := FromFrame(agenttui.Frame{}); err == nil {
		t.Fatal("expected invalid frame error")
	}
}

func TestGoldenLifecycleUpdateLoadAssertAndMismatch(t *testing.T) {
	driver, err := NewDriver(Options{Width: 24, Height: 6})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	screen, err := driver.Screen("lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv(UpdateGoldenEnv, "yes")
	if !UpdateGolden() {
		t.Fatal("truthy update flag was not recognized")
	}
	if compareErr := screen.CompareGolden(dir, "lifecycle"); compareErr != nil {
		t.Fatal(compareErr)
	}
	t.Setenv(UpdateGoldenEnv, "")
	screen.AssertGolden(t, dir, "lifecycle")
	loaded, err := LoadGoldenScreen(dir, "lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name() != "lifecycle" || loaded.Plain() != screen.Plain() || loaded.Styled() != screen.Styled() {
		t.Fatal("loaded golden does not match written content")
	}
	if err := screen.CompareGolden(dir, ""); err == nil {
		t.Fatal("expected empty golden name error")
	}
	paths := PathsFor(dir, "lifecycle")
	if err := os.WriteFile(paths.Plain, []byte("different\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := screen.CompareGolden(dir, "lifecycle"); err == nil || !strings.Contains(err.Error(), "screen mismatch") {
		t.Fatalf("pixel mismatch error = %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(UpdateGoldenEnv, "true")
	if err := screen.CompareGolden(filepath.Join(blocked, "child"), "screen"); err == nil {
		t.Fatal("expected golden directory creation error")
	}
}

func TestScriptSessionQueueConfigurationAndFailureContracts(t *testing.T) {
	t.Parallel()
	session := &ScriptSession{}
	update := sessionUpdate(t, 1)
	if err := session.PushUpdate(update); err != nil {
		t.Fatal(err)
	}
	got, receiveErr := session.Receive(context.Background())
	if receiveErr != nil || got.Revision() != 1 {
		t.Fatalf("receive = %#v, %v", got, receiveErr)
	}
	if err := session.PushUpdate(agenttui.SessionUpdate{}); err == nil {
		t.Fatal("expected invalid update error")
	}
	_, nilReceiveErr := session.Receive(nil) //nolint:staticcheck // Deliberately verify the documented nil-context boundary.
	if nilReceiveErr == nil {
		t.Fatal("expected nil receive context error")
	}
	intent, intentErr := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if intentErr != nil {
		t.Fatal(intentErr)
	}
	if _, err := session.Perform(context.Background(), intent); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured perform error = %v", err)
	}
	_, nilPerformErr := session.Perform(nil, intent) //nolint:staticcheck // Deliberately verify the documented nil-context boundary.
	if nilPerformErr == nil {
		t.Fatal("expected nil perform context error")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Perform(canceled, intent); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled perform error = %v", err)
	}
	if _, err := session.Perform(context.Background(), agenttui.Intent{}); err == nil {
		t.Fatal("expected invalid intent error")
	}
	if err := session.SetPerformError(nil); err == nil {
		t.Fatal("expected nil perform error rejection")
	}
	configured := errors.New("configured")
	if err := session.SetPerformError(configured); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Perform(context.Background(), intent); !errors.Is(err, configured) {
		t.Fatalf("configured perform error = %v", err)
	}
	result, resultErr := agenttui.NewCommandResult(testText(t, "done"), nil)
	if resultErr != nil {
		t.Fatal(resultErr)
	}
	if err := session.SetPerformResult(result); err != nil {
		t.Fatal(err)
	}
	gotResult, err := session.Perform(context.Background(), intent)
	if err != nil || gotResult.Message().String() != "done" {
		t.Fatalf("perform result = %#v, %v", gotResult, err)
	}
	if len(session.Intents()) != 3 {
		t.Fatalf("recorded intents = %d", len(session.Intents()))
	}
	session.Close()
	if err := session.PushUpdate(update); err == nil {
		t.Fatal("expected push after close error")
	}
	if err := session.SetPerformResult(result); err == nil {
		t.Fatal("expected result configuration after close error")
	}
	if _, err := session.Receive(context.Background()); err == nil {
		t.Fatal("expected receive after close error")
	}
	if _, err := session.Perform(context.Background(), intent); err == nil {
		t.Fatal("expected perform after close error")
	}
}

func TestScriptSessionWakesBlockedReceive(t *testing.T) {
	t.Parallel()
	session := NewScriptSession()
	result := make(chan agenttui.SessionUpdate, 1)
	failure := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		update, err := session.Receive(ctx)
		if err != nil {
			failure <- err
			return
		}
		result <- update
	}()
	if err := session.PushUpdate(sessionUpdate(t, 4)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-failure:
		t.Fatal(err)
	case update := <-result:
		if update.Revision() != 4 {
			t.Fatalf("revision = %d", update.Revision())
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

type failingRenderer struct{}

func (failingRenderer) Render(agenttui.ViewData, agenttui.Size, agenttui.Theme) (agenttui.Frame, error) {
	return agenttui.Frame{}, errors.New("render failed")
}

func sessionUpdate(t *testing.T, revision uint64) agenttui.SessionUpdate {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(testText(t, "Workspace"), nil)
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusReady, testText(t, "ready"), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenttui.NewSessionSnapshot(
		revision,
		workspace,
		status,
		[]agenttui.Text{testText(t, "event")},
		nil,
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

func testText(t *testing.T, value string) agenttui.Text {
	t.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

package tuittest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

func TestDriverTypesEditsAndSnapshotsPixelPerfectChrome(t *testing.T) {
	t.Parallel()
	driver, err := tuittest.NewDriver(tuittest.Options{Width: 48, Height: 12})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if typeErr := driver.Type("list owners"); typeErr != nil {
		t.Fatal(typeErr)
	}
	if driver.Prompt() != "list owners" {
		t.Fatalf("prompt = %q", driver.Prompt())
	}
	if keyErr := driver.Key("backspace"); keyErr != nil {
		t.Fatal(keyErr)
	}
	if driver.Prompt() != "list owner" {
		t.Fatalf("edited prompt = %q", driver.Prompt())
	}
	screen, err := driver.Snapshot("typed-prompt")
	if err != nil {
		t.Fatal(err)
	}
	if !screen.Contains("Spice Agent") || !screen.Contains("list owner") {
		t.Fatalf("screen missing expected content\n%s", screen.AgentReport())
	}
	if screen.Width() != 48 || screen.Height() != 12 {
		t.Fatalf("screen size = %dx%d", screen.Width(), screen.Height())
	}
	if _, _, visible := screen.Cursor(); !visible {
		t.Fatal("expected visible cursor on normal mode prompt")
	}
	// Pixel contract: every plain line is exactly the canvas width.
	for index, line := range screen.Lines() {
		if tuittest.CellWidth(line) != 48 {
			t.Fatalf("line %d width = %d, want 48 (%q)", index, tuittest.CellWidth(line), line)
		}
	}
	if err := screen.CompareGolden("testdata", "typed-prompt"); err != nil {
		t.Fatal(err)
	}
}

func TestDriverScenarioInjectsSessionUpdatesAndSubmit(t *testing.T) {
	t.Parallel()
	session := tuittest.NewScriptSession()
	result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if configureErr := session.SetPerformResult(result); configureErr != nil {
		t.Fatal(configureErr)
	}
	driver, err := tuittest.NewDriver(tuittest.Options{
		Width:   40,
		Height:  10,
		Session: session,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	readyStatus, err := agenttui.NewStatus(agenttui.StatusReady, mustText(t, "ready for prompts"), []agenttui.Text{
		mustText(t, "enter submit"),
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := agenttui.NewWorkspace(mustText(t, "Commerce"), []agenttui.Section{
		mustSection(t, "Module", "orders"),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenttui.NewSessionSnapshot(1, workspace, readyStatus, []agenttui.Text{
		mustText(t, "connected"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	update, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.RunScenario(
		tuittest.UpdateStep(update),
		tuittest.SnapshotStep("ready"),
		tuittest.TypeStep("show orders"),
		tuittest.KeyStep("enter"),
		tuittest.SnapshotStep("submitted"),
	); err != nil {
		t.Fatal(err)
	}
	history := driver.History()
	if len(history) != 2 {
		t.Fatalf("history = %d", len(history))
	}
	if !history[0].Contains("Commerce") || !history[0].Contains("[READY]") {
		t.Fatalf("ready screen:\n%s", history[0].AgentReport())
	}
	intents := session.Intents()
	if len(intents) != 1 || intents[0].Kind() != agenttui.IntentSubmit {
		t.Fatalf("intents = %#v", intents)
	}
	if intents[0].Values()[0].String() != "show orders" {
		t.Fatalf("submitted value = %q", intents[0].Values()[0].String())
	}
	if got, ok := driver.LastResult(); !ok || got.Message().String() != "accepted" {
		t.Fatalf("last result = %#v, %t", got, ok)
	}
}

func TestDriverAccessibleModeAndQuit(t *testing.T) {
	t.Parallel()
	driver, err := tuittest.NewDriver(tuittest.Options{
		Width:      60,
		Height:     16,
		Accessible: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	screen, err := driver.Screen("accessible-initial")
	if err != nil {
		t.Fatal(err)
	}
	if !screen.Accessible() {
		t.Fatal("expected accessible screen")
	}
	if strings.Contains(screen.Styled(), "<ESC>") {
		t.Fatalf("accessible screen should not include ANSI escapes:\n%s", screen.Styled())
	}
	if err := driver.Key("ctrl+c"); err != nil {
		t.Fatal(err)
	}
	if !driver.QuitRequested() {
		t.Fatal("ctrl+c should request quit")
	}
}

func TestRenderScreenGoldenMatchesFixedRenderer(t *testing.T) {
	t.Parallel()
	workspace, err := agenttui.NewWorkspace(mustText(t, "PetClinic"), []agenttui.Section{
		mustSection(t, "Summary", "2 owners\n1 pet"),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusDisconnected, mustText(t, "session unavailable"), []agenttui.Text{
		mustText(t, "ctrl+c quit"),
	})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := agenttui.NewEditor("owner")
	if err != nil {
		t.Fatal(err)
	}
	data, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{
		mustText(t, "visit scheduled"),
	})
	if err != nil {
		t.Fatal(err)
	}
	screen, err := tuittest.RenderScreen(data, tuittest.RenderOptions{
		Width:  48,
		Height: 10,
		Name:   "petclinic-disconnected",
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := "testdata"
	if err := screen.CompareGolden(dir, "petclinic-disconnected"); err != nil {
		t.Fatal(err)
	}
}

func TestScriptSessionRecordsPerformErrors(t *testing.T) {
	t.Parallel()
	session := tuittest.NewScriptSession()
	if err := session.SetPerformError(errBoom{}); err != nil {
		t.Fatal(err)
	}
	driver, err := tuittest.NewDriver(tuittest.Options{Session: session, Width: 32, Height: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if typeErr := driver.Type("hello"); typeErr != nil {
		t.Fatal(typeErr)
	}
	if keyErr := driver.Key("enter"); keyErr != nil {
		t.Fatal(keyErr)
	}
	screen, err := driver.Screen("error")
	if err != nil {
		t.Fatal(err)
	}
	if !screen.Contains("[ERROR]") || !screen.Contains("operation failed") {
		t.Fatalf("error screen:\n%s", screen.AgentReport())
	}
}

func TestDriverUsesInjectedBindingForSemanticAction(t *testing.T) {
	t.Parallel()
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	customKey, err := agenttui.NewKey("ctrl+p", "")
	if err != nil {
		t.Fatal(err)
	}
	for index, binding := range bindings {
		if binding.Action() != agenttui.ActionCursorEnd {
			continue
		}
		custom, bindingErr := agenttui.NewBinding(binding.Action(), []agenttui.Key{customKey}, binding.Help())
		if bindingErr != nil {
			t.Fatal(bindingErr)
		}
		bindings[index] = custom
	}
	driver, err := tuittest.NewDriver(tuittest.Options{Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Type("owners"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key("home"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Action(agenttui.ActionCursorEnd); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("!"); err != nil {
		t.Fatal(err)
	}
	if driver.Prompt() != "owners!" {
		t.Fatalf("prompt = %q", driver.Prompt())
	}
}

func TestDriverTreatsGraphemeAsOneEditAndClosesOnQuit(t *testing.T) {
	t.Parallel()
	driver, err := tuittest.NewDriver(tuittest.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	value := "e\u0301"
	if got := tuittest.GraphemeCount(value); got != 1 {
		t.Fatalf("grapheme count = %d, want 1", got)
	}
	if err := driver.Type(value); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key("backspace"); err != nil {
		t.Fatal(err)
	}
	if driver.Prompt() != "" {
		t.Fatalf("prompt after grapheme backspace = %q", driver.Prompt())
	}
	if err := driver.Key("ctrl+q"); err != nil {
		t.Fatal(err)
	}
	if !driver.QuitRequested() {
		t.Fatal("quit was not recorded")
	}
	if err := driver.Type("later"); !errors.Is(err, tuittest.ErrDriverClosed) {
		t.Fatalf("type after quit error = %v", err)
	}
	if _, err := driver.Screen("final"); err != nil {
		t.Fatalf("screen after quit: %v", err)
	}
}

func TestDriverReturnsPerformTimeoutAndCancelsWork(t *testing.T) {
	t.Parallel()
	session := &blockingSession{finished: make(chan struct{})}
	driver, err := tuittest.NewDriver(tuittest.Options{
		Session:      session,
		DrainTimeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if typeErr := driver.Type("run"); typeErr != nil {
		t.Fatal(typeErr)
	}
	err = driver.Key("enter")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("submit error = %v", err)
	}
	select {
	case <-session.finished:
	case <-time.After(time.Second):
		t.Fatal("blocked perform did not receive cancellation")
	}
	if err := driver.Resize(80, 24); !errors.Is(err, tuittest.ErrDriverClosed) {
		t.Fatalf("resize after timeout error = %v", err)
	}
}

func TestScriptSessionReceiveCancellationAndClose(t *testing.T) {
	t.Parallel()
	session := tuittest.NewScriptSession()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Receive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("receive error = %v", err)
	}
	receiveCalls, performCalls := session.Stats()
	if receiveCalls != 1 || performCalls != 0 {
		t.Fatalf("stats = (%d,%d)", receiveCalls, performCalls)
	}
	session.Close()
	session.Close()
	if err := session.SetPerformError(errBoom{}); err == nil {
		t.Fatal("expected closed configuration error")
	}
}

func TestGoldenComparisonRequiresMetadataAndNeverBootstraps(t *testing.T) {
	t.Setenv(tuittest.UpdateGoldenEnv, "")
	driver, err := tuittest.NewDriver(tuittest.Options{Width: 20, Height: 6})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	screen, err := driver.Screen("metadata")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := writeGoldens(dir, "metadata", screen); err != nil {
		t.Fatal(err)
	}
	paths := tuittest.PathsFor(dir, "metadata")
	if err := os.WriteFile(paths.Report, []byte("stale metadata\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := screen.CompareGolden(dir, "metadata"); err == nil || !strings.Contains(err.Error(), "metadata mismatch") {
		t.Fatalf("metadata comparison error = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := screen.CompareGolden(missing, "screen"); err == nil {
		t.Fatal("expected missing golden error")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("normal comparison created directory: %v", err)
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type blockingSession struct {
	finished chan struct{}
}

func (*blockingSession) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	<-ctx.Done()
	return agenttui.SessionUpdate{}, ctx.Err()
}

func (session *blockingSession) Perform(ctx context.Context, _ agenttui.Intent) (agenttui.CommandResult, error) {
	defer close(session.finished)
	<-ctx.Done()
	return agenttui.CommandResult{}, ctx.Err()
}

func mustText(t *testing.T, value string) agenttui.Text {
	t.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func mustSection(t *testing.T, title, body string) agenttui.Section {
	t.Helper()
	section, err := agenttui.NewSection(mustText(t, title), mustText(t, body))
	if err != nil {
		t.Fatal(err)
	}
	return section
}

func writeGoldens(dir, name string, screen tuittest.Screen) error {
	paths := tuittest.PathsFor(dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(paths.Styled, []byte(screen.Styled()+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(paths.Plain, []byte(screen.Plain()+"\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(paths.Report, []byte(screen.AgentReport()), 0o644)
}

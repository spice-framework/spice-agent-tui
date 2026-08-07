package presentation

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestModelResizeAndKeyboardEditing(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	updated, command := model.Update(tea.WindowSizeMsg{Width: agenttui.MaximumWidth + 20, Height: 9})
	if command != nil {
		t.Fatal("resize returned command")
	}
	model = asModel(t, updated)
	if model.Size().Width() != agenttui.MaximumWidth || model.Size().Height() != 9 {
		t.Fatalf("resized model = %dx%d", model.Size().Width(), model.Size().Height())
	}
	model = updateKey(t, model, tea.Key{Code: 'x', Text: "x"})
	model = updateKey(t, model, tea.Key{Code: tea.KeyLeft})
	model = updateKey(t, model, tea.Key{Code: tea.KeyBackspace})
	if model.Editor().Value().String() != "ownex" || model.Editor().Cursor() != 4 {
		t.Fatalf("edited prompt = %q at %d", model.Editor().Value().String(), model.Editor().Cursor())
	}
	model = updateKey(t, model, tea.Key{Code: tea.KeyRight})
	if model.Editor().Cursor() != 5 {
		t.Fatalf("right cursor = %d", model.Editor().Cursor())
	}
	if view := model.View(); !view.AltScreen || view.Content == "" {
		t.Fatalf("View() = %#v", view)
	}
	model = updateKey(t, model, tea.Key{Code: tea.KeyHome})
	if model.Editor().Cursor() != 0 {
		t.Fatalf("home cursor = %d", model.Editor().Cursor())
	}
	model = updateKey(t, model, tea.Key{Code: tea.KeyEnd})
	if model.Editor().Cursor() != 5 {
		t.Fatalf("end cursor = %d", model.Editor().Cursor())
	}
}

func TestModelResizeSequencePreservesStreamingAndEditorState(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	message, err := agenttui.NewActivityUpdate(1, mustText(t, "streaming"))
	if err != nil {
		t.Fatal(err)
	}
	model = updateMessage(t, model, sessionUpdateMsg{update: message})
	wantEditor := model.Editor()
	for _, dimensions := range [][2]int{{0, 0}, {1, 1}, {12, 3}, {240, 100}, {500, 500}, {32, 7}} {
		model = updateMessage(t, model, tea.WindowSizeMsg{Width: dimensions[0], Height: dimensions[1]})
		if model.Editor() != wantEditor || model.Revision() != 1 || len(model.Activity()) == 0 {
			t.Fatalf("resize %v lost state", dimensions)
		}
		view := model.View()
		if view.Content == "" {
			t.Fatalf("resize %v rendered blank view", dimensions)
		}
	}
}

func TestModelExplicitQuitUnknownAndInvalidInput(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if command == nil || asModel(t, updated).Editor() != model.Editor() {
		t.Fatal("ctrl+c did not return quit command without mutation")
	}
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if command != nil || asModel(t, updated).Editor() != model.Editor() {
		t.Fatal("unknown key mutated model")
	}
	updated, command = model.Update(struct{}{})
	if command != nil || asModel(t, updated).Editor() != model.Editor() {
		t.Fatal("unknown message mutated model")
	}
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "\x1b"}))
	if command != nil || asModel(t, updated).Editor() != model.Editor() {
		t.Fatal("unsafe key text mutated model")
	}
}

func TestModelConstructionAndRenderFailure(t *testing.T) {
	t.Parallel()
	view := fixtureView(t)
	if _, err := NewModel(nil, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.DarkTheme(), standardBindings(t), nil); err == nil {
		t.Fatal("NewModel(nil renderer) error = nil")
	}
	if _, err := NewModel(FixedRenderer{}, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.ThemeState{}, standardBindings(t), nil); err == nil {
		t.Fatal("NewModel(zero theme) error = nil")
	}
	model := fixtureModel(t, failingRenderer{})
	if got := model.View(); !got.AltScreen || got.Content != "Spice Agent TUI\n[ERROR] render unavailable" {
		t.Fatalf("failure View() = %#v", got)
	}
	accessible := model.WithAccessibleMode(true).View()
	if accessible.AltScreen || accessible.Cursor != nil || strings.Contains(accessible.Content, "\x1b") ||
		!strings.Contains(accessible.Content, "[DISCONNECTED]") {
		t.Fatalf("accessible failure View() = %#v", accessible)
	}
}

func TestModelAppliesRevisionedStreamingSnapshotsAndBoundsActivity(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	view := fixtureView(t)
	disconnected := mustStatusLevel(t, agenttui.StatusDisconnected, "session unavailable")
	snapshot := mustSnapshotUpdateMessage(t, 2, view.Workspace(), disconnected, view.Activity(), nil)
	model = updateMessage(t, model, snapshot)
	if model.Revision() != 2 || model.Status().Level() != agenttui.StatusDisconnected {
		t.Fatalf("snapshot state = revision %d, status %q", model.Revision(), model.Status().Level())
	}
	stale := mustSnapshotUpdateMessage(
		t, 1, view.Workspace(), mustStatusLevel(t, agenttui.StatusReady, "stale"), nil, nil,
	)
	model = updateMessage(t, model, stale)
	if model.Status().Level() != agenttui.StatusDisconnected {
		t.Fatal("stale snapshot replaced current status")
	}
	for revision := uint64(3); revision < 3+agenttui.MaximumActivityItems+4; revision++ {
		update, messageErr := agenttui.NewActivityUpdate(revision, mustText(t, fmt.Sprintf("stream %03d", revision)))
		if messageErr != nil {
			t.Fatal(messageErr)
		}
		model = updateMessage(t, model, sessionUpdateMsg{update: update})
	}
	activity := model.Activity()
	if len(activity) != agenttui.MaximumActivityItems || activity[len(activity)-1].String() != "stream 134" {
		t.Fatalf("bounded activity = %d items, last %q", len(activity), activity[len(activity)-1].String())
	}
	activity[0] = mustText(t, "mutated")
	if model.Activity()[0].String() == "mutated" {
		t.Fatal("Model.Activity() did not return a defensive copy")
	}
	unchanged := updateMessage(t, model, sessionUpdateMsg{})
	if unchanged.Revision() != model.Revision() || unchanged.Status().Level() != model.Status().Level() ||
		unchanged.Status().Message() != model.Status().Message() {
		t.Fatal("invalid snapshot mutated model")
	}
}

func TestModelNavigatesBoundedPromptHistory(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	history, err := agenttui.NewPromptHistoryUpdate(
		1,
		[]agenttui.Text{mustText(t, "first"), mustText(t, "second")},
	)
	if err != nil {
		t.Fatal(err)
	}
	model = updateMessage(t, model, sessionUpdateMsg{update: history})
	model = updateKey(t, model, tea.Key{Code: tea.KeyUp})
	if model.Editor().Value().String() != "second" {
		t.Fatalf("previous history = %q", model.Editor().Value().String())
	}
	model = updateKey(t, model, tea.Key{Code: tea.KeyUp})
	if model.Editor().Value().String() != "first" {
		t.Fatalf("oldest history = %q", model.Editor().Value().String())
	}
	model = updateKey(t, model, tea.Key{Code: tea.KeyDown})
	model = updateKey(t, model, tea.Key{Code: tea.KeyDown})
	if model.Editor().Value().String() != "owner" {
		t.Fatalf("restored draft = %q", model.Editor().Value().String())
	}
}

func TestModelAccessibleViewIsPlainAndStatusExplicit(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{}).WithAccessibleMode(true)
	view := model.View()
	if view.AltScreen || view.Cursor != nil || strings.Contains(view.Content, "\x1b") ||
		!strings.Contains(view.Content, "[DISCONNECTED] session unavailable") {
		t.Fatalf("accessible View() = %#v", view)
	}
}

func TestAccessibleViewIsLineOrientedAndResizeStable(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{}).WithAccessibleMode(true)
	before := model.View()
	model = updateMessage(t, model, tea.WindowSizeMsg{Width: 1, Height: 1})
	after := model.View()
	if before.Content != after.Content || strings.HasSuffix(after.Content, " ") ||
		strings.Count(after.Content, "\n") >= agenttui.MaximumHeight {
		t.Fatalf("accessible resize replayed a canvas\nbefore=%q\nafter=%q", before.Content, after.Content)
	}
}

func TestModelRejectsDuplicateInjectedBindings(t *testing.T) {
	t.Parallel()
	view := fixtureView(t)
	key := mustKey(t, "enter")
	first := mustBinding(t, agenttui.ActionSubmit, []agenttui.Key{key})
	sameAction := mustBinding(t, agenttui.ActionSubmit, []agenttui.Key{mustKey(t, "ctrl+enter")})
	sameKey := mustBinding(t, agenttui.ActionRespond, []agenttui.Key{key})
	for _, bindings := range [][]agenttui.KeyBinding{{first, sameAction}, {first, sameKey}, nil} {
		if _, err := NewModel(
			FixedRenderer{}, view.Workspace(), view.Status(), view.Prompt(), view.Activity(),
			agenttui.DarkTheme(), bindings, nil,
		); err == nil {
			t.Fatalf("NewModel(%d bindings) error = nil", len(bindings))
		}
	}
}

func fixtureModel(t *testing.T, renderer agenttui.Renderer) Model {
	t.Helper()
	view := fixtureView(t)
	model, err := NewModel(renderer, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.DarkTheme(), standardBindings(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.Init() != nil {
		t.Fatal("Init() command != nil")
	}
	return model
}

func standardBindings(t *testing.T) []agenttui.KeyBinding {
	t.Helper()
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	return bindings
}

func mustKey(t *testing.T, stroke string) agenttui.Key {
	t.Helper()
	key, err := agenttui.NewKey(stroke, "")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustBinding(t *testing.T, action agenttui.Action, keys []agenttui.Key) agenttui.Binding {
	t.Helper()
	binding, err := agenttui.NewBinding(action, keys, mustText(t, string(action)))
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func mustSnapshotUpdateMessage(
	t *testing.T,
	revision uint64,
	workspace agenttui.WorkspaceState,
	status agenttui.StatusState,
	activity []agenttui.Text,
	history []agenttui.Text,
) sessionUpdateMsg {
	t.Helper()
	snapshot, err := agenttui.NewSessionSnapshot(revision, workspace, status, activity, history)
	if err != nil {
		t.Fatal(err)
	}
	update, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return sessionUpdateMsg{update: update}
}

func updateKey(t *testing.T, model Model, key tea.Key) Model {
	t.Helper()
	updated, command := model.Update(tea.KeyPressMsg(key))
	if command != nil {
		t.Fatal("editing key returned command")
	}
	return asModel(t, updated)
}

func updateMessage(t *testing.T, model Model, message tea.Msg) Model {
	t.Helper()
	updated, command := model.Update(message)
	if command != nil {
		t.Fatal("presentation message returned command")
	}
	return asModel(t, updated)
}

func mustStatusLevel(t *testing.T, level agenttui.StatusLevel, message string) agenttui.StatusState {
	t.Helper()
	status, err := agenttui.NewStatus(level, mustText(t, message), []agenttui.Text{mustText(t, "ctrl+c quit")})
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func asModel(t *testing.T, value tea.Model) Model {
	t.Helper()
	model, ok := value.(Model)
	if !ok {
		t.Fatalf("model type = %T", value)
	}
	return model
}

type failingRenderer struct{}

func (failingRenderer) Render(agenttui.ViewData, agenttui.Size, agenttui.Theme) (agenttui.Frame, error) {
	return agenttui.Frame{}, errors.New("render failed")
}

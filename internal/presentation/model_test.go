package presentation

import (
	"errors"
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
}

func TestModelQuitUnknownAndInvalidInput(t *testing.T) {
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
	if _, err := NewModel(nil, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.DarkTheme()); err == nil {
		t.Fatal("NewModel(nil renderer) error = nil")
	}
	if _, err := NewModel(FixedRenderer{}, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.ThemeState{}); err == nil {
		t.Fatal("NewModel(zero theme) error = nil")
	}
	model := fixtureModel(t, failingRenderer{})
	if got := model.View(); !got.AltScreen || got.Content != "Spice Agent TUI\nrender unavailable" {
		t.Fatalf("failure View() = %#v", got)
	}
}

func fixtureModel(t *testing.T, renderer agenttui.Renderer) Model {
	t.Helper()
	view := fixtureView(t)
	model, err := NewModel(renderer, view.Workspace(), view.Status(), view.Prompt(), view.Activity(), agenttui.DarkTheme())
	if err != nil {
		t.Fatal(err)
	}
	if model.Init() != nil {
		t.Fatal("Init() command != nil")
	}
	return model
}

func updateKey(t *testing.T, model Model, key tea.Key) Model {
	t.Helper()
	updated, command := model.Update(tea.KeyPressMsg(key))
	if command != nil {
		t.Fatal("editing key returned command")
	}
	return asModel(t, updated)
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

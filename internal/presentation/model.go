package presentation

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

// Model is the deterministic Bubble Tea v2 presentation state. It owns no
// client, daemon, transport, annotation, or generated-application behavior.
type Model struct {
	renderer  agenttui.Renderer
	workspace agenttui.WorkspaceState
	status    agenttui.StatusState
	editor    agenttui.EditorState
	activity  []agenttui.Text
	bindings  []agenttui.Binding
	theme     agenttui.ThemeState
	size      agenttui.Size
}

// NewModel constructs a bounded presentation model from immutable snapshots.
func NewModel(
	renderer agenttui.Renderer,
	workspace agenttui.WorkspaceState,
	status agenttui.StatusState,
	editor agenttui.EditorState,
	activity []agenttui.Text,
	theme agenttui.ThemeState,
) (Model, error) {
	if renderer == nil {
		return Model{}, errors.New("renderer must not be nil")
	}
	if err := theme.Validate(); err != nil {
		return Model{}, fmt.Errorf("theme must be initialized: %w", err)
	}
	if _, err := agenttui.NewViewData(workspace, status, editor, activity); err != nil {
		return Model{}, err
	}
	bindings, err := defaultBindings()
	if err != nil {
		return Model{}, err
	}
	return Model{
		renderer: renderer, workspace: workspace, status: status, editor: editor,
		activity: append([]agenttui.Text(nil), activity...), bindings: bindings,
		theme: theme, size: agenttui.BoundedSize(80, 24),
	}, nil
}

// Init implements tea.Model without starting background work.
func (Model) Init() tea.Cmd { return nil }

// Update implements tea.Model using only resize and presentation key events.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.size = agenttui.BoundedSize(message.Width, message.Height)
		return model, nil
	case tea.KeyPressMsg:
		return model.updateKey(message)
	default:
		return model, nil
	}
}

func (model Model) updateKey(message tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key, err := agenttui.NewKey(message.Keystroke(), message.Key().Text)
	if err != nil {
		return model, nil
	}
	for _, binding := range model.bindings {
		if !binding.Matches(key) {
			continue
		}
		switch binding.Action() {
		case agenttui.ActionQuit:
			return model, tea.Quit
		case agenttui.ActionCursorLeft:
			model.editor = model.editor.Move(agenttui.MoveLeft)
		case agenttui.ActionCursorRight:
			model.editor = model.editor.Move(agenttui.MoveRight)
		case agenttui.ActionBackspace:
			model.editor = model.editor.Backspace()
		}
		return model, nil
	}
	if key.Text() != "" {
		if editor, insertErr := model.editor.Insert(key.Text()); insertErr == nil {
			model.editor = editor
		}
	}
	return model, nil
}

// View implements tea.Model and always requests the alternate screen.
func (model Model) View() tea.View {
	data, err := agenttui.NewViewData(model.workspace, model.status, model.editor, model.activity)
	if err != nil {
		return failureView()
	}
	frame, err := model.renderer.Render(data, model.size, model.theme)
	if err != nil {
		return failureView()
	}
	view := tea.NewView(frame.Content())
	view.AltScreen = true
	return view
}

// Size returns the current bounded terminal size for deterministic tests.
func (model Model) Size() agenttui.Size { return model.size }

// Editor returns the current immutable prompt snapshot.
func (model Model) Editor() agenttui.EditorState { return model.editor }

func failureView() tea.View {
	view := tea.NewView("Spice Agent TUI\nrender unavailable")
	view.AltScreen = true
	return view
}

func defaultBindings() ([]agenttui.Binding, error) {
	specifications := []struct {
		action agenttui.Action
		keys   []string
		help   string
	}{
		{action: agenttui.ActionQuit, keys: []string{"ctrl+c"}, help: "ctrl+c quit"},
		{action: agenttui.ActionCursorLeft, keys: []string{"left"}, help: "← move"},
		{action: agenttui.ActionCursorRight, keys: []string{"right"}, help: "→ move"},
		{action: agenttui.ActionBackspace, keys: []string{"backspace"}, help: "backspace delete"},
	}
	result := make([]agenttui.Binding, 0, len(specifications))
	for _, specification := range specifications {
		keys := make([]agenttui.Key, 0, len(specification.keys))
		for _, value := range specification.keys {
			key, err := agenttui.NewKey(value, "")
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
		}
		help, err := agenttui.NewText(specification.help)
		if err != nil {
			return nil, err
		}
		binding, err := agenttui.NewBinding(specification.action, keys, help)
		if err != nil {
			return nil, err
		}
		result = append(result, binding)
	}
	return result, nil
}

var _ tea.Model = Model{}

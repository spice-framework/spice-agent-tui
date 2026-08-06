package agenttui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CursorMove is a relative editor movement.
type CursorMove int

const (
	// MoveLeft moves one rune toward the start.
	MoveLeft CursorMove = -1
	// MoveRight moves one rune toward the end.
	MoveRight CursorMove = 1
)

// EditorState is an immutable PromptEditor implementation.
type EditorState struct {
	value  string
	cursor int
}

// NewEditor constructs an editor with its cursor after the initial value.
func NewEditor(initial string) (EditorState, error) {
	editor := EditorState{value: initial, cursor: utf8.RuneCountInString(initial)}
	return editor, editor.Validate()
}

// Value returns prompt text.
func (editor EditorState) Value() Text { return Text{value: editor.value} }

// Cursor returns the zero-based rune boundary.
func (editor EditorState) Cursor() int { return editor.cursor }

// Validate reports whether prompt text and its rune cursor are well formed.
func (editor EditorState) Validate() error {
	if err := validatePromptText(editor.value); err != nil {
		return err
	}
	if editor.cursor < 0 || editor.cursor > utf8.RuneCountInString(editor.value) {
		return errors.New("prompt cursor is outside its text")
	}
	return nil
}

// Insert returns a new editor with text inserted at the cursor.
func (editor EditorState) Insert(value string) (EditorState, error) {
	if err := editor.Validate(); err != nil {
		return EditorState{}, fmt.Errorf("edit invalid prompt: %w", err)
	}
	if err := validatePromptText(value); err != nil {
		return EditorState{}, err
	}
	current := []rune(editor.value)
	inserted := []rune(value)
	combined := make([]rune, 0, len(current)+len(inserted))
	combined = append(combined, current[:editor.cursor]...)
	combined = append(combined, inserted...)
	combined = append(combined, current[editor.cursor:]...)
	result := string(combined)
	if len(result) > MaximumPromptBytes {
		return EditorState{}, fmt.Errorf("prompt exceeds %d bytes", MaximumPromptBytes)
	}
	return EditorState{value: result, cursor: editor.cursor + len(inserted)}, nil
}

// Move returns a new editor at the nearest valid cursor boundary.
func (editor EditorState) Move(direction CursorMove) EditorState {
	if editor.Validate() != nil {
		return EditorState{}
	}
	switch direction {
	case MoveLeft:
		editor.cursor = max(editor.cursor-1, 0)
	case MoveRight:
		editor.cursor = min(editor.cursor+1, utf8.RuneCountInString(editor.value))
	}
	return editor
}

// Backspace removes the rune immediately before the cursor.
func (editor EditorState) Backspace() EditorState {
	if editor.Validate() != nil {
		return EditorState{}
	}
	if editor.cursor == 0 {
		return editor
	}
	current := []rune(editor.value)
	current = slices.Delete(current, editor.cursor-1, editor.cursor)
	return EditorState{value: string(current), cursor: editor.cursor - 1}
}

// Clear returns an empty editor.
func (EditorState) Clear() EditorState { return EditorState{} }

func validatePromptText(value string) error {
	if !utf8.ValidString(value) || len(value) > MaximumPromptBytes {
		return errors.New("prompt must be bounded valid UTF-8")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("prompt must not contain control characters")
		}
	}
	return nil
}

// Action identifies one presentation-only keyboard operation.
type Action string

const (
	// ActionQuit requests normal shell termination.
	ActionQuit Action = "quit"
	// ActionCursorLeft moves the prompt cursor left.
	ActionCursorLeft Action = "cursor-left"
	// ActionCursorRight moves the prompt cursor right.
	ActionCursorRight Action = "cursor-right"
	// ActionBackspace removes the previous prompt rune.
	ActionBackspace Action = "backspace"
)

func validAction(action Action) bool {
	return action == ActionQuit || action == ActionCursorLeft || action == ActionCursorRight || action == ActionBackspace
}

// Key is one immutable UI-neutral key event.
type Key struct {
	stroke string
	text   string
}

// NewKey constructs a canonical key event.
func NewKey(stroke, text string) (Key, error) {
	stroke = strings.ToLower(strings.TrimSpace(stroke))
	key := Key{stroke: stroke, text: text}
	return key, key.Validate()
}

// Stroke returns the canonical physical key identity.
func (key Key) Stroke() string { return key.stroke }

// Text returns printable inserted text, if any.
func (key Key) Text() string { return key.text }

// Validate reports whether the key identity and printable text are safe.
func (key Key) Validate() error {
	if key.stroke == "" || len(key.stroke) > 64 || strings.ContainsAny(key.stroke, "\n\t\x1b") {
		return errors.New("keystroke must be a bounded single-line identity")
	}
	return validatePromptText(key.text)
}

// Binding is an immutable KeyBinding implementation.
type Binding struct {
	action Action
	keys   []Key
	help   Text
}

// NewBinding constructs a semantic key binding.
func NewBinding(action Action, keys []Key, help Text) (Binding, error) {
	binding := Binding{action: action, keys: slices.Clone(keys), help: help}
	return binding, binding.Validate()
}

// Action returns the semantic action.
func (binding Binding) Action() Action { return binding.action }

// Keys returns a defensive copy.
func (binding Binding) Keys() []Key { return slices.Clone(binding.keys) }

// Help returns accessible key help.
func (binding Binding) Help() Text { return binding.help }

// Matches reports whether the canonical keystroke matches.
func (binding Binding) Matches(key Key) bool {
	if binding.Validate() != nil || key.Validate() != nil {
		return false
	}
	return slices.ContainsFunc(binding.keys, func(candidate Key) bool { return candidate.stroke == key.stroke })
}

// Validate reports whether the binding contains complete safe key metadata.
func (binding Binding) Validate() error {
	if !validAction(binding.action) {
		return fmt.Errorf("unsupported key action %q", binding.action)
	}
	if len(binding.keys) == 0 || len(binding.keys) > 8 {
		return errors.New("key binding must contain between one and eight keys")
	}
	for index, key := range binding.keys {
		if err := key.Validate(); err != nil {
			return fmt.Errorf("key binding key %d: %w", index, err)
		}
	}
	if err := binding.help.Validate(); err != nil {
		return fmt.Errorf("key help: %w", err)
	}
	return validateSingleLine(binding.help, "key help")
}

var (
	_ PromptEditor = EditorState{}
	_ KeyBinding   = Binding{}
)

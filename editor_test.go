package agenttui

import (
	"strings"
	"testing"
)

func TestEditorUsesRuneBoundariesAndImmutableUpdates(t *testing.T) {
	t.Parallel()
	editor, err := NewEditor("a界")
	if err != nil || editor.Cursor() != 2 {
		t.Fatalf("NewEditor() = %#v, %v", editor, err)
	}
	moved := editor.Move(MoveLeft)
	inserted, err := moved.Insert("🙂")
	if err != nil || inserted.Value().String() != "a🙂界" || inserted.Cursor() != 2 {
		t.Fatalf("Insert() = %#v, %v", inserted, err)
	}
	if editor.Value().String() != "a界" || editor.Cursor() != 2 {
		t.Fatal("editor was mutated")
	}
	deleted := inserted.Backspace()
	if deleted.Value().String() != "a界" || deleted.Cursor() != 1 || deleted.Move(MoveLeft).Move(MoveLeft).Cursor() != 0 || deleted.Move(MoveRight).Move(MoveRight).Cursor() != 2 {
		t.Fatalf("editor movement/delete = %#v", deleted)
	}
	if deleted.Move(42) != deleted || (EditorState{}).Backspace() != (EditorState{}) || deleted.Clear() != (EditorState{}) {
		t.Fatal("editor boundary operations are not stable")
	}
}

func TestEditorRejectsUnsafeOrOversizedInput(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"bad\ninput", "escape\x1b", string([]byte{0xff}), strings.Repeat("x", MaximumPromptBytes+1)} {
		if _, err := NewEditor(value); err == nil {
			t.Fatalf("NewEditor(%q) error = nil", value[:min(len(value), 20)])
		}
	}
	editor, err := NewEditor(strings.Repeat("x", MaximumPromptBytes))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := editor.Insert("y"); err == nil {
		t.Fatal("Insert(oversize) error = nil")
	}
	malformed := EditorState{value: "x", cursor: 2}
	if err := malformed.Validate(); err == nil {
		t.Fatal("Validate(malformed cursor) error = nil")
	}
	if _, err := malformed.Insert("y"); err == nil {
		t.Fatal("Insert(malformed cursor) error = nil")
	}
	if malformed.Move(MoveLeft) != (EditorState{}) || malformed.Backspace() != (EditorState{}) {
		t.Fatal("malformed cursor operation did not fail closed")
	}
}

func TestKeyBindingsAreCanonicalAndImmutable(t *testing.T) {
	t.Parallel()
	key, err := NewKey(" CTRL+C ", "")
	if err != nil || key.Stroke() != "ctrl+c" || key.Text() != "" {
		t.Fatalf("NewKey() = %#v, %v", key, err)
	}
	for _, stroke := range []string{"", "bad\nkey", "\x1b", strings.Repeat("x", 65)} {
		if _, keyErr := NewKey(stroke, ""); keyErr == nil {
			t.Fatalf("NewKey(%q) error = nil", stroke)
		}
	}
	if _, keyErr := NewKey("a", "\n"); keyErr == nil {
		t.Fatal("NewKey(control text) error = nil")
	}
	keys := []Key{key}
	binding, err := NewBinding(ActionQuit, keys, mustText(t, "quit"))
	if err != nil {
		t.Fatal(err)
	}
	keys[0] = Key{}
	copyKeys := binding.Keys()
	copyKeys[0] = Key{}
	if binding.Action() != ActionQuit || binding.Help().String() != "quit" || !binding.Matches(key) || binding.Matches(Key{}) || binding.Keys()[0] != key {
		t.Fatal("binding is mutable or matching is incorrect")
	}
	if _, err := NewBinding("unknown", []Key{key}, Text{}); err == nil {
		t.Fatal("NewBinding(unknown) error = nil")
	}
	if _, err := NewBinding(ActionQuit, nil, Text{}); err == nil {
		t.Fatal("NewBinding(no keys) error = nil")
	}
	if _, err := NewBinding(ActionQuit, make([]Key, 9), Text{}); err == nil {
		t.Fatal("NewBinding(too many keys) error = nil")
	}
	if _, err := NewBinding(ActionQuit, []Key{key}, mustText(t, "bad\nhelp")); err == nil {
		t.Fatal("NewBinding(multiline help) error = nil")
	}
	if err := (Binding{}).Validate(); err == nil || (Binding{}).Matches(key) {
		t.Fatal("zero binding did not fail closed")
	}
}

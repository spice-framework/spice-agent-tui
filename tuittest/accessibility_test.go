package tuittest

import (
	"testing"
	"unicode/utf8"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestValidateAccessibilityRejectsTerminalControlCorpus(t *testing.T) {
	t.Parallel()
	controls := []struct {
		name  string
		value string
	}{
		{name: "CSI", value: "\x1b[31m"},
		{name: "OSC", value: "\x1b]0;title\x07"},
		{name: "C1 CSI", value: "\u009b31m"},
		{name: "C1 OSC", value: "\u009dtitle\u009c"},
		{name: "NUL", value: "\x00"},
		{name: "DEL", value: "\x7f"},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			t.Parallel()
			screen := Screen{
				width: 1, height: 1, styled: control.value, plain: control.value,
				plainLines: []string{control.value}, accessible: true,
				statusLevel: string(agenttui.StatusReady), status: "ready",
			}
			if err := screen.ValidateAccessibility(); err == nil {
				t.Fatal("expected accessible-control rejection")
			}
			if _, err := agenttui.NewText(control.value); err == nil {
				t.Fatal("semantic Text accepted terminal control corpus")
			}
		})
	}
}

func TestValidateAccessibilityRejectsModeAndSemanticViolations(t *testing.T) {
	t.Parallel()
	valid := Screen{
		width: 8, height: 1, styled: "[READY] ready", plain: "[READY] ready",
		plainLines: []string{"[READY] ready"}, accessible: true,
		statusLevel: string(agenttui.StatusReady), status: "ready",
	}
	if err := valid.ValidateAccessibility(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Screen)
	}{
		{name: "normal mode", mutate: func(screen *Screen) { screen.accessible = false }},
		{name: "alternate screen", mutate: func(screen *Screen) { screen.altScreen = true }},
		{name: "cursor control", mutate: func(screen *Screen) { screen.cursorVisible = true }},
		{name: "normalized escape", mutate: func(screen *Screen) { screen.styled = "<ESC>[31m" }},
		{name: "unsupported status", mutate: func(screen *Screen) { screen.statusLevel = "unknown" }},
		{name: "missing status label", mutate: func(screen *Screen) { screen.plain = "ready" }},
		{name: "missing status message", mutate: func(screen *Screen) { screen.status = "different" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.ValidateAccessibility(); err == nil {
				t.Fatal("expected accessibility validation error")
			}
		})
	}
}

func FuzzAccessibleUnicodeCorpus(fuzz *testing.F) {
	for _, seed := range []string{
		"plain ASCII", "e\u0301", "👩‍💻", "诊所界", "שלום مرحبا",
		"ZWJ 👩‍💻 combining e\u0301 CJK 诊所 bidi שלום مرحبا\nsecond\tcolumn",
	} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, value string) {
		if len(value) > 2048 || !utf8.ValidString(value) {
			return
		}
		body, err := agenttui.NewText(value)
		if err != nil {
			return
		}
		title, err := agenttui.NewText("Unicode corpus")
		if err != nil {
			t.Fatal(err)
		}
		sectionTitle, err := agenttui.NewText("Content")
		if err != nil {
			t.Fatal(err)
		}
		section, err := agenttui.NewSection(sectionTitle, body)
		if err != nil {
			return
		}
		workspace, err := agenttui.NewWorkspace(title, []agenttui.Section{section})
		if err != nil {
			return
		}
		statusText, err := agenttui.NewText("ready")
		if err != nil {
			t.Fatal(err)
		}
		status, err := agenttui.NewStatus(agenttui.StatusReady, statusText, nil)
		if err != nil {
			t.Fatal(err)
		}
		editor, err := agenttui.NewEditor("")
		if err != nil {
			t.Fatal(err)
		}
		view, err := agenttui.NewViewData(workspace, status, editor, nil)
		if err != nil {
			return
		}
		driver, err := NewDriver(Options{Width: 60, Height: 16, Accessible: true, Initial: &view})
		if err != nil {
			t.Fatal(err)
		}
		defer driver.Close()
		screen, err := driver.Screen("fuzz-accessible")
		if err != nil {
			t.Fatal(err)
		}
		if err := screen.ValidateAccessibility(); err != nil {
			t.Fatal(err)
		}
	})
}

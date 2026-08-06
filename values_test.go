package agenttui

import (
	"strings"
	"testing"
)

func TestTextValidation(t *testing.T) {
	t.Parallel()
	valid, err := NewText("orders\nready\t✓")
	if err != nil || valid.String() != "orders\nready\t✓" {
		t.Fatalf("NewText() = %q, %v", valid.String(), err)
	}
	for _, value := range []string{"escape\x1b[31m", "nul\x00", string([]byte{0xff}), strings.Repeat("x", MaximumTextBytes+1)} {
		if _, err := NewText(value); err == nil {
			t.Fatalf("NewText(%q) error = nil", value[:min(len(value), 20)])
		}
	}
	if err := (Text{value: "\x1b"}).Validate(); err == nil {
		t.Fatal("Text.Validate(escape) error = nil")
	}
}

func TestSizeAndFrameBounds(t *testing.T) {
	t.Parallel()
	valid, err := NewSize(80, 24)
	if err != nil || valid.Width() != 80 || valid.Height() != 24 {
		t.Fatalf("NewSize() = %#v, %v", valid, err)
	}
	for _, dimensions := range [][2]int{{0, 1}, {1, 0}, {MaximumWidth + 1, 1}, {1, MaximumHeight + 1}} {
		if _, sizeErr := NewSize(dimensions[0], dimensions[1]); sizeErr == nil {
			t.Fatalf("NewSize%v error = nil", dimensions)
		}
	}
	bounded := BoundedSize(-1, MaximumHeight+20)
	if bounded.Width() != 1 || bounded.Height() != MaximumHeight {
		t.Fatalf("BoundedSize() = %dx%d", bounded.Width(), bounded.Height())
	}
	frame, err := NewFrame("frame", valid)
	if err != nil || frame.Content() != "frame" || frame.Size() != valid {
		t.Fatalf("NewFrame() = %#v, %v", frame, err)
	}
	if _, frameErr := NewFrame(strings.Repeat("x", MaximumFrameBytes+1), valid); frameErr == nil {
		t.Fatal("NewFrame(oversize) error = nil")
	}
	if _, frameErr := NewFrame("frame", Size{}); frameErr == nil {
		t.Fatal("NewFrame(zero size) error = nil")
	}
	styled, err := NewFrame("\x1b[31merror\x1b[0m", valid)
	if err != nil || styled.PlainContent() != "error" {
		t.Fatalf("Frame.PlainContent() = %q, %v", styled.PlainContent(), err)
	}
	withCursor, err := styled.WithCursor(79, 23)
	x, y, visible := withCursor.Cursor()
	if err != nil || x != 79 || y != 23 || !visible {
		t.Fatalf("Frame.Cursor() = %d,%d,%t, %v", x, y, visible, err)
	}
	if _, cursorErr := styled.WithCursor(80, 23); cursorErr == nil {
		t.Fatal("Frame.WithCursor(outside) error = nil")
	}
}

func TestWorkspaceAndStatusAreImmutable(t *testing.T) {
	t.Parallel()
	title := mustText(t, "Commerce")
	section, err := NewSection(mustText(t, "Orders"), mustText(t, "No pending orders"))
	if err != nil {
		t.Fatal(err)
	}
	sections := []Section{section}
	workspace, err := NewWorkspace(title, sections)
	if err != nil {
		t.Fatal(err)
	}
	sections[0] = Section{}
	copySections := workspace.Sections()
	copySections[0] = Section{}
	if workspace.Title() != title || workspace.Sections()[0].Title().String() != "Orders" {
		t.Fatal("workspace retained caller mutation")
	}
	if _, sectionErr := NewSection(Text{}, Text{}); sectionErr == nil {
		t.Fatal("NewSection(empty title) error = nil")
	}
	if _, workspaceErr := NewWorkspace(mustText(t, "bad\nname"), nil); workspaceErr == nil {
		t.Fatal("NewWorkspace(multiline title) error = nil")
	}
	if _, workspaceErr := NewWorkspace(title, make([]Section, maximumSections+1)); workspaceErr == nil {
		t.Fatal("NewWorkspace(oversize) error = nil")
	}
	if validationErr := (WorkspaceState{title: title, sections: []Section{{}}}).Validate(); validationErr == nil {
		t.Fatal("WorkspaceState.Validate(zero section) error = nil")
	}

	hints := []Text{mustText(t, "ctrl+c quit")}
	status, err := NewStatus(StatusReady, mustText(t, "ready"), hints)
	if err != nil {
		t.Fatal(err)
	}
	hints[0] = Text{}
	copyHints := status.Hints()
	copyHints[0] = Text{}
	if status.Level() != StatusReady || status.Message().String() != "ready" || status.Hints()[0].String() == "" {
		t.Fatal("status retained caller mutation")
	}
	if _, err := NewStatus("unknown", Text{}, nil); err == nil {
		t.Fatal("NewStatus(unknown) error = nil")
	}
	if _, err := NewStatus(StatusReady, mustText(t, "bad\nstatus"), nil); err == nil {
		t.Fatal("NewStatus(multiline) error = nil")
	}
	if _, err := NewStatus(StatusReady, Text{}, make([]Text, maximumHints+1)); err == nil {
		t.Fatal("NewStatus(oversize hints) error = nil")
	}
	if err := (StatusState{level: StatusReady, hints: []Text{{value: "\x1b"}}}).Validate(); err == nil {
		t.Fatal("StatusState.Validate(unsafe hint) error = nil")
	}
	for _, level := range []StatusLevel{StatusDisconnected, StatusReconnecting, StatusError} {
		if _, err := NewStatus(level, mustText(t, string(level)), nil); err != nil {
			t.Fatalf("NewStatus(%q) error = %v", level, err)
		}
	}
}

func TestThemesExposeStableSemanticPalettes(t *testing.T) {
	t.Parallel()
	light := LightTheme()
	dark := DarkTheme()
	if light.Name() != "spice-light" || light.Mode() != ThemeLight || dark.Name() != "spice-dark" || dark.Mode() != ThemeDark {
		t.Fatalf("built-in themes = %#v, %#v", light, dark)
	}
	if light.Palette().Foreground() == dark.Palette().Foreground() {
		t.Fatal("light and dark foregrounds must differ")
	}
	red, green, blue := NewColor(1, 2, 3).RGB()
	if red != 1 || green != 2 || blue != 3 {
		t.Fatalf("RGB() = %d,%d,%d", red, green, blue)
	}
	palette := NewPalette(NewColor(1, 1, 1), NewColor(2, 2, 2), NewColor(3, 3, 3), NewColor(4, 4, 4), NewColor(5, 5, 5))
	theme, err := NewTheme("custom", ThemeDark, palette)
	if err != nil || theme.Palette().Accent() != palette.Accent() || theme.Palette().Warning() != palette.Warning() || theme.Palette().Failure() != palette.Failure() || theme.Palette().Muted() != palette.Muted() {
		t.Fatalf("NewTheme() = %#v, %v", theme, err)
	}
	for _, name := range []string{"", " spaced ", "bad\nname"} {
		if _, err := NewTheme(name, ThemeLight, palette); err == nil {
			t.Fatalf("NewTheme(%q) error = nil", name)
		}
	}
	if _, err := NewTheme("custom", "unknown", palette); err == nil {
		t.Fatal("NewTheme(unknown mode) error = nil")
	}
	if err := (ThemeState{}).Validate(); err == nil {
		t.Fatal("ThemeState.Validate(zero) error = nil")
	}
}

func TestViewDataBoundsAndDefensiveCopies(t *testing.T) {
	t.Parallel()
	workspace := mustWorkspace(t, nil)
	status := mustStatus(t, StatusBusy)
	editor, err := NewEditor("inspect")
	if err != nil {
		t.Fatal(err)
	}
	activity := []Text{mustText(t, "started")}
	view, err := NewViewData(workspace, status, editor, activity)
	if err != nil {
		t.Fatal(err)
	}
	activity[0] = Text{}
	copyActivity := view.Activity()
	copyActivity[0] = Text{}
	if view.Workspace().Title().String() != "Spice" || view.Status().Level() != StatusBusy || view.Prompt().Value().String() != "inspect" || view.Activity()[0].String() != "started" {
		t.Fatal("view retained caller mutation")
	}
	if _, viewErr := NewViewData(workspace, status, editor, make([]Text, MaximumActivityItems+1)); viewErr == nil {
		t.Fatal("NewViewData(oversize activity) error = nil")
	}
	if _, viewErr := NewViewData(WorkspaceState{}, status, editor, nil); viewErr == nil {
		t.Fatal("NewViewData(zero workspace) error = nil")
	}
	if _, viewErr := NewViewData(workspace, StatusState{}, editor, nil); viewErr == nil {
		t.Fatal("NewViewData(zero status) error = nil")
	}
	if validationErr := (ViewData{}).Validate(); validationErr == nil {
		t.Fatal("ViewData.Validate(zero) error = nil")
	}
	body := mustText(t, strings.Repeat("x", MaximumTextBytes))
	sections := make([]Section, 9)
	for index := range sections {
		sections[index], err = NewSection(mustText(t, "part"), body)
		if err != nil {
			t.Fatal(err)
		}
	}
	large := mustWorkspace(t, sections)
	if _, err := NewViewData(large, status, editor, nil); err == nil {
		t.Fatal("NewViewData(aggregate oversize) error = nil")
	}
}

func mustText(t *testing.T, value string) Text {
	t.Helper()
	text, err := NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func mustWorkspace(t *testing.T, sections []Section) WorkspaceState {
	t.Helper()
	workspace, err := NewWorkspace(mustText(t, "Spice"), sections)
	if err != nil {
		t.Fatal(err)
	}
	return workspace
}

func mustStatus(t *testing.T, level StatusLevel) StatusState {
	t.Helper()
	status, err := NewStatus(level, mustText(t, "ready"), []Text{mustText(t, "ctrl+c quit")})
	if err != nil {
		t.Fatal(err)
	}
	return status
}

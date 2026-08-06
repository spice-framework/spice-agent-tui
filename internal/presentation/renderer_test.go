package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestFixedRendererMatchesThemeGoldensAndDimensions(t *testing.T) {
	t.Parallel()
	data := fixtureView(t)
	size, err := agenttui.NewSize(48, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		theme agenttui.ThemeState
	}{
		{name: "light", theme: agenttui.LightTheme()},
		{name: "dark", theme: agenttui.DarkTheme()},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame, renderErr := (FixedRenderer{}).Render(data, size, test.theme)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			lines := strings.Split(frame.Content(), "\n")
			if len(lines) != size.Height() {
				t.Fatalf("rendered lines = %d, want %d", len(lines), size.Height())
			}
			for index, line := range lines {
				if width := ansi.StringWidth(line); width != size.Width() {
					t.Fatalf("line %d width = %d, want %d", index, width, size.Width())
				}
			}
			golden, readErr := os.ReadFile(filepath.Join("testdata", test.name+".golden"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got, want := normalizeFrame(frame.Content()), strings.TrimSuffix(string(golden), "\n"); got != want {
				t.Fatalf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestFixedRendererBoundsSmallFramesAndLongLines(t *testing.T) {
	t.Parallel()
	data := fixtureView(t)
	for _, size := range []agenttui.Size{agenttui.BoundedSize(1, 1), agenttui.BoundedSize(12, 2), agenttui.BoundedSize(18, 3)} {
		frame, err := (FixedRenderer{}).Render(data, size, agenttui.DarkTheme())
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(frame.Content(), "\n")
		if len(lines) != size.Height() {
			t.Fatalf("size %dx%d rendered %d lines", size.Width(), size.Height(), len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != size.Width() {
				t.Fatalf("rendered width = %d, want %d", ansi.StringWidth(line), size.Width())
			}
		}
	}
}

func TestFixedRendererRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()
	if _, err := (FixedRenderer{}).Render(fixtureView(t), agenttui.Size{}, agenttui.DarkTheme()); err == nil {
		t.Fatal("Render(zero size) error = nil")
	}
	if _, err := (FixedRenderer{}).Render(fixtureView(t), agenttui.BoundedSize(80, 24), nil); err == nil {
		t.Fatal("Render(nil theme) error = nil")
	}
}

func fixtureView(t *testing.T) agenttui.ViewData {
	t.Helper()
	sections := []agenttui.Section{
		mustSection(t, "Summary", "2 owners\n1 pet"),
		mustSection(t, "Notes", "No alerts"),
	}
	workspace, err := agenttui.NewWorkspace(mustText(t, "PetClinic"), sections)
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusWarning, mustText(t, "disconnected"), []agenttui.Text{
		mustText(t, "ctrl+c quit"), mustText(t, "r retry"),
	})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := agenttui.NewEditor("owner")
	if err != nil {
		t.Fatal(err)
	}
	data, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{
		mustText(t, "visit scheduled"), mustText(t, "email queued"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustSection(t *testing.T, title, body string) agenttui.Section {
	t.Helper()
	section, err := agenttui.NewSection(mustText(t, title), mustText(t, body))
	if err != nil {
		t.Fatal(err)
	}
	return section
}

func mustText(t *testing.T, value string) agenttui.Text {
	t.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func normalizeFrame(content string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		lines[index] = strings.ReplaceAll(strings.TrimRight(line, " "), "\x1b", "<ESC>")
	}
	return strings.Join(lines, "\n")
}

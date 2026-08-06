package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/x/ansi"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestFixedRendererMatchesThemeGoldensAndDimensions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		theme         agenttui.ThemeState
		data          agenttui.ViewData
		size          agenttui.Size
		cursorX       int
		cursorY       int
		cursorVisible bool
	}{
		{name: "light", theme: agenttui.LightTheme(), data: fixtureView(t), size: agenttui.BoundedSize(48, 10), cursorX: 7, cursorY: 8, cursorVisible: true},
		{name: "dark", theme: agenttui.DarkTheme(), data: fixtureView(t), size: agenttui.BoundedSize(48, 10), cursorX: 7, cursorY: 8, cursorVisible: true},
		{name: "compact-light", theme: agenttui.LightTheme(), data: unicodeView(t), size: agenttui.BoundedSize(24, 6), cursorX: 8, cursorY: 4, cursorVisible: true},
		{name: "compact-dark", theme: agenttui.DarkTheme(), data: unicodeView(t), size: agenttui.BoundedSize(24, 6), cursorX: 8, cursorY: 4, cursorVisible: true},
		{name: "tiny-light", theme: agenttui.LightTheme(), data: fixtureView(t), size: agenttui.BoundedSize(1, 1)},
		{name: "tiny-dark", theme: agenttui.DarkTheme(), data: fixtureView(t), size: agenttui.BoundedSize(1, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame, renderErr := (FixedRenderer{}).Render(test.data, test.size, test.theme)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			lines := strings.Split(frame.Content(), "\n")
			if len(lines) != test.size.Height() {
				t.Fatalf("rendered lines = %d, want %d", len(lines), test.size.Height())
			}
			for index, line := range lines {
				if width := ansi.StringWidth(line); width != test.size.Width() {
					t.Fatalf("line %d width = %d, want %d", index, width, test.size.Width())
				}
			}
			x, y, visible := frame.Cursor()
			if x != test.cursorX || y != test.cursorY || visible != test.cursorVisible {
				t.Fatalf("cursor = %d,%d,%t, want %d,%d,%t", x, y, visible, test.cursorX, test.cursorY, test.cursorVisible)
			}
			if strings.Contains(frame.PlainContent(), "\x1b") {
				t.Fatal("plain frame contains terminal styling")
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

func TestFixedRendererIsConcurrentAndDeterministic(t *testing.T) {
	t.Parallel()
	renderer := FixedRenderer{}
	data := unicodeView(t)
	size := agenttui.BoundedSize(32, 7)
	want, err := renderer.Render(data, size, agenttui.DarkTheme())
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 32 {
		wait.Go(func() {
			for range 20 {
				got, renderErr := renderer.Render(data, size, agenttui.DarkTheme())
				if renderErr != nil || got.Content() != want.Content() {
					t.Errorf("concurrent Render() differs: %v", renderErr)
					return
				}
			}
		})
	}
	wait.Wait()
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

func TestRenderStatusAlwaysNamesConnectionAndErrorStates(t *testing.T) {
	t.Parallel()
	for _, level := range []agenttui.StatusLevel{
		agenttui.StatusDisconnected,
		agenttui.StatusReconnecting,
		agenttui.StatusError,
	} {
		status, err := agenttui.NewStatus(level, mustText(t, "detail"), nil)
		if err != nil {
			t.Fatal(err)
		}
		plain := ansi.Strip(renderStatus(status, agenttui.DarkTheme().Palette()))
		want := "[" + strings.ToUpper(string(level)) + "] detail"
		if plain != want {
			t.Fatalf("renderStatus(%q) = %q, want %q", level, plain, want)
		}
	}
}

func TestPromptCursorRemainsVisibleForLongWideUnicode(t *testing.T) {
	t.Parallel()
	editor, err := agenttui.NewEditor(strings.Repeat("界", 20) + "e\u0301")
	if err != nil {
		t.Fatal(err)
	}
	line, cursorX := renderPrompt(editor, agenttui.DarkTheme().Palette(), 12)
	if ansi.StringWidth(line) > 12 || cursorX < 0 || cursorX >= 12 || strings.Contains(ansi.Strip(line), "�") {
		t.Fatalf("renderPrompt() width=%d cursor=%d content=%q", ansi.StringWidth(line), cursorX, ansi.Strip(line))
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
		mustText(t, "visit scheduled"), mustText(t, "email queued"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func unicodeView(t *testing.T) agenttui.ViewData {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(mustText(t, "诊所 🐾"), []agenttui.Section{
		mustSection(t, "状态", "e\u0301 ready\n👩‍💻 stream"),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusReconnecting, mustText(t, "attempt 2 of 3"), []agenttui.Text{
		mustText(t, "ctrl+c cancel"),
	})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := agenttui.NewEditor("ab界👩‍💻")
	if err != nil {
		t.Fatal(err)
	}
	data, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{mustText(t, "响应中…")})
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

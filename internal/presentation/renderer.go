package presentation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
)

// FixedRenderer deterministically fills every terminal cell in a bounded
// semantic frame. It performs no I/O and retains no render state.
type FixedRenderer struct{}

// Render implements agenttui.Renderer.
func (FixedRenderer) Render(data agenttui.ViewData, size agenttui.Size, theme agenttui.Theme) (agenttui.Frame, error) {
	if theme == nil {
		return agenttui.Frame{}, errors.New("render theme must not be nil")
	}
	if err := size.Validate(); err != nil {
		return agenttui.Frame{}, fmt.Errorf("render size: %w", err)
	}
	lines := make([]string, size.Height())
	palette := theme.Palette()
	if size.Height() > 0 {
		lines[0] = styled(palette.Accent(), true, data.Workspace().Title().String())
	}
	bodyEnd := size.Height()
	if size.Height() >= 2 {
		bodyEnd--
		lines[bodyEnd] = renderStatus(data.Status(), palette)
	}
	if size.Height() >= 3 {
		bodyEnd--
		lines[bodyEnd] = renderPrompt(data.Prompt(), palette)
	}
	body := semanticBody(data, palette)
	for index := 1; index < bodyEnd && index-1 < len(body); index++ {
		lines[index] = body[index-1]
	}
	for index := range lines {
		lines[index] = fitLine(lines[index], size.Width())
	}
	return agenttui.NewFrame(strings.Join(lines, "\n"), size)
}

func semanticBody(data agenttui.ViewData, palette agenttui.Palette) []string {
	result := make([]string, 0)
	for _, section := range data.Workspace().Sections() {
		bodyLines := splitSemanticLines(section.Body().String())
		if len(bodyLines) == 0 {
			bodyLines = []string{""}
		}
		result = append(result, styled(palette.Foreground(), true, section.Title().String()+": ")+
			styled(palette.Foreground(), false, bodyLines[0]))
		for _, line := range bodyLines[1:] {
			result = append(result, styled(palette.Foreground(), false, "  "+line))
		}
	}
	if activity := data.Activity(); len(activity) > 0 {
		result = append(result, styled(palette.Muted(), true, "Activity"))
		for _, item := range activity {
			for _, line := range splitSemanticLines(item.String()) {
				result = append(result, styled(palette.Foreground(), false, "• "+line))
			}
		}
	}
	return result
}

func splitSemanticLines(value string) []string {
	value = strings.ReplaceAll(value, "\t", "    ")
	return strings.Split(value, "\n")
}

func renderPrompt(editor agenttui.EditorState, palette agenttui.Palette) string {
	value := []rune(editor.Value().String())
	cursor := editor.Cursor()
	withCursor := make([]rune, 0, len(value)+1)
	withCursor = append(withCursor, value[:cursor]...)
	withCursor = append(withCursor, '│')
	withCursor = append(withCursor, value[cursor:]...)
	return styled(palette.Accent(), true, "> ") + styled(palette.Foreground(), false, string(withCursor))
}

func renderStatus(status agenttui.StatusState, palette agenttui.Palette) string {
	color := palette.Muted()
	switch status.Level() {
	case agenttui.StatusBusy:
		color = palette.Accent()
	case agenttui.StatusWarning:
		color = palette.Warning()
	case agenttui.StatusError:
		color = palette.Failure()
	}
	result := styled(color, true, strings.ToUpper(string(status.Level()))+" "+status.Message().String())
	if hints := status.Hints(); len(hints) > 0 {
		values := make([]string, len(hints))
		for index, hint := range hints {
			values[index] = hint.String()
		}
		result += styled(palette.Muted(), false, "  "+strings.Join(values, " · "))
	}
	return result
}

func styled(color agenttui.Color, bold bool, value string) string {
	red, green, blue := color.RGB()
	prefix := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", red, green, blue)
	if bold {
		prefix += ansiBold
	}
	return prefix + value + ansiReset
}

func fitLine(value string, width int) string {
	value = ansi.Truncate(value, width, "")
	return value + strings.Repeat(" ", max(width-ansi.StringWidth(value), 0))
}

var _ agenttui.Renderer = FixedRenderer{}

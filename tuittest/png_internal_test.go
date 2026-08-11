package tuittest

import (
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestRenderPNGRejectsInvalidScreensAndOptions(t *testing.T) {
	t.Parallel()
	valid := Screen{width: 1, height: 1, plainLines: []string{" "}}
	tests := []struct {
		name    string
		screen  Screen
		options PNGOptions
	}{
		{name: "zero screen", screen: Screen{}},
		{name: "too wide", screen: Screen{width: agenttui.MaximumWidth + 1, height: 1, plainLines: []string{""}}},
		{name: "too tall", screen: Screen{width: 1, height: agenttui.MaximumHeight + 1, plainLines: []string{""}}},
		{name: "line mismatch", screen: Screen{width: 1, height: 2, plainLines: []string{""}}},
		{name: "bad theme", screen: valid, options: PNGOptions{ThemeMode: agenttui.ThemeMode("sepia"), Scale: 1}},
		{name: "negative scale", screen: valid, options: PNGOptions{Scale: -1}},
		{name: "large scale", screen: valid, options: PNGOptions{Scale: maximumPNGScale + 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := RenderPNG(test.screen, test.options); err == nil {
				t.Fatal("expected PNG validation error")
			}
		})
	}
}

func TestRenderPNGBoundaryScreen(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ width, height int }{{width: 1, height: 1}, {width: 120, height: 40}} {
		lines := make([]string, size.height)
		for index := range lines {
			lines[index] = strings.Repeat(" ", size.width)
		}
		screen := Screen{width: size.width, height: size.height, plainLines: lines}
		if _, err := RenderPNG(screen, PNGOptions{}); err != nil {
			t.Fatalf("RenderPNG(%dx%d): %v", size.width, size.height, err)
		}
	}
}

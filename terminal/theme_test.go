package terminal

import (
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestSnapshotThemeReadsSPIOnce(t *testing.T) {
	t.Parallel()
	theme := &mutableTheme{name: "application-theme", mode: agenttui.ThemeDark}
	snapshot, err := snapshotTheme(theme)
	if err != nil {
		t.Fatal(err)
	}
	theme.name = "changed"
	theme.mode = agenttui.ThemeLight
	if snapshot.Name() != "application-theme" || snapshot.Mode() != agenttui.ThemeDark {
		t.Fatalf("theme snapshot changed with source = %#v", snapshot)
	}
}

type mutableTheme struct {
	name string
	mode agenttui.ThemeMode
}

func (theme *mutableTheme) Name() string             { return theme.name }
func (theme *mutableTheme) Mode() agenttui.ThemeMode { return theme.mode }
func (*mutableTheme) Palette() agenttui.Palette      { return agenttui.Palette{} }

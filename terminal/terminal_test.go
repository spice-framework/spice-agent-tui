package terminal_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/terminal"
)

var (
	_ func() agenttui.Renderer = terminal.NewFixedRenderer
	_ func(
		agenttui.Session,
		agenttui.Renderer,
		agenttui.Theme,
		[]agenttui.KeyBinding,
		agenttui.ViewData,
		agenttui.TerminalIO,
		agenttui.TerminalConfig,
	) (agenttui.Shell, error) = terminal.NewShell
)

func TestFacadeConstructsOnlyFromPublicContracts(t *testing.T) {
	t.Parallel()
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	streams, err := agenttui.NewTerminalIO(bytes.NewBuffer(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	shell, err := terminal.NewShell(
		facadeSession{},
		terminal.NewFixedRenderer(),
		agenttui.DarkTheme(),
		bindings,
		facadeInitialView(t),
		streams,
		agenttui.NewTerminalConfig(true),
	)
	if err != nil || shell == nil {
		t.Fatalf("NewShell() = %#v, %v", shell, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := shell.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(cancelled) error = %v", err)
	}
}

func TestFacadeSnapshotsThemeAndRejectsPanics(t *testing.T) {
	t.Parallel()
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	streams, err := agenttui.NewTerminalIO(bytes.NewBuffer(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, theme := range []agenttui.Theme{nil, panicTheme{}} {
		if _, err := terminal.NewShell(
			facadeSession{}, terminal.NewFixedRenderer(), theme, bindings,
			facadeInitialView(t), streams, agenttui.NewTerminalConfig(false),
		); err == nil {
			t.Fatalf("NewShell(theme=%T) error = nil", theme)
		}
	}
}

type facadeSession struct{}

func (facadeSession) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	<-ctx.Done()
	return agenttui.SessionUpdate{}, context.Cause(ctx)
}

func (facadeSession) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	return agenttui.CommandResult{}, errors.New("not called")
}

type panicTheme struct{}

func (panicTheme) Name() string              { panic("secret") }
func (panicTheme) Mode() agenttui.ThemeMode  { return agenttui.ThemeDark }
func (panicTheme) Palette() agenttui.Palette { return agenttui.Palette{} }

func facadeInitialView(t *testing.T) agenttui.ViewData {
	t.Helper()
	title, err := agenttui.NewText("Spice Agent")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := agenttui.NewWorkspace(title, nil)
	if err != nil {
		t.Fatal(err)
	}
	message, err := agenttui.NewText("connecting")
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusReconnecting, message, nil)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := agenttui.NewEditor("")
	if err != nil {
		t.Fatal(err)
	}
	view, err := agenttui.NewViewData(workspace, status, prompt, nil)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

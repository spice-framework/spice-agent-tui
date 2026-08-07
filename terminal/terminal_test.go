package terminal_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

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

func TestFacadeRunsOneInteractiveTerminalLifecycle(t *testing.T) {
	t.Parallel()
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	cancelMessage, err := agenttui.NewText("cancellation requested")
	if err != nil {
		t.Fatal(err)
	}
	cancelResult, err := agenttui.NewCommandResult(cancelMessage, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := &interactiveSession{
		receiveStarted: make(chan struct{}),
		receiveStopped: make(chan struct{}),
		submitIntent:   make(chan agenttui.Intent, 1),
		submitStopped:  make(chan struct{}),
		cancelIntent:   make(chan agenttui.Intent, 1),
		cancelResult:   cancelResult,
	}
	input, writer := io.Pipe()
	t.Cleanup(func() {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("close terminal input: %v", closeErr)
		}
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("close terminal input writer: %v", closeErr)
		}
	})
	var output bytes.Buffer
	streams, err := agenttui.NewTerminalIO(input, &output)
	if err != nil {
		t.Fatal(err)
	}
	shell, err := terminal.NewShell(
		session,
		terminal.NewFixedRenderer(),
		agenttui.DarkTheme(),
		bindings,
		facadeInitialView(t),
		streams,
		agenttui.NewTerminalConfig(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	runResult := make(chan error, 1)
	go func() { runResult <- shell.Run(ctx) }()
	waitForSignal(t, session.receiveStarted, "session receive start")

	if _, err := writer.Write([]byte("ship it\r")); err != nil {
		t.Fatal(err)
	}
	submit := waitForIntent(t, session.submitIntent, "submit intent")
	if values := submit.Values(); submit.Kind() != agenttui.IntentSubmit ||
		len(values) != 1 || values[0].String() != "ship it" {
		t.Fatalf("submit intent = kind %q, values %v", submit.Kind(), values)
	}

	// Ctrl-X must remain available while the ordinary submit lane is blocked.
	if _, err := writer.Write([]byte{'\x18'}); err != nil {
		t.Fatal(err)
	}
	cancelIntent := waitForIntent(t, session.cancelIntent, "cancel intent")
	if cancelIntent.Kind() != agenttui.IntentCancelActiveRun || len(cancelIntent.Values()) != 0 {
		t.Fatalf("cancel intent = kind %q, values %v", cancelIntent.Kind(), cancelIntent.Values())
	}

	// Ctrl-Q follows the second standard quit binding and must cancel every
	// shell-owned operation before Bubble Tea reports a normal exit.
	if _, err := writer.Write([]byte{'\x11'}); err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, session.receiveStopped, "session receive cancellation")
	waitForSignal(t, session.submitStopped, "submit cancellation")
	select {
	case runErr := <-runResult:
		if runErr != nil {
			t.Fatalf("Run(interactive terminal) error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interactive terminal did not stop after Ctrl-Q")
	}
	if output.Len() == 0 {
		t.Fatal("interactive terminal produced no Bubble Tea output")
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

type interactiveSession struct {
	receiveStarted chan struct{}
	receiveStopped chan struct{}
	submitIntent   chan agenttui.Intent
	submitStopped  chan struct{}
	cancelIntent   chan agenttui.Intent
	cancelResult   agenttui.CommandResult
}

func (session *interactiveSession) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	close(session.receiveStarted)
	<-ctx.Done()
	close(session.receiveStopped)
	return agenttui.SessionUpdate{}, context.Cause(ctx)
}

func (session *interactiveSession) Perform(
	ctx context.Context,
	intent agenttui.Intent,
) (agenttui.CommandResult, error) {
	switch intent.Kind() {
	case agenttui.IntentSubmit:
		select {
		case session.submitIntent <- intent:
		case <-ctx.Done():
			return agenttui.CommandResult{}, context.Cause(ctx)
		}
		<-ctx.Done()
		close(session.submitStopped)
		return agenttui.CommandResult{}, context.Cause(ctx)
	case agenttui.IntentCancelActiveRun:
		select {
		case session.cancelIntent <- intent:
			return session.cancelResult, nil
		case <-ctx.Done():
			return agenttui.CommandResult{}, context.Cause(ctx)
		}
	default:
		return agenttui.CommandResult{}, errors.New("unexpected interactive intent")
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func waitForIntent(
	t *testing.T,
	intents <-chan agenttui.Intent,
	name string,
) agenttui.Intent {
	t.Helper()
	select {
	case intent := <-intents:
		return intent
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
		return agenttui.Intent{}
	}
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

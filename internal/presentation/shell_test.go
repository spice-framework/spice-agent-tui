package presentation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestShellRejectsMissingOutputAndContext(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	terminal := mustTerminalIO(t, bytes.NewBuffer(nil), &bytes.Buffer{})
	config := mustTerminalConfig(t, true)
	if _, err := NewShell(Model{}, terminal, config); err == nil {
		t.Fatal("NewShell(zero model) error = nil")
	}
	if _, err := NewShell(model, agenttui.TerminalIO{}, config); err == nil {
		t.Fatal("NewShell(zero terminal) error = nil")
	}
	shell, err := NewShell(model, terminal, config)
	if err != nil {
		t.Fatal(err)
	}
	if concrete, ok := shell.(*bubbleShell); !ok || !concrete.model.accessible {
		t.Fatal("terminal accessible configuration was not applied")
	}
	if err := shell.Run(nil); err == nil { //nolint:staticcheck // This public boundary must reject a nil context.
		t.Fatal("Run(nil context) error = nil")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := shell.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(cancelled) error = %v", err)
	}
}

func TestShellHonorsCancellationDuringRun(t *testing.T) {
	model := fixtureModel(t, FixedRenderer{})
	var output bytes.Buffer
	shell, err := NewShell(model, mustTerminalIO(t, bytes.NewBuffer(nil), &output), mustTerminalConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- shell.Run(ctx) }()
	cancel()
	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run() error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not honor cancellation")
	}
}

func TestShellCancellationUnblocksOneActiveReceive(t *testing.T) {
	effects := &blockingEffects{started: make(chan struct{}), finished: make(chan struct{})}
	model := fixtureEffectsModel(t, effects, context.Background())
	var output bytes.Buffer
	shell, err := NewShell(
		model,
		mustTerminalIO(t, bytes.NewBuffer(nil), &output),
		mustTerminalConfig(t, false),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	runResult := make(chan error, 1)
	go func() { runResult <- shell.Run(ctx) }()
	select {
	case <-effects.started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("effects receive did not start")
	}
	cancel()
	select {
	case <-effects.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("effects receive leaked after shell cancellation")
	}
	select {
	case runErr := <-runResult:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("Run() error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
	if effects.calls != 1 {
		t.Fatalf("Receive() calls = %d, want 1", effects.calls)
	}
}

func TestShellQuitsCleanlyFromCtrlCInput(t *testing.T) {
	model := fixtureModel(t, FixedRenderer{})
	var output bytes.Buffer
	shell, err := NewShell(model, mustTerminalIO(t, bytes.NewBufferString("\x03"), &output), mustTerminalConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := shell.Run(ctx); err != nil {
		t.Fatalf("Run(ctrl+c) error = %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("Run(ctrl+c) produced no terminal lifecycle output")
	}
}

func TestShellCtrlCCancelsActiveReceiveBeforeNormalExit(t *testing.T) {
	effects := &blockingEffects{started: make(chan struct{}), finished: make(chan struct{})}
	model := fixtureEffectsModel(t, effects, context.Background())
	input, writer := io.Pipe()
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("close input: %v", closeErr)
		}
	}()
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("close input writer: %v", closeErr)
		}
	}()
	var output bytes.Buffer
	shell, err := NewShell(model, mustTerminalIO(t, input, &output), mustTerminalConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	runResult := make(chan error, 1)
	go func() { runResult <- shell.Run(t.Context()) }()
	select {
	case <-effects.started:
	case <-time.After(2 * time.Second):
		t.Fatal("effects receive did not start")
	}
	if _, err := writer.Write([]byte{'\x03'}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-effects.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("Ctrl-C did not cancel the active receive")
	}
	select {
	case runErr := <-runResult:
		if runErr != nil {
			t.Fatalf("Run(ctrl+c) error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run(ctrl+c) did not exit")
	}
	if effects.calls != 1 {
		t.Fatalf("Receive() calls = %d, want 1", effects.calls)
	}
}

func TestShellCtrlCCancelsActivePerformBeforeNormalExit(t *testing.T) {
	effects := &blockingAllEffects{
		receiveStarted:  make(chan struct{}),
		receiveFinished: make(chan struct{}),
		performStarted:  make(chan struct{}),
		performFinished: make(chan struct{}),
	}
	model := fixtureEffectsModel(t, effects, context.Background())
	input, writer := io.Pipe()
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("close input: %v", closeErr)
		}
	}()
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("close input writer: %v", closeErr)
		}
	}()
	var output bytes.Buffer
	shell, err := NewShell(model, mustTerminalIO(t, input, &output), mustTerminalConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	runResult := make(chan error, 1)
	go func() { runResult <- shell.Run(t.Context()) }()
	select {
	case <-effects.receiveStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("effects receive did not start")
	}
	if _, err := writer.Write([]byte{'\r'}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-effects.performStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("semantic perform did not start")
	}
	if _, err := writer.Write([]byte{'\x03'}); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []struct {
		name     string
		finished <-chan struct{}
	}{
		{name: "receive", finished: effects.receiveFinished},
		{name: "perform", finished: effects.performFinished},
	} {
		select {
		case <-operation.finished:
		case <-time.After(2 * time.Second):
			t.Fatalf("Ctrl-C did not cancel active %s", operation.name)
		}
	}
	select {
	case runErr := <-runResult:
		if runErr != nil {
			t.Fatalf("Run(ctrl+c) error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run(ctrl+c) did not exit")
	}
}

func mustTerminalIO(t *testing.T, input io.Reader, output io.Writer) agenttui.TerminalIO {
	t.Helper()
	terminal, err := agenttui.NewTerminalIO(input, output)
	if err != nil {
		t.Fatal(err)
	}
	return terminal
}

func mustTerminalConfig(t *testing.T, accessible bool) agenttui.TerminalConfig {
	t.Helper()
	config, err := agenttui.NewTerminalConfig(accessible, "default", "revision-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

type blockingEffects struct {
	started  chan struct{}
	finished chan struct{}
	calls    int
}

type blockingAllEffects struct {
	receiveStarted  chan struct{}
	receiveFinished chan struct{}
	performStarted  chan struct{}
	performFinished chan struct{}
}

func (effects *blockingAllEffects) Receive(ctx context.Context, _ OperationToken) (tea.Msg, error) {
	close(effects.receiveStarted)
	<-ctx.Done()
	close(effects.receiveFinished)
	return nil, ctx.Err()
}

func (effects *blockingAllEffects) Perform(
	ctx context.Context,
	_ OperationToken,
	_ agenttui.Intent,
) (agenttui.CommandResult, error) {
	close(effects.performStarted)
	<-ctx.Done()
	close(effects.performFinished)
	return agenttui.CommandResult{}, ctx.Err()
}

func (effects *blockingEffects) Receive(ctx context.Context, _ OperationToken) (tea.Msg, error) {
	effects.calls++
	close(effects.started)
	<-ctx.Done()
	close(effects.finished)
	return nil, ctx.Err()
}

func (*blockingEffects) Perform(
	context.Context,
	OperationToken,
	agenttui.Intent,
) (agenttui.CommandResult, error) {
	return agenttui.CommandResult{}, errors.New("unexpected perform")
}

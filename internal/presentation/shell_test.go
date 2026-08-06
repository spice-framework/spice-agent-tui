package presentation

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestShellRejectsMissingOutputAndContext(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	if _, err := NewShell(Model{}, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("NewShell(zero model) error = nil")
	}
	if _, err := NewShell(model, nil, nil); err == nil {
		t.Fatal("NewShell(nil output) error = nil")
	}
	shell, err := NewShell(model, nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
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
	shell, err := NewShell(model, nil, &output)
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

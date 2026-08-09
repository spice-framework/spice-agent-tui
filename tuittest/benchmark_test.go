package tuittest_test

import (
	"context"
	"errors"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

var (
	benchmarkScreen       tuittest.Screen
	benchmarkReceiveError error
)

func BenchmarkSessionEventIngestionAndScreen(b *testing.B) {
	view := benchmarkView(b)
	session := tuittest.NewScriptSession()
	driver, err := tuittest.NewDriver(tuittest.Options{
		Width:   80,
		Height:  24,
		Initial: &view,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(driver.Close)
	b.Cleanup(session.Close)
	ctx := context.Background()
	revision := uint64(1)
	history := []agenttui.Text{
		benchmarkText(b, "show owners"),
		benchmarkText(b, "show visits for Fido"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		snapshot, snapshotErr := agenttui.NewSessionSnapshot(
			revision,
			view.Workspace(),
			view.Status(),
			view.Activity(),
			history,
		)
		if snapshotErr != nil {
			b.Fatal(snapshotErr)
		}
		update, updateErr := agenttui.NewSnapshotUpdate(snapshot)
		if updateErr != nil {
			b.Fatal(updateErr)
		}
		if pushErr := session.PushUpdate(update); pushErr != nil {
			b.Fatal(pushErr)
		}
		received, receiveErr := session.Receive(ctx)
		if receiveErr != nil {
			b.Fatal(receiveErr)
		}
		if injectErr := driver.InjectUpdate(received); injectErr != nil {
			b.Fatal(injectErr)
		}
		screen, screenErr := driver.Screen("session-snapshot")
		if screenErr != nil {
			b.Fatal(screenErr)
		}
		benchmarkScreen = screen
		revision++
	}
}

func BenchmarkRenderScreen(b *testing.B) {
	view := benchmarkView(b)
	options := tuittest.RenderOptions{
		Width:  80,
		Height: 24,
		Theme:  agenttui.DarkTheme(),
		Name:   "benchmark",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		screen, err := tuittest.RenderScreen(view, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkScreen = screen
	}
}

func BenchmarkScriptSessionReceiveCanceled(b *testing.B) {
	session := tuittest.NewScriptSession()
	b.Cleanup(session.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := session.Receive(ctx)
		if !errors.Is(err, context.Canceled) {
			b.Fatalf("Receive() error = %v, want context.Canceled", err)
		}
		benchmarkReceiveError = err
	}
}

func benchmarkView(b *testing.B) agenttui.ViewData {
	b.Helper()
	workspace, err := agenttui.NewWorkspace(benchmarkText(b, "PetClinic"), []agenttui.Section{
		benchmarkSection(b, "Owners", "2 owners\nMary and George"),
		benchmarkSection(b, "Visits", "Fido: annual checkup\nLeo: vaccination"),
	})
	if err != nil {
		b.Fatal(err)
	}
	status, err := agenttui.NewStatus(
		agenttui.StatusReady,
		benchmarkText(b, "ready for prompts"),
		[]agenttui.Text{benchmarkText(b, "enter submit"), benchmarkText(b, "ctrl+c quit")},
	)
	if err != nil {
		b.Fatal(err)
	}
	editor, err := agenttui.NewEditor("find owner 界👩‍💻")
	if err != nil {
		b.Fatal(err)
	}
	view, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{
		benchmarkText(b, "connected to local daemon"),
		benchmarkText(b, "loaded owner and visit tools"),
	})
	if err != nil {
		b.Fatal(err)
	}
	return view
}

func benchmarkSection(b *testing.B, title, body string) agenttui.Section {
	b.Helper()
	section, err := agenttui.NewSection(benchmarkText(b, title), benchmarkText(b, body))
	if err != nil {
		b.Fatal(err)
	}
	return section
}

func benchmarkText(b *testing.B, value string) agenttui.Text {
	b.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		b.Fatal(err)
	}
	return text
}

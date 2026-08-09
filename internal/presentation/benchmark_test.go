package presentation

import (
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

var (
	benchmarkPresentationFrame   agenttui.Frame
	benchmarkPresentationModel   Model
	benchmarkPresentationContent string
)

func BenchmarkModelSnapshotUpdateAndView(b *testing.B) {
	view := benchmarkPresentationView(b)
	bindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		b.Fatal(err)
	}
	model, err := NewModel(
		FixedRenderer{},
		view.Workspace(),
		view.Status(),
		view.Prompt(),
		view.Activity(),
		agenttui.DarkTheme(),
		bindings,
		nil,
	)
	if err != nil {
		b.Fatal(err)
	}
	revision := uint64(1)
	history := []agenttui.Text{
		benchmarkPresentationText(b, "list owners"),
		benchmarkPresentationText(b, "show visits"),
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
		message, messageErr := TestSessionUpdateMessage(update)
		if messageErr != nil {
			b.Fatal(messageErr)
		}
		next, command, applyErr := TestApplyMessage(model, message)
		if applyErr != nil {
			b.Fatal(applyErr)
		}
		if command != nil {
			b.Fatal("snapshot update unexpectedly returned a command")
		}
		model = next
		content, _, _, _, _ := TestViewContent(model)
		benchmarkPresentationModel = model
		benchmarkPresentationContent = content
		revision++
	}
}

func BenchmarkFixedRendererRender(b *testing.B) {
	view := benchmarkPresentationView(b)
	size := agenttui.BoundedSize(80, 24)
	theme := agenttui.DarkTheme()
	renderer := FixedRenderer{}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		frame, err := renderer.Render(view, size, theme)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPresentationFrame = frame
	}
}

func benchmarkPresentationView(b *testing.B) agenttui.ViewData {
	b.Helper()
	workspace, err := agenttui.NewWorkspace(benchmarkPresentationText(b, "PetClinic"), []agenttui.Section{
		benchmarkPresentationSection(b, "Owners", "2 owners\nMary and George"),
		benchmarkPresentationSection(b, "Visits", "Fido: annual checkup\nLeo: vaccination"),
	})
	if err != nil {
		b.Fatal(err)
	}
	status, err := agenttui.NewStatus(
		agenttui.StatusReady,
		benchmarkPresentationText(b, "ready for prompts"),
		[]agenttui.Text{
			benchmarkPresentationText(b, "enter submit"),
			benchmarkPresentationText(b, "ctrl+c quit"),
		},
	)
	if err != nil {
		b.Fatal(err)
	}
	editor, err := agenttui.NewEditor("find owner 界👩‍💻")
	if err != nil {
		b.Fatal(err)
	}
	view, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{
		benchmarkPresentationText(b, "connected to local daemon"),
		benchmarkPresentationText(b, "loaded owner and visit tools"),
	})
	if err != nil {
		b.Fatal(err)
	}
	return view
}

func benchmarkPresentationSection(b *testing.B, title, body string) agenttui.Section {
	b.Helper()
	section, err := agenttui.NewSection(
		benchmarkPresentationText(b, title),
		benchmarkPresentationText(b, body),
	)
	if err != nil {
		b.Fatal(err)
	}
	return section
}

func benchmarkPresentationText(b *testing.B, value string) agenttui.Text {
	b.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		b.Fatal(err)
	}
	return text
}

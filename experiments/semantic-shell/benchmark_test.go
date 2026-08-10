package semanticshell

import (
	"bytes"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func BenchmarkApplyActivityUpdate(b *testing.B) {
	workspace, _ := agenttui.NewWorkspace(benchmarkText("workspace"), nil)
	status, _ := agenttui.NewStatus(agenttui.StatusReady, benchmarkText("ready"), nil)
	snapshot, _ := agenttui.NewSessionSnapshot(1, workspace, status, nil, nil)
	initial, _ := agenttui.NewSnapshotUpdate(snapshot)
	activity := benchmarkText("one deterministic activity item")
	b.ReportAllocs()
	for range b.N {
		state := semanticState{}
		_, _ = state.apply(initial)
		update, _ := agenttui.NewActivityUpdate(2, activity)
		_, _ = state.apply(update)
	}
}

func BenchmarkEncodeSemanticView(b *testing.B) {
	view := semanticView{
		Revision:  1,
		Workspace: semanticWorkspace{Title: "workspace", Sections: []semanticSection{{Title: "status", Body: "ready"}}},
		Status:    semanticStatus{Level: "ready", Message: "connected", Hints: []string{"submit", "cancel"}},
		Activity:  []string{"one", "two"}, PromptHistory: []string{"previous"},
	}
	b.ReportAllocs()
	for range b.N {
		var output bytes.Buffer
		emitter := newEmitter(&output, func() {})
		emitter.view(view)
	}
}

func benchmarkText(value string) agenttui.Text {
	text, err := agenttui.NewText(value)
	if err != nil {
		panic(err)
	}
	return text
}

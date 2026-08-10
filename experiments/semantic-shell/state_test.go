package semanticshell

import (
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestSemanticStateAppliesIncrementalUpdatesAndClonesViews(t *testing.T) {
	t.Parallel()
	state := semanticState{}
	initial, err := state.apply(mustSnapshotUpdate(t, 1))
	if err != nil || initial.Revision != 1 {
		t.Fatalf("initial state = %#v, %v", initial, err)
	}
	activity, err := state.apply(mustActivityUpdate(t, 2, "working"))
	if err != nil || activity.Revision != 2 || len(activity.Activity) != 2 || activity.Activity[1] != "working" {
		t.Fatalf("activity state = %#v, %v", activity, err)
	}
	historyUpdate, err := agenttui.NewPromptHistoryUpdate(3, []agenttui.Text{mustText(t, "previous")})
	if err != nil {
		t.Fatal(err)
	}
	history, err := state.apply(historyUpdate)
	if err != nil || history.Revision != 3 || len(history.PromptHistory) != 1 || history.PromptHistory[0] != "previous" {
		t.Fatalf("history state = %#v, %v", history, err)
	}
	history.Activity[0] = "mutated"
	history.PromptHistory[0] = "mutated"
	copyView := state.view()
	if copyView.Activity[0] == "mutated" || copyView.PromptHistory[0] == "mutated" {
		t.Fatal("semantic view aliases caller slices")
	}
}

func TestSemanticStateBoundsAccumulatedActivity(t *testing.T) {
	t.Parallel()
	state := semanticState{
		initialized: true,
		workspace:   semanticWorkspace{Title: "workspace"},
		status:      semanticStatus{Level: string(agenttui.StatusReady), Message: "ready"},
	}
	item := strings.Repeat("x", agenttui.MaximumTextBytes)
	for range 10 {
		state.activity = append(state.activity, item)
	}
	state.trimActivity()
	if len(state.activity) >= 10 || len(state.activity) == 0 {
		t.Fatalf("bounded activity length = %d", len(state.activity))
	}
}

func TestSemanticStateRejectsInvalidPortableValues(t *testing.T) {
	t.Parallel()
	if _, err := publicWorkspace(semanticWorkspace{}); err == nil {
		t.Fatal("empty workspace accepted")
	}
	if _, err := publicWorkspace(semanticWorkspace{
		Title: "workspace", Sections: []semanticSection{{Title: "invalid\x00", Body: "body"}},
	}); err == nil {
		t.Fatal("invalid section accepted")
	}
	if _, err := publicStatus(semanticStatus{Level: "invalid", Message: "status"}); err == nil {
		t.Fatal("invalid status level accepted")
	}
	if _, err := publicStatus(semanticStatus{
		Level: string(agenttui.StatusReady), Message: "status", Hints: []string{"invalid\x00"},
	}); err == nil {
		t.Fatal("invalid status hint accepted")
	}
}

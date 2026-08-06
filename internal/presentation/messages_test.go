package presentation

import (
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestPresentationMessagesValidateBoundsAndCopyInputs(t *testing.T) {
	t.Parallel()
	view := fixtureView(t)
	activity := view.Activity()
	snapshot, err := NewSnapshotMsg(1, view.Workspace(), view.Status(), activity)
	if err != nil {
		t.Fatal(err)
	}
	activity[0] = mustText(t, "changed")
	if snapshot.activity[0].String() == "changed" {
		t.Fatal("snapshot retained caller activity slice")
	}
	if _, snapshotErr := NewSnapshotMsg(0, view.Workspace(), view.Status(), nil); snapshotErr == nil {
		t.Fatal("NewSnapshotMsg(zero revision) error = nil")
	}
	if _, snapshotErr := NewSnapshotMsg(1, agenttui.WorkspaceState{}, view.Status(), nil); snapshotErr == nil {
		t.Fatal("NewSnapshotMsg(zero workspace) error = nil")
	}
	if _, activityErr := NewActivityMsg(0, mustText(t, "delta")); activityErr == nil {
		t.Fatal("NewActivityMsg(zero revision) error = nil")
	}
	if _, activityErr := NewActivityMsg(1, agenttui.Text{}); activityErr != nil {
		t.Fatalf("empty safe activity should remain valid: %v", activityErr)
	}

	entries := []agenttui.Text{mustText(t, "first"), mustText(t, "second")}
	history, err := NewPromptHistoryMsg(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0] = mustText(t, "changed")
	if history.entries[0].String() != "first" {
		t.Fatal("prompt history retained caller slice")
	}
	if _, historyErr := NewPromptHistoryMsg(make([]agenttui.Text, agenttui.MaximumPromptHistoryItems+1)); historyErr == nil {
		t.Fatal("NewPromptHistoryMsg(oversize) error = nil")
	}
	if _, historyErr := NewPromptHistoryMsg([]agenttui.Text{mustText(t, "bad\nentry")}); historyErr == nil {
		t.Fatal("NewPromptHistoryMsg(multiline) error = nil")
	}
	if _, historyErr := NewPromptHistoryMsg([]agenttui.Text{mustText(t, strings.Repeat("x", agenttui.MaximumPromptBytes+1))}); historyErr == nil {
		t.Fatal("NewPromptHistoryMsg(oversize entry) error = nil")
	}
}

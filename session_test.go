package agenttui_test

import (
	"context"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestSessionValuesAreTaggedBoundedAndDefensivelyCopied(t *testing.T) {
	t.Parallel()
	workspace, status := sessionView(t)
	activity := []agenttui.Text{sessionText(t, "started")}
	history := []agenttui.Text{sessionText(t, "first prompt")}
	snapshot, err := agenttui.NewSessionSnapshot(7, workspace, status, activity, history)
	if err != nil {
		t.Fatal(err)
	}
	activity[0] = sessionText(t, "mutated")
	history[0] = sessionText(t, "mutated")
	if snapshot.Activity()[0].String() != "started" || snapshot.PromptHistory()[0].String() != "first prompt" {
		t.Fatal("session snapshot retained caller-owned slices")
	}

	update, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil || update.Kind() != agenttui.SessionUpdateSnapshot || update.Revision() != 7 {
		t.Fatalf("NewSnapshotUpdate() = %#v, %v", update, err)
	}
	returned, ok := update.Snapshot()
	if !ok || returned.Activity()[0].String() != "started" {
		t.Fatalf("Snapshot() = %#v, %t", returned, ok)
	}
	copyActivity := returned.Activity()
	copyActivity[0] = sessionText(t, "changed")
	again, _ := update.Snapshot()
	if again.Activity()[0].String() != "started" {
		t.Fatal("session update exposed mutable snapshot storage")
	}

	activityUpdate, err := agenttui.NewActivityUpdate(8, sessionText(t, "continued"))
	if err != nil || activityUpdate.Kind() != agenttui.SessionUpdateActivity {
		t.Fatalf("NewActivityUpdate() = %#v, %v", activityUpdate, err)
	}
	if item, present := activityUpdate.Activity(); !present || item.String() != "continued" {
		t.Fatalf("Activity() = %#v, %t", item, present)
	}
	if _, present := activityUpdate.Snapshot(); present {
		t.Fatal("activity update exposed a snapshot payload")
	}

	historyUpdate, err := agenttui.NewPromptHistoryUpdate(9, history)
	if err != nil || historyUpdate.Kind() != agenttui.SessionUpdatePromptHistory {
		t.Fatalf("NewPromptHistoryUpdate() = %#v, %v", historyUpdate, err)
	}
	entries, present := historyUpdate.PromptHistory()
	if !present || len(entries) != 1 {
		t.Fatalf("PromptHistory() = %#v, %t", entries, present)
	}
	entries[0] = sessionText(t, "changed")
	againEntries, againPresent := historyUpdate.PromptHistory()
	if !againPresent || len(againEntries) != 1 || againEntries[0].String() != "mutated" {
		t.Fatal("session update exposed mutable history storage")
	}
}

func TestSessionValuesRejectZeroRevisionsAndOversizedHistory(t *testing.T) {
	t.Parallel()
	workspace, status := sessionView(t)
	if _, err := agenttui.NewSessionSnapshot(0, workspace, status, nil, nil); err == nil {
		t.Fatal("NewSessionSnapshot(zero revision) error = nil")
	}
	if _, err := agenttui.NewActivityUpdate(0, sessionText(t, "item")); err == nil {
		t.Fatal("NewActivityUpdate(zero revision) error = nil")
	}
	history := make([]agenttui.Text, agenttui.MaximumPromptHistoryItems+1)
	if _, err := agenttui.NewPromptHistoryUpdate(1, history); err == nil {
		t.Fatal("NewPromptHistoryUpdate(oversized) error = nil")
	}
}

func TestPublicSessionContractHasNoPresentationDependency(t *testing.T) {
	t.Parallel()
	var session agenttui.Session = sessionStub{}
	if _, err := session.Receive(t.Context()); err == nil {
		t.Fatal("session stub receive error = nil")
	}
	intent, err := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Perform(t.Context(), intent); err == nil {
		t.Fatal("session stub perform error = nil")
	}
}

type sessionStub struct{}

func (sessionStub) Receive(context.Context) (agenttui.SessionUpdate, error) {
	return agenttui.SessionUpdate{}, context.Canceled
}

func (sessionStub) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	return agenttui.CommandResult{}, context.Canceled
}

func sessionView(t *testing.T) (agenttui.WorkspaceState, agenttui.StatusState) {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(sessionText(t, "Spice Agent"), nil)
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(agenttui.StatusReady, sessionText(t, "connected"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, status
}

func sessionText(t *testing.T, value string) agenttui.Text {
	t.Helper()
	text, err := agenttui.NewText(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

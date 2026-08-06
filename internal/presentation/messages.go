package presentation

import (
	"errors"
	"fmt"
	"slices"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

// SnapshotMsg replaces presentation data at a monotonically increasing
// revision. It is client-neutral and performs no I/O.
type SnapshotMsg struct {
	revision  uint64
	workspace agenttui.WorkspaceState
	status    agenttui.StatusState
	activity  []agenttui.Text
}

// NewSnapshotMsg constructs a validated immutable presentation snapshot.
func NewSnapshotMsg(
	revision uint64,
	workspace agenttui.WorkspaceState,
	status agenttui.StatusState,
	activity []agenttui.Text,
) (SnapshotMsg, error) {
	message := SnapshotMsg{
		revision: revision, workspace: workspace, status: status,
		activity: slices.Clone(activity),
	}
	return message, message.Validate()
}

// Validate reports whether the snapshot is bounded and initialized.
func (message SnapshotMsg) Validate() error {
	if message.revision == 0 {
		return errors.New("snapshot revision must be positive")
	}
	empty, err := agenttui.NewEditor("")
	if err != nil {
		return fmt.Errorf("construct snapshot validation editor: %w", err)
	}
	if _, err := agenttui.NewViewData(message.workspace, message.status, empty, message.activity); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	return nil
}

// ActivityMsg appends one streaming presentation item at a monotonically
// increasing revision. The model evicts oldest items to preserve all bounds.
type ActivityMsg struct {
	revision uint64
	item     agenttui.Text
}

// NewActivityMsg constructs one validated streaming activity update.
func NewActivityMsg(revision uint64, item agenttui.Text) (ActivityMsg, error) {
	message := ActivityMsg{revision: revision, item: item}
	return message, message.Validate()
}

// Validate reports whether the update is initialized and terminal-safe.
func (message ActivityMsg) Validate() error {
	if message.revision == 0 {
		return errors.New("activity revision must be positive")
	}
	if err := message.item.Validate(); err != nil {
		return fmt.Errorf("activity item: %w", err)
	}
	return nil
}

// PromptHistoryMsg replaces bounded local prompt history without submitting or
// executing any prompt.
type PromptHistoryMsg struct {
	entries []agenttui.Text
}

// NewPromptHistoryMsg constructs immutable presentation-owned input history.
func NewPromptHistoryMsg(entries []agenttui.Text) (PromptHistoryMsg, error) {
	message := PromptHistoryMsg{entries: slices.Clone(entries)}
	return message, message.Validate()
}

// Validate reports whether every history entry is a safe single-line prompt.
func (message PromptHistoryMsg) Validate() error {
	if len(message.entries) > agenttui.MaximumPromptHistoryItems {
		return fmt.Errorf("prompt history exceeds %d items", agenttui.MaximumPromptHistoryItems)
	}
	for index, entry := range message.entries {
		if _, err := agenttui.NewEditor(entry.String()); err != nil {
			return fmt.Errorf("prompt history item %d: %w", index, err)
		}
	}
	return nil
}

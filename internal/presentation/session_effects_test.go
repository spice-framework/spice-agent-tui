package presentation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestSessionEffectsTranslateUpdatesAndPreserveGlobalRevision(t *testing.T) {
	t.Parallel()
	view := fixtureView(t)
	history := []agenttui.Text{mustText(t, "previous")}
	snapshot, err := agenttui.NewSessionSnapshot(
		1, view.Workspace(), view.Status(), view.Activity(), history,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshotUpdate, err := agenttui.NewSnapshotUpdate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	historyUpdate, err := agenttui.NewPromptHistoryUpdate(2, []agenttui.Text{mustText(t, "newer")})
	if err != nil {
		t.Fatal(err)
	}
	session := &queuedSession{updates: []agenttui.SessionUpdate{snapshotUpdate, historyUpdate}}
	effects, err := NewSessionEffects(session)
	if err != nil {
		t.Fatal(err)
	}
	model := fixtureEffectsModel(t, effects, t.Context())

	first := model.Init()
	if first == nil {
		t.Fatal("session receive was not armed")
	}
	updated, next := model.Update(first())
	model = asModel(t, updated)
	if model.Revision() != 1 || len(model.promptHistory) != 1 || next == nil {
		t.Fatalf("first session update = revision %d history %d next %v", model.Revision(), len(model.promptHistory), next)
	}
	updated, next = model.Update(next())
	model = asModel(t, updated)
	if model.Revision() != 2 || len(model.promptHistory) != 1 || model.promptHistory[0].String() != "newer" || next == nil {
		t.Fatalf("history update = revision %d history %#v next %v", model.Revision(), model.promptHistory, next)
	}
	if session.receiveCalls != 2 {
		t.Fatalf("Receive() calls = %d", session.receiveCalls)
	}
}

func TestSessionEffectsContainPanicsAndNeverRetry(t *testing.T) {
	t.Parallel()
	effects, err := NewSessionEffects(&panicSession{})
	if err != nil {
		t.Fatal(err)
	}
	if _, receiveErr := effects.Receive(t.Context(), 1); receiveErr == nil ||
		receiveErr.Error() != "session operation panicked" || strings.Contains(receiveErr.Error(), "secret") {
		t.Fatalf("Receive() error = %v", receiveErr)
	}
	intent, err := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := effects.Perform(t.Context(), 1, intent); err == nil || err.Error() != "session operation panicked" || strings.Contains(err.Error(), "secret") {
		t.Fatalf("Perform() error = %v", err)
	}
}

func TestSessionEffectsRejectNestedIntentAndPreserveCancellation(t *testing.T) {
	t.Parallel()
	intent, err := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	message, err := agenttui.NewText("unexpected")
	if err != nil {
		t.Fatal(err)
	}
	nested, err := agenttui.NewCommandResult(message, &intent)
	if err != nil {
		t.Fatal(err)
	}
	session := &queuedSession{result: nested}
	effects, err := NewSessionEffects(session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := effects.Perform(t.Context(), 1, intent); err == nil || !strings.Contains(err.Error(), "nested intent") {
		t.Fatalf("Perform(nested result) error = %v", err)
	}
	if session.performCalls != 1 {
		t.Fatalf("Perform() calls = %d, want exactly one", session.performCalls)
	}

	cause := errors.New("caller stopped")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	if _, err := effects.Receive(ctx, 2); !errors.Is(err, cause) {
		t.Fatalf("Receive(cancelled) error = %v", err)
	}
	if _, err := effects.Perform(ctx, 2, intent); !errors.Is(err, cause) {
		t.Fatalf("Perform(cancelled) error = %v", err)
	}
	if session.receiveCalls != 0 || session.performCalls != 1 {
		t.Fatalf("cancelled operations reached session: receive=%d perform=%d", session.receiveCalls, session.performCalls)
	}
}

func TestSessionEffectsPreserveCommittedResultsWhenCancellationRaces(t *testing.T) {
	t.Parallel()
	update, err := agenttui.NewActivityUpdate(1, mustText(t, "committed update"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := agenttui.NewCommandResult(mustText(t, "committed operation"), nil)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	receiveContext, cancelReceive := context.WithCancelCause(t.Context())
	receiveSession := &cancelAfterSuccessSession{cancel: func() { cancelReceive(errors.New("late receive cancellation")) }, update: update}
	effects, err := NewSessionEffects(receiveSession)
	if err != nil {
		t.Fatal(err)
	}
	message, err := effects.Receive(receiveContext, 1)
	if err != nil {
		t.Fatalf("Receive() lost committed update: %v", err)
	}
	received, ok := message.(sessionUpdateMsg)
	if !ok || received.update.Revision() != update.Revision() {
		t.Fatalf("Receive() message = %#v", message)
	}

	performContext, cancelPerform := context.WithCancelCause(t.Context())
	performSession := &cancelAfterSuccessSession{cancel: func() { cancelPerform(errors.New("late perform cancellation")) }, result: result}
	effects, err = NewSessionEffects(performSession)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := effects.Perform(performContext, 2, intent)
	if err != nil || committed.Message().String() != result.Message().String() {
		t.Fatalf("Perform() = %#v, %v", committed, err)
	}
}

func TestSessionEffectsPermitReceiveAndPerformConcurrently(t *testing.T) {
	t.Parallel()
	update, err := agenttui.NewActivityUpdate(1, mustText(t, "received"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := agenttui.NewCommandResult(mustText(t, "performed"), nil)
	if err != nil {
		t.Fatal(err)
	}
	session := &concurrentSession{
		receiveStarted: make(chan struct{}), releaseReceive: make(chan struct{}),
		update: update, result: result,
	}
	defer func() {
		select {
		case <-session.releaseReceive:
		default:
			close(session.releaseReceive)
		}
	}()
	effects, err := NewSessionEffects(session)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	received := make(chan error, 1)
	go func() {
		_, receiveErr := effects.Receive(ctx, 1)
		received <- receiveErr
	}()
	select {
	case <-session.receiveStarted:
	case <-ctx.Done():
		t.Fatal("Receive did not start")
	}
	intent, err := agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	performed := make(chan error, 1)
	go func() {
		_, performErr := effects.Perform(ctx, 1, intent)
		performed <- performErr
	}()
	select {
	case performErr := <-performed:
		if performErr != nil {
			t.Fatal(performErr)
		}
	case <-ctx.Done():
		t.Fatal("Perform blocked behind the active Receive")
	}
	close(session.releaseReceive)
	select {
	case receiveErr := <-received:
		if receiveErr != nil {
			t.Fatal(receiveErr)
		}
	case <-ctx.Done():
		t.Fatal("Receive did not finish after release")
	}
}

func TestSessionEffectsStopOnNonMonotonicRevision(t *testing.T) {
	t.Parallel()
	first, err := agenttui.NewActivityUpdate(1, mustText(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := agenttui.NewActivityUpdate(1, mustText(t, "duplicate"))
	if err != nil {
		t.Fatal(err)
	}
	session := &queuedSession{updates: []agenttui.SessionUpdate{first, duplicate}}
	effects, err := NewSessionEffects(session)
	if err != nil {
		t.Fatal(err)
	}
	model := fixtureEffectsModel(t, effects, t.Context())
	updated, next := model.Update(model.Init()())
	model = asModel(t, updated)
	if next == nil {
		t.Fatal("first valid update was not rearmed")
	}
	updated, retry := model.Update(next())
	model = asModel(t, updated)
	if retry != nil || model.Revision() != 1 || model.Status().Level() != agenttui.StatusError || session.receiveCalls != 2 {
		t.Fatalf(
			"duplicate revision state = retry %v revision %d status %q calls %d",
			retry, model.Revision(), model.Status().Level(), session.receiveCalls,
		)
	}
}

type queuedSession struct {
	updates      []agenttui.SessionUpdate
	result       agenttui.CommandResult
	receiveCalls int
	performCalls int
}

func (session *queuedSession) Receive(context.Context) (agenttui.SessionUpdate, error) {
	session.receiveCalls++
	if len(session.updates) == 0 {
		return agenttui.SessionUpdate{}, errors.New("script exhausted")
	}
	update := session.updates[0]
	session.updates = session.updates[1:]
	return update, nil
}

func (session *queuedSession) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	session.performCalls++
	return session.result, nil
}

type panicSession struct{}

func (*panicSession) Receive(context.Context) (agenttui.SessionUpdate, error) {
	panic("secret receive")
}

func (*panicSession) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	panic("secret perform")
}

type cancelAfterSuccessSession struct {
	cancel func()
	update agenttui.SessionUpdate
	result agenttui.CommandResult
}

type concurrentSession struct {
	receiveStarted chan struct{}
	releaseReceive chan struct{}
	update         agenttui.SessionUpdate
	result         agenttui.CommandResult
}

func (session *concurrentSession) Receive(context.Context) (agenttui.SessionUpdate, error) {
	close(session.receiveStarted)
	<-session.releaseReceive
	return session.update, nil
}

func (session *concurrentSession) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	return session.result, nil
}

func (session *cancelAfterSuccessSession) Receive(context.Context) (agenttui.SessionUpdate, error) {
	session.cancel()
	return session.update, nil
}

func (session *cancelAfterSuccessSession) Perform(context.Context, agenttui.Intent) (agenttui.CommandResult, error) {
	session.cancel()
	return session.result, nil
}

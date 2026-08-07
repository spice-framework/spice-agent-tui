package presentation

import (
	"context"
	"errors"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	agenttui "github.com/spice-framework/spice-agent-tui"
)

func TestEffectsRunOnlyInsideCommandsAndReceiveRearmsOnce(t *testing.T) {
	t.Parallel()
	effects := &scriptedEffects{
		receive: []tea.Msg{
			mustActivityMessage(t, 1, "first"),
			mustActivityMessage(t, 2, "second"),
		},
	}
	model := fixtureEffectsModel(t, effects, t.Context())
	command := model.Init()
	if command == nil || effects.receiveCalls != 0 {
		t.Fatalf("Init() command=%v receive calls=%d", command, effects.receiveCalls)
	}
	first := command()
	if effects.receiveCalls != 1 {
		t.Fatalf("first command receive calls=%d", effects.receiveCalls)
	}
	updated, rearm := model.Update(first)
	model = asModel(t, updated)
	if rearm == nil || model.Revision() != 1 {
		t.Fatalf("first receive revision=%d rearm=%v", model.Revision(), rearm)
	}

	stale := receiveCompletedMsg{token: 1, message: mustActivityMessage(t, 99, "stale")}
	updated, duplicateRearm := model.Update(stale)
	model = asModel(t, updated)
	if duplicateRearm != nil || model.Revision() != 1 || effects.receiveCalls != 1 {
		t.Fatal("stale receive was applied or rearmed")
	}
	second := rearm()
	if effects.receiveCalls != 2 {
		t.Fatalf("second command receive calls=%d", effects.receiveCalls)
	}
	updated, nextRearm := model.Update(second)
	model = asModel(t, updated)
	if nextRearm == nil || model.Revision() != 2 {
		t.Fatalf("second receive revision=%d rearm=%v", model.Revision(), nextRearm)
	}
}

func TestSemanticEffectUsesTokenAndCommitsPromptOnlyOnSuccess(t *testing.T) {
	t.Parallel()
	result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
	if err != nil {
		t.Fatal(err)
	}
	effects := &scriptedEffects{performResult: result}
	model := fixtureEffectsModel(t, effects, t.Context())

	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = asModel(t, updated)
	if command == nil || effects.performCalls != 0 || model.Editor().Value().String() != "owner" || len(model.promptHistory) != 0 {
		t.Fatal("submit executed eagerly or committed prompt optimistically")
	}
	_, duplicate := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if duplicate != nil {
		t.Fatal("second effect armed while one operation was active")
	}
	completion := command()
	if effects.performCalls != 1 || effects.lastIntent.Kind() != agenttui.IntentSubmit ||
		effects.lastIntent.Values()[0].String() != "owner" {
		t.Fatalf("performed intent = %#v, calls=%d", effects.lastIntent, effects.performCalls)
	}
	updated, followup := model.Update(completion)
	model = asModel(t, updated)
	accepted, ok := model.LastResult()
	if followup != nil || !ok || accepted.Message().String() != "accepted" ||
		model.Editor().Value().String() != "" || len(model.promptHistory) != 1 {
		t.Fatalf("successful completion state = editor %q history %d result %#v,%t",
			model.Editor().Value().String(), len(model.promptHistory), accepted, ok)
	}

	model.editor, err = agenttui.NewEditor("next")
	if err != nil {
		t.Fatal(err)
	}
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = asModel(t, updated)
	updated, staleCommand := model.Update(effectCompletedMsg{token: 1, result: result})
	model = asModel(t, updated)
	if staleCommand != nil || !model.operationActive || model.Editor().Value().String() != "next" {
		t.Fatal("stale operation completion mutated the active operation")
	}
	updated, _ = model.Update(command())
	model = asModel(t, updated)
	if model.operationActive || model.Editor().Value().String() != "" || len(model.promptHistory) != 2 {
		t.Fatal("current operation completion was not committed exactly once")
	}
}

func TestCancelUsesIndependentControlLaneWhileOperationIsActive(t *testing.T) {
	t.Parallel()
	result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
	if err != nil {
		t.Fatal(err)
	}
	effects := &scriptedEffects{performResult: result}
	model := fixtureEffectsModel(t, effects, t.Context())
	updated, submit := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = asModel(t, updated)
	updated, cancel := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = asModel(t, updated)
	if submit == nil || cancel == nil || !model.operationActive || !model.cancelActive {
		t.Fatalf("control lanes = submit %v cancel %v operation %t cancel %t", submit, cancel, model.operationActive, model.cancelActive)
	}
	_, duplicateCancel := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if duplicateCancel != nil {
		t.Fatal("duplicate cancel armed while the cancel lane was active")
	}
	updated, _ = model.Update(cancel())
	model = asModel(t, updated)
	if model.cancelActive || !model.operationActive || effects.lastIntent.Kind() != agenttui.IntentCancelActiveRun {
		t.Fatalf("cancel completion = operation %t cancel %t intent %q", model.operationActive, model.cancelActive, effects.lastIntent.Kind())
	}
	updated, _ = model.Update(submit())
	model = asModel(t, updated)
	if model.operationActive || model.cancelActive || effects.lastIntent.Kind() != agenttui.IntentSubmit {
		t.Fatalf("submit completion = operation %t cancel %t intent %q", model.operationActive, model.cancelActive, effects.lastIntent.Kind())
	}
}

func TestDefiniteEffectFailureRetainsDraftAndDoesNotRetry(t *testing.T) {
	t.Parallel()
	effects := &scriptedEffects{performErr: errors.New("unsafe\x1b[31m detail")}
	model := fixtureEffectsModel(t, effects, t.Context())
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = asModel(t, updated)
	updated, retry := model.Update(command())
	model = asModel(t, updated)
	if retry != nil || effects.performCalls != 1 || model.Editor().Value().String() != "owner" ||
		len(model.promptHistory) != 0 || stringsContainsControl(model.Status().Message().String()) {
		t.Fatalf("failed effect state = calls %d editor %q history %d status %q",
			effects.performCalls, model.Editor().Value().String(), len(model.promptHistory), model.Status().Message().String())
	}
}

func TestInjectedBindingsMapCancelAndResponseToSemanticIntents(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		key        tea.Key
		wantKind   agenttui.IntentKind
		wantValues int
	}{
		{name: "cancel", key: tea.Key{Code: tea.KeyEscape}, wantKind: agenttui.IntentCancelActiveRun},
		{name: "respond", key: tea.Key{Code: tea.KeyEnter, Mod: tea.ModAlt}, wantKind: agenttui.IntentRespond, wantValues: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
			if err != nil {
				t.Fatal(err)
			}
			effects := &scriptedEffects{performResult: result}
			model := fixtureEffectsModel(t, effects, t.Context())
			updated, command := model.Update(tea.KeyPressMsg(test.key))
			model = asModel(t, updated)
			if command == nil {
				t.Fatal("semantic binding returned no effect command")
			}
			updated, _ = model.Update(command())
			model = asModel(t, updated)
			if effects.lastIntent.Kind() != test.wantKind || len(effects.lastIntent.Values()) != test.wantValues || model.operationActive {
				t.Fatalf("intent = %#v, operation active=%t", effects.lastIntent, model.operationActive)
			}
		})
	}
}

func TestNoEffectsFallbackDoesNotExecuteSemanticActions(t *testing.T) {
	t.Parallel()
	model := fixtureModel(t, FixedRenderer{})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = asModel(t, updated)
	if command != nil || model.Editor().Value().String() != "owner" {
		t.Fatal("no-effects fallback mutated or launched work")
	}
}

func fixtureEffectsModel(t *testing.T, effects Effects, ctx context.Context) Model {
	t.Helper()
	view := fixtureView(t)
	model, err := NewModel(
		FixedRenderer{}, view.Workspace(), view.Status(), view.Prompt(), view.Activity(),
		agenttui.DarkTheme(), standardBindings(t), effects,
	)
	if err != nil {
		t.Fatal(err)
	}
	return model.withEffectsContext(ctx, nil)
}

func mustActivityMessage(t *testing.T, revision uint64, value string) sessionUpdateMsg {
	t.Helper()
	update, err := agenttui.NewActivityUpdate(revision, mustText(t, value))
	if err != nil {
		t.Fatal(err)
	}
	return sessionUpdateMsg{update: update}
}

type scriptedEffects struct {
	mu            sync.Mutex
	receive       []tea.Msg
	receiveCalls  int
	performCalls  int
	performResult agenttui.CommandResult
	performErr    error
	lastIntent    agenttui.Intent
}

func (effects *scriptedEffects) Receive(ctx context.Context, _ OperationToken) (tea.Msg, error) {
	effects.mu.Lock()
	defer effects.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	effects.receiveCalls++
	if len(effects.receive) == 0 {
		return nil, errors.New("script exhausted")
	}
	message := effects.receive[0]
	effects.receive = effects.receive[1:]
	return message, nil
}

func (effects *scriptedEffects) Perform(
	ctx context.Context,
	_ OperationToken,
	intent agenttui.Intent,
) (agenttui.CommandResult, error) {
	effects.mu.Lock()
	defer effects.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return agenttui.CommandResult{}, err
	}
	effects.performCalls++
	effects.lastIntent = intent
	return effects.performResult, effects.performErr
}

func stringsContainsControl(value string) bool {
	for _, character := range value {
		if character < ' ' && character != '\n' && character != '\t' {
			return true
		}
	}
	return false
}

package agenttui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInvocationIntentAndResultAreBoundedImmutableValues(t *testing.T) {
	t.Parallel()
	arguments := []Text{mustText(t, "owners"), mustText(t, "list")}
	invocation, err := NewInvocation("operation-1", arguments)
	if err != nil {
		t.Fatal(err)
	}
	arguments[0] = Text{}
	copyArguments := invocation.Arguments()
	copyArguments[0] = Text{}
	if invocation.OperationID() != "operation-1" || invocation.Arguments()[0].String() != "owners" {
		t.Fatal("invocation retained caller mutation")
	}
	if _, validationErr := NewInvocation(" bad ", nil); validationErr == nil {
		t.Fatal("NewInvocation(invalid identity) error = nil")
	}
	if _, validationErr := NewInvocation("operation", make([]Text, MaximumCommandArguments+1)); validationErr == nil {
		t.Fatal("NewInvocation(oversize arguments) error = nil")
	}

	values := []Text{mustText(t, "answer")}
	intent, err := NewIntent(IntentRespond, values)
	if err != nil {
		t.Fatal(err)
	}
	values[0] = Text{}
	copyValues := intent.Values()
	copyValues[0] = Text{}
	if intent.Kind() != IntentRespond || intent.Values()[0].String() != "answer" {
		t.Fatal("intent retained caller mutation")
	}
	for _, invalid := range []Intent{
		{},
		{kind: IntentCancelActiveRun, values: []Text{mustText(t, "unexpected")}},
		{kind: IntentSubmit},
		{kind: "unknown"},
	} {
		if invalid.Validate() == nil {
			t.Fatalf("Intent.Validate(%#v) error = nil", invalid)
		}
	}
	result, err := NewCommandResult(mustText(t, "queued"), &intent)
	if err != nil {
		t.Fatal(err)
	}
	gotIntent, ok := result.Intent()
	if !ok || gotIntent.Kind() != IntentRespond || result.Message().String() != "queued" {
		t.Fatalf("command result = %#v, %#v, %t", result, gotIntent, ok)
	}
}

func TestCommandMetadataRejectsInvalidAndDuplicateIDs(t *testing.T) {
	t.Parallel()
	commands := []Command{
		stubCommand{id: "owners.list", summary: mustText(t, "List owners")},
		stubCommand{id: "owners.list", summary: mustText(t, "List owners again")},
	}
	if err := ValidateCommands(commands); err == nil || !strings.Contains(err.Error(), "indexes 0 and 1") {
		t.Fatalf("ValidateCommands(duplicate) error = %v", err)
	}
	if err := ValidateCommands([]Command{stubCommand{id: "bad id", summary: mustText(t, "bad")}}); err == nil {
		t.Fatal("ValidateCommands(invalid ID) error = nil")
	}
	if err := ValidateCommands([]Command{nil}); err == nil {
		t.Fatal("ValidateCommands(nil) error = nil")
	}
	if err := ValidateCommands(commands[:1]); err != nil {
		t.Fatalf("ValidateCommands(valid) error = %v", err)
	}
}

func TestTerminalIOAndConfigAreExplicitValidatedValues(t *testing.T) {
	t.Parallel()
	input := bytes.NewBufferString("input")
	output := &bytes.Buffer{}
	terminal, err := NewTerminalIO(input, output)
	if err != nil || terminal.Input() != input || terminal.Output() != output {
		t.Fatalf("NewTerminalIO() = %#v, %v", terminal, err)
	}
	if _, validationErr := NewTerminalIO(nil, output); validationErr == nil {
		t.Fatal("NewTerminalIO(nil input) error = nil")
	}
	if _, validationErr := NewTerminalIO(input, nil); validationErr == nil {
		t.Fatal("NewTerminalIO(nil output) error = nil")
	}
	config, err := NewTerminalConfig(true, "coding/default", "revision-7", 2*time.Second)
	if err != nil || !config.Accessible() || config.DefinitionID() != "coding/default" ||
		config.DefinitionRevision() != "revision-7" || config.ShutdownTimeout() != 2*time.Second {
		t.Fatalf("NewTerminalConfig() = %#v, %v", config, err)
	}
	for _, test := range []struct {
		definition string
		revision   string
		timeout    time.Duration
	}{
		{definition: "", revision: "revision-1", timeout: time.Second},
		{definition: "bad definition", revision: "revision-1", timeout: time.Second},
		{definition: "valid", timeout: time.Second},
		{definition: "valid", revision: "revision-1", timeout: 0},
		{definition: "valid", revision: "revision-1", timeout: MaximumShutdownTimeout + time.Nanosecond},
	} {
		if _, configErr := NewTerminalConfig(false, test.definition, test.revision, test.timeout); configErr == nil {
			t.Fatalf("NewTerminalConfig(%q, %s) error = nil", test.definition, test.timeout)
		}
	}
}

type stubCommand struct {
	id      string
	summary Text
}

func (command stubCommand) ID() string    { return command.id }
func (command stubCommand) Summary() Text { return command.summary }
func (stubCommand) Execute(context.Context, Invocation) (CommandResult, error) {
	return CommandResult{}, errors.New("not implemented")
}

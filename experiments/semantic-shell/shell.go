package semanticshell

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

const (
	// MaximumInputBytes bounds one complete command line, excluding its newline.
	MaximumInputBytes = agenttui.MaximumPromptBytes + 16
	// MaximumOutputBytes bounds one encoded JSON Lines record.
	MaximumOutputBytes = agenttui.MaximumFrameBytes
	ordinaryQueueDepth = 8
	cancelQueueDepth   = 1
	recordSchema       = "spice.agent.semantic-shell/v1alpha1"
)

var (
	// ErrInvalidConfiguration reports a missing Session or stream.
	ErrInvalidConfiguration = errors.New("semantic shell configuration is invalid")
	// ErrInput reports malformed, oversized, or failed command input.
	ErrInput = errors.New("semantic shell input failed")
	// ErrOutput reports a failed or oversized JSON Lines record.
	ErrOutput = errors.New("semantic shell output failed")
	// ErrSessionReceive reports a failed, panicked, invalid, or stale receive.
	ErrSessionReceive = errors.New("semantic shell session receive failed")
)

// Shell is one line-oriented semantic client. Shell is single-use.
type Shell struct {
	session agenttui.Session
	input   io.ReadCloser
	output  io.Writer

	mu      sync.Mutex
	running bool
}

// New validates and constructs a semantic shell. The shell closes input when
// Run ends so cancellation can interrupt an otherwise blocking read. It never
// closes output or acquires any transport implicitly.
func New(session agenttui.Session, input io.ReadCloser, output io.Writer) (*Shell, error) {
	if session == nil || input == nil || output == nil {
		return nil, ErrInvalidConfiguration
	}
	return &Shell{session: session, input: input, output: output}, nil
}

// Run consumes commands until quit, EOF, caller cancellation, or a terminal
// receive/output failure. Session operations are attempted exactly once.
func (shell *Shell) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidConfiguration
	}
	shell.mu.Lock()
	if shell.running {
		shell.mu.Unlock()
		return ErrInvalidConfiguration
	}
	shell.running = true
	shell.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopClose := context.AfterFunc(runCtx, func() { safeClose(shell.input) })
	defer func() {
		stopClose()
		safeClose(shell.input)
	}()

	emitter := newEmitter(shell.output, cancel)
	ordinary := make(chan operation, ordinaryQueueDepth)
	cancels := make(chan operation, cancelQueueDepth)
	terminal := make(chan error, 1)

	var workers sync.WaitGroup
	workers.Add(3)
	go receiveLoop(runCtx, &workers, shell.session, emitter, terminal, cancel)
	go performLoop(runCtx, &workers, shell.session, ordinary, emitter)
	go performLoop(runCtx, &workers, shell.session, cancels, emitter)

	inputErr := shell.readCommands(runCtx, cancel, ordinary, cancels, emitter)
	cancel()
	workers.Wait()

	if err := emitter.err(); err != nil {
		return err
	}
	select {
	case err := <-terminal:
		return err
	default:
	}
	if inputErr != nil {
		return inputErr
	}
	return nil
}

type operation struct {
	id      uint64
	command string
	intent  agenttui.Intent
}

func (shell *Shell) readCommands(
	ctx context.Context,
	cancel context.CancelFunc,
	ordinary chan<- operation,
	cancels chan<- operation,
	emitter *recordEmitter,
) (resultErr error) {
	defer func() {
		if recover() != nil {
			emitter.issue("input_failed", "command input failed", "", 0)
			resultErr = ErrInput
		}
	}()
	scanner := bufio.NewScanner(shell.input)
	scanner.Buffer(make([]byte, 256), MaximumInputBytes+1)
	var operationID uint64
	for scanner.Scan() {
		command, value, err := parseCommand(scanner.Text())
		if err != nil {
			emitter.issue("invalid_command", "command input is invalid", "", 0)
			continue
		}
		if command == "quit" {
			emitter.result("quit", 0, "semantic shell stopped")
			cancel()
			return nil
		}
		intent, err := commandIntent(command, value)
		if err != nil {
			emitter.issue("invalid_command", "command input is invalid", command, 0)
			continue
		}
		operationID++
		request := operation{id: operationID, command: command, intent: intent}
		queue := ordinary
		if intent.Kind() == agenttui.IntentCancelActiveRun {
			queue = cancels
		}
		select {
		case queue <- request:
		case <-ctx.Done():
			return nil
		default:
			emitter.issue("queue_full", "command queue is full", command, operationID)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		emitter.issue("input_failed", "command input failed", "", 0)
		return ErrInput
	}
	return nil
}

func safeClose(closer io.Closer) {
	defer func() { _ = recover() }()
	_ = closer.Close()
}

func parseCommand(line string) (string, string, error) {
	if len(line) > MaximumInputBytes || strings.ContainsRune(line, '\x00') {
		return "", "", ErrInput
	}
	line = strings.TrimSuffix(line, "\r")
	command, value, hasValue := strings.Cut(line, " ")
	switch command {
	case "submit", "respond":
		if !hasValue || value == "" {
			return "", "", ErrInput
		}
	case "cancel", "quit":
		if hasValue {
			return "", "", ErrInput
		}
	default:
		return "", "", ErrInput
	}
	return command, value, nil
}

func commandIntent(command, value string) (agenttui.Intent, error) {
	switch command {
	case "submit":
		return textIntent(agenttui.IntentSubmit, value)
	case "respond":
		return textIntent(agenttui.IntentRespond, value)
	case "cancel":
		return agenttui.NewIntent(agenttui.IntentCancelActiveRun, nil)
	default:
		return agenttui.Intent{}, ErrInput
	}
}

func textIntent(kind agenttui.IntentKind, value string) (agenttui.Intent, error) {
	if len(value) > agenttui.MaximumPromptBytes {
		return agenttui.Intent{}, ErrInput
	}
	text, err := agenttui.NewText(value)
	if err != nil {
		return agenttui.Intent{}, ErrInput
	}
	intent, err := agenttui.NewIntent(kind, []agenttui.Text{text})
	if err != nil {
		return agenttui.Intent{}, ErrInput
	}
	return intent, nil
}

func receiveLoop(
	ctx context.Context,
	workers *sync.WaitGroup,
	session agenttui.Session,
	emitter *recordEmitter,
	terminal chan<- error,
	cancel context.CancelFunc,
) {
	defer workers.Done()
	state := semanticState{}
	for {
		update, err := safeReceive(ctx, session)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			emitter.issue("receive_failed", "session receive failed", "", 0)
			signalTerminal(terminal, ErrSessionReceive)
			cancel()
			return
		}
		view, err := state.apply(update)
		if err != nil {
			emitter.issue("invalid_update", "session update is invalid", "", 0)
			signalTerminal(terminal, ErrSessionReceive)
			cancel()
			return
		}
		emitter.view(view)
	}
}

func signalTerminal(terminal chan<- error, err error) {
	select {
	case terminal <- err:
	default:
	}
}

func performLoop(
	ctx context.Context,
	workers *sync.WaitGroup,
	session agenttui.Session,
	requests <-chan operation,
	emitter *recordEmitter,
) {
	defer workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case request := <-requests:
			result, err := safePerform(ctx, session, request.intent)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				emitter.issue("perform_failed", "session command failed", request.command, request.id)
				continue
			}
			if err = result.Validate(); err != nil {
				emitter.issue("invalid_result", "session command result is invalid", request.command, request.id)
				continue
			}
			emitter.result(request.command, request.id, result.Message().String())
		}
	}
}

func safeReceive(ctx context.Context, session agenttui.Session) (update agenttui.SessionUpdate, err error) {
	defer func() {
		if recover() != nil {
			update = agenttui.SessionUpdate{}
			err = ErrSessionReceive
		}
	}()
	return session.Receive(ctx)
}

func safePerform(
	ctx context.Context,
	session agenttui.Session,
	intent agenttui.Intent,
) (result agenttui.CommandResult, err error) {
	defer func() {
		if recover() != nil {
			result = agenttui.CommandResult{}
			err = errors.New("semantic shell session perform failed")
		}
	}()
	return session.Perform(ctx, intent)
}

type record struct {
	Schema    string        `json:"schema"`
	Sequence  uint64        `json:"sequence"`
	Type      string        `json:"type"`
	Code      string        `json:"code,omitempty"`
	Message   string        `json:"message,omitempty"`
	Command   string        `json:"command,omitempty"`
	Operation uint64        `json:"operation,omitempty"`
	Revision  uint64        `json:"revision,omitempty"`
	View      *semanticView `json:"view,omitempty"`
}

type recordEmitter struct {
	mu       sync.Mutex
	output   io.Writer
	cancel   context.CancelFunc
	sequence uint64
	failure  error
}

func newEmitter(output io.Writer, cancel context.CancelFunc) *recordEmitter {
	return &recordEmitter{output: output, cancel: cancel}
}

func (emitter *recordEmitter) view(view semanticView) {
	emitter.emit(record{Type: "view", Revision: view.Revision, View: &view})
}

func (emitter *recordEmitter) result(command string, operation uint64, message string) {
	emitter.emit(record{Type: "result", Command: command, Operation: operation, Message: message})
}

func (emitter *recordEmitter) issue(code, message, command string, operation uint64) {
	emitter.emit(record{
		Type: "error", Code: code, Message: message, Command: command, Operation: operation,
	})
}

func (emitter *recordEmitter) emit(value record) {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	if emitter.failure != nil {
		return
	}
	emitter.sequence++
	value.Schema = recordSchema
	value.Sequence = emitter.sequence
	content, err := json.Marshal(value)
	if err != nil || len(content)+1 > MaximumOutputBytes {
		emitter.failure = ErrOutput
		emitter.cancel()
		return
	}
	content = append(content, '\n')
	if err = writeAll(emitter.output, content); err != nil {
		emitter.failure = ErrOutput
		emitter.cancel()
	}
}

func (emitter *recordEmitter) err() error {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	return emitter.failure
}

func writeAll(output io.Writer, content []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrOutput
		}
	}()
	for len(content) != 0 {
		written, writeErr := output.Write(content)
		if writeErr != nil {
			return ErrOutput
		}
		if written <= 0 || written > len(content) {
			return ErrOutput
		}
		content = content[written:]
	}
	return nil
}

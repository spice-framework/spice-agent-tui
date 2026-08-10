// Package agentcompat contains the removable protocol adapter used only by the
// semantic-shell compatibility experiment. It is not a TUI transport.
package agentcompat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent/client"
	"github.com/spice-framework/spice-agent/client/grpcclient"
	"github.com/spice-framework/spice-agent/daemon/endpoint"
	"github.com/spice-framework/spice-agent/daemon/localipc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const updateQueueDepth = 16

// AdapterFacts are the negotiated facts asserted by the conformance test.
type AdapterFacts struct {
	ProtocolMinor uint32
	AttemptID     bool
}

// Adapter translates the released TUI Session values to the public Agent
// client contract. Only the experiment conformance test constructs it.
type Adapter struct {
	session    client.Session
	connection *grpc.ClientConn
	updates    chan agenttui.SessionUpdate

	mu        sync.Mutex
	revision  uint64
	operation uint64
	run       client.RunRef
	hasRun    bool
}

// Dial establishes a real local-IPC gRPC session with an exact semantic
// profile. Minor 2 uses legacy initialization; minor 3 uses exact replay.
func Dial(
	ctx context.Context,
	address string,
	token endpoint.Token,
	maximumMinor uint32,
) (*Adapter, AdapterFacts, error) {
	if maximumMinor != 2 && maximumMinor != 3 {
		return nil, AdapterFacts{}, errors.New("agent compatibility profile must be minor 2 or 3")
	}
	connection, err := grpc.NewClient(
		"passthrough:///spice-agent-tui-semantic-shell",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableServiceConfig(),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return localipc.Dial(ctx, address)
		}),
	)
	if err != nil {
		return nil, AdapterFacts{}, fmt.Errorf("create compatibility connection: %w", err)
	}
	fail := func(cause error) (*Adapter, AdapterFacts, error) {
		_ = connection.Close()
		return nil, AdapterFacts{}, cause
	}
	connector, err := grpcclient.New(grpcclient.Config{Connection: connection, Token: token})
	if err != nil {
		return fail(fmt.Errorf("create compatibility connector: %w", err))
	}
	request, hasAttempt, err := initializeRequest(maximumMinor)
	if err != nil {
		return fail(err)
	}
	session, err := connector.Initialize(ctx, request)
	if err != nil {
		return fail(fmt.Errorf("initialize compatibility session: %w", err))
	}
	adapter := &Adapter{
		session: session, connection: connection, updates: make(chan agenttui.SessionUpdate, updateQueueDepth), revision: 1,
	}
	update, err := initialUpdate(maximumMinor)
	if err != nil {
		_ = adapter.Close()
		return nil, AdapterFacts{}, err
	}
	adapter.updates <- update
	return adapter, AdapterFacts{
		ProtocolMinor: session.Connection().Protocol().Minor(), AttemptID: hasAttempt,
	}, nil
}

func initializeRequest(maximumMinor uint32) (client.InitializeRequest, bool, error) {
	version, err := client.NewProtocolVersion(1, maximumMinor, 0)
	if err != nil {
		return client.InitializeRequest{}, false, err
	}
	protocol, err := client.NewProtocolRange(version, version)
	if err != nil {
		return client.InitializeRequest{}, false, err
	}
	build, err := client.NewBuild("semantic-shell", "source-built", "conformance", "go1.26.5")
	if err != nil {
		return client.InitializeRequest{}, false, err
	}
	limits, err := client.NewLimits(1<<20, 64, 64, 1<<20, 4, 4)
	if err != nil {
		return client.InitializeRequest{}, false, err
	}
	if maximumMinor == 2 {
		request, requestErr := client.NewLegacyInitializeRequest(protocol, build, nil, nil, limits)
		return request, false, requestErr
	}
	attempt, err := client.NewInitializationAttemptID()
	if err != nil {
		return client.InitializeRequest{}, false, err
	}
	request, err := client.NewInitializeRequestWithAttempt(protocol, build, nil, nil, limits, attempt)
	return request, true, err
}

func initialUpdate(minor uint32) (agenttui.SessionUpdate, error) {
	workspace, err := agenttui.NewWorkspace(mustText("Agent protocol conformance"), nil)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	status, err := agenttui.NewStatus(
		agenttui.StatusReady,
		mustText(fmt.Sprintf("connected to source-built protocol 1.%d", minor)),
		[]agenttui.Text{mustText("submit"), mustText("respond"), mustText("cancel")},
	)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	snapshot, err := agenttui.NewSessionSnapshot(1, workspace, status, nil, nil)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	return agenttui.NewSnapshotUpdate(snapshot)
}

// Receive implements the released UI-neutral Session contract.
func (adapter *Adapter) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	select {
	case update := <-adapter.updates:
		return update, nil
	case <-ctx.Done():
		return agenttui.SessionUpdate{}, context.Cause(ctx)
	}
}

// Perform translates one UI-neutral intent to an exact public Agent mutation.
func (adapter *Adapter) Perform(ctx context.Context, intent agenttui.Intent) (agenttui.CommandResult, error) {
	if err := intent.Validate(); err != nil {
		return agenttui.CommandResult{}, err
	}
	adapter.mu.Lock()
	adapter.operation++
	operationNumber := adapter.operation
	adapter.mu.Unlock()
	operation, err := client.NewOperationID(fmt.Sprintf("semantic-shell-%d", operationNumber))
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	switch intent.Kind() {
	case agenttui.IntentSubmit:
		return adapter.submit(ctx, operation, intent.Values()[0].String())
	case agenttui.IntentRespond:
		return adapter.respond(ctx, operation, intent.Values()[0].String())
	case agenttui.IntentCancelActiveRun:
		return adapter.cancel(ctx, operation)
	default:
		return agenttui.CommandResult{}, errors.New("unsupported semantic-shell intent")
	}
}

func (adapter *Adapter) submit(
	ctx context.Context,
	operation client.OperationID,
	value string,
) (agenttui.CommandResult, error) {
	definition := adapter.session.Connection().Catalog().Definitions()[0].Ref()
	input, err := client.NewInput("message-"+operation.String(), value)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	request, err := client.NewStartRequest(operation, definition, input)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	result, err := adapter.session.Start(ctx, request)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	adapter.mu.Lock()
	adapter.run, adapter.hasRun = result.Run(), true
	adapter.mu.Unlock()
	return adapter.result("run submitted", "submitted")
}

func (adapter *Adapter) respond(
	ctx context.Context,
	operation client.OperationID,
	value string,
) (agenttui.CommandResult, error) {
	adapter.mu.Lock()
	run, hasRun := adapter.run, adapter.hasRun
	adapter.mu.Unlock()
	if !hasRun {
		return agenttui.CommandResult{}, errors.New("no active conformance run")
	}
	structured, err := client.NewStructuredText(value)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	response, err := client.NewInteractionResponse("approval", structured)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	request, err := client.NewRespondRequest(run, operation, response)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	if _, err = adapter.session.Respond(ctx, request); err != nil {
		return agenttui.CommandResult{}, err
	}
	return adapter.result("interaction accepted", "responded")
}

func (adapter *Adapter) cancel(
	ctx context.Context,
	operation client.OperationID,
) (agenttui.CommandResult, error) {
	adapter.mu.Lock()
	run, hasRun := adapter.run, adapter.hasRun
	adapter.mu.Unlock()
	if !hasRun {
		return agenttui.CommandResult{}, errors.New("no active conformance run")
	}
	request, err := client.NewCancelRequest(run, operation, "semantic-shell request")
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	if _, err = adapter.session.Cancel(ctx, request); err != nil {
		return agenttui.CommandResult{}, err
	}
	return adapter.result("run cancellation requested", "cancelled")
}

func (adapter *Adapter) result(message, activity string) (agenttui.CommandResult, error) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.revision++
	update, err := agenttui.NewActivityUpdate(adapter.revision, mustText(activity))
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	adapter.updates <- update
	return agenttui.NewCommandResult(mustText(message), nil)
}

func mustText(value string) agenttui.Text {
	text, err := agenttui.NewText(value)
	if err != nil {
		panic("fixed agent compatibility text is invalid")
	}
	return text
}

// Close releases only this conformance adapter and its channel.
func (adapter *Adapter) Close() error {
	if adapter == nil {
		return nil
	}
	var result error
	if adapter.session != nil {
		result = errors.Join(result, adapter.session.Close())
	}
	if adapter.connection != nil {
		result = errors.Join(result, adapter.connection.Close())
	}
	return result
}

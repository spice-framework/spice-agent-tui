package releasedversionskew

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent/client"
	"github.com/spice-framework/spice-agent/client/grpcclient"
	commonv1 "github.com/spice-framework/spice-agent/common/v1"
	"github.com/spice-framework/spice-agent/daemon/endpoint"
	"github.com/spice-framework/spice-agent/daemon/localipc"
	enginev1 "github.com/spice-framework/spice-agent/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testTimeout = 10 * time.Second

var _ agenttui.Session = (*semanticAdapter)(nil)

func TestReleasedClientPeerSemanticContract(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("released client version skew is hosted on Linux and Windows")
	}
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	token, err := endpoint.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	address := compatibilityAddress(t)
	listener, err := localipc.Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	peer := newCompatibilityPeer(t, listener, token)

	wrong, err := endpoint.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err = initializeWithToken(ctx, address, wrong); err == nil {
		t.Fatal("wrong-token initialize unexpectedly succeeded")
	}

	adapter, err := newSemanticAdapter(ctx, address, token)
	if err != nil {
		t.Fatal(err)
	}
	update, err := adapter.Receive(ctx)
	if err != nil || update.Kind() != agenttui.SessionUpdateSnapshot || update.Revision() != 1 {
		t.Fatalf("initial semantic update = %#v, %v", update, err)
	}
	for _, test := range []struct {
		kind   agenttui.IntentKind
		value  string
		result string
	}{
		{kind: agenttui.IntentSubmit, value: "hello", result: "submitted"},
		{kind: agenttui.IntentRespond, value: "approved", result: "responded"},
		{kind: agenttui.IntentCancelActiveRun, result: "cancelled"},
	} {
		intent := mustIntent(t, test.kind, test.value)
		result, performErr := adapter.Perform(ctx, intent)
		if performErr != nil || result.Message().String() != test.result {
			t.Fatalf("perform %s = %#v, %v", test.kind, result, performErr)
		}
	}
	if err = adapter.Close(); err != nil {
		t.Fatal(err)
	}
	stats := peer.stop(t)
	if stats != (peerStats{Initialize: 1, Start: 1, Respond: 1, Cancel: 1}) {
		t.Fatalf("peer stats = %#v", stats)
	}
	if runtime.GOOS != "windows" {
		if _, err = os.Stat(address); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("local IPC socket remains after cleanup: %v", err)
		}
	}
	if err = ctx.Err(); err != nil {
		t.Fatalf("released semantic contract exceeded its bound: %v", err)
	}
}

func mustIntent(t *testing.T, kind agenttui.IntentKind, value string) agenttui.Intent {
	t.Helper()
	values := []agenttui.Text(nil)
	if value != "" {
		text, err := agenttui.NewText(value)
		if err != nil {
			t.Fatal(err)
		}
		values = []agenttui.Text{text}
	}
	intent, err := agenttui.NewIntent(kind, values)
	if err != nil {
		t.Fatal(err)
	}
	return intent
}

func compatibilityAddress(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\spice-agent-tui-released-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, "engine.sock")
}

func initializeWithToken(ctx context.Context, address string, token endpoint.Token) error {
	session, connection, err := initializeSession(ctx, address, token)
	if session != nil {
		_ = session.Close()
	}
	if connection != nil {
		_ = connection.Close()
	}
	return err
}

func initializeSession(
	ctx context.Context,
	address string,
	token endpoint.Token,
) (client.Session, *grpc.ClientConn, error) {
	connection, err := grpc.NewClient(
		"passthrough:///spice-agent-tui-released-version-skew",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableServiceConfig(),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return localipc.Dial(ctx, address)
		}),
	)
	if err != nil {
		return nil, nil, err
	}
	connector, err := grpcclient.New(grpcclient.Config{Connection: connection, Token: token})
	if err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	request, err := initializeRequest()
	if err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	session, err := connector.Initialize(ctx, request)
	if err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	return session, connection, nil
}

func initializeRequest() (client.InitializeRequest, error) {
	version, err := client.NewProtocolVersion(1, 3, 0)
	if err != nil {
		return client.InitializeRequest{}, err
	}
	protocol, err := client.NewProtocolRange(version, version)
	if err != nil {
		return client.InitializeRequest{}, err
	}
	build, err := client.NewBuild("released-semantic-client", "matrix", "public-module", "go1.26.5")
	if err != nil {
		return client.InitializeRequest{}, err
	}
	limits, err := client.NewLimits(1<<20, 64, 64, 1<<20, 4, 4)
	if err != nil {
		return client.InitializeRequest{}, err
	}
	attempt, err := client.NewInitializationAttemptID()
	if err != nil {
		return client.InitializeRequest{}, err
	}
	return client.NewInitializeRequestWithAttempt(protocol, build, nil, nil, limits, attempt)
}

type semanticAdapter struct {
	session    client.Session
	connection *grpc.ClientConn
	updates    chan agenttui.SessionUpdate
	run        client.RunRef
	hasRun     bool
	operation  uint64
}

func newSemanticAdapter(
	ctx context.Context,
	address string,
	token endpoint.Token,
) (*semanticAdapter, error) {
	session, connection, err := initializeSession(ctx, address, token)
	if err != nil {
		return nil, err
	}
	update, err := initialUpdate()
	if err != nil {
		_ = session.Close()
		_ = connection.Close()
		return nil, err
	}
	adapter := &semanticAdapter{session: session, connection: connection, updates: make(chan agenttui.SessionUpdate, 1)}
	adapter.updates <- update
	return adapter, nil
}

func initialUpdate() (agenttui.SessionUpdate, error) {
	title, err := agenttui.NewText("released semantic matrix")
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	workspace, err := agenttui.NewWorkspace(title, nil)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	message, err := agenttui.NewText("authenticated")
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	statusState, err := agenttui.NewStatus(agenttui.StatusReady, message, nil)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	snapshot, err := agenttui.NewSessionSnapshot(1, workspace, statusState, nil, nil)
	if err != nil {
		return agenttui.SessionUpdate{}, err
	}
	return agenttui.NewSnapshotUpdate(snapshot)
}

func (adapter *semanticAdapter) Receive(ctx context.Context) (agenttui.SessionUpdate, error) {
	select {
	case update := <-adapter.updates:
		return update, nil
	case <-ctx.Done():
		return agenttui.SessionUpdate{}, context.Cause(ctx)
	}
}

func (adapter *semanticAdapter) Perform(
	ctx context.Context,
	intent agenttui.Intent,
) (agenttui.CommandResult, error) {
	if err := intent.Validate(); err != nil {
		return agenttui.CommandResult{}, err
	}
	adapter.operation++
	operation, err := client.NewOperationID(fmt.Sprintf("semantic-%d", adapter.operation))
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	switch intent.Kind() {
	case agenttui.IntentSubmit:
		definition := adapter.session.Connection().Catalog().Definitions()[0].Ref()
		input, inputErr := client.NewInput("message-1", intent.Values()[0].String())
		if inputErr != nil {
			return agenttui.CommandResult{}, inputErr
		}
		request, requestErr := client.NewStartRequest(operation, definition, input)
		if requestErr != nil {
			return agenttui.CommandResult{}, requestErr
		}
		result, startErr := adapter.session.Start(ctx, request)
		if startErr != nil {
			return agenttui.CommandResult{}, startErr
		}
		adapter.run, adapter.hasRun = result.Run(), true
		return commandResult("submitted")
	case agenttui.IntentRespond:
		if !adapter.hasRun {
			return agenttui.CommandResult{}, errors.New("no active run")
		}
		value, valueErr := client.NewStructuredText(intent.Values()[0].String())
		if valueErr != nil {
			return agenttui.CommandResult{}, valueErr
		}
		response, responseErr := client.NewInteractionResponse("approval", value)
		if responseErr != nil {
			return agenttui.CommandResult{}, responseErr
		}
		request, requestErr := client.NewRespondRequest(adapter.run, operation, response)
		if requestErr != nil {
			return agenttui.CommandResult{}, requestErr
		}
		if _, responseErr = adapter.session.Respond(ctx, request); responseErr != nil {
			return agenttui.CommandResult{}, responseErr
		}
		return commandResult("responded")
	case agenttui.IntentCancelActiveRun:
		if !adapter.hasRun {
			return agenttui.CommandResult{}, errors.New("no active run")
		}
		request, requestErr := client.NewCancelRequest(adapter.run, operation, "semantic matrix")
		if requestErr != nil {
			return agenttui.CommandResult{}, requestErr
		}
		if _, err = adapter.session.Cancel(ctx, request); err != nil {
			return agenttui.CommandResult{}, err
		}
		return commandResult("cancelled")
	default:
		return agenttui.CommandResult{}, errors.New("unsupported intent")
	}
}

func commandResult(value string) (agenttui.CommandResult, error) {
	message, err := agenttui.NewText(value)
	if err != nil {
		return agenttui.CommandResult{}, err
	}
	return agenttui.NewCommandResult(message, nil)
}

func (adapter *semanticAdapter) Close() error {
	if adapter == nil {
		return nil
	}
	return errors.Join(adapter.session.Close(), adapter.connection.Close())
}

type peerStats struct {
	Initialize int
	Start      int
	Respond    int
	Cancel     int
}

type compatibilityPeer struct {
	server   *grpc.Server
	listener net.Listener
	service  *peerService
	done     <-chan struct{}
	once     sync.Once
}

func newCompatibilityPeer(
	t *testing.T,
	listener net.Listener,
	token endpoint.Token,
) *compatibilityPeer {
	t.Helper()
	authorization, err := token.AuthorizationValue()
	if err != nil {
		t.Fatal(err)
	}
	service := new(peerService)
	server := grpc.NewServer(grpc.UnaryInterceptor(authorizeUnary(authorization)))
	enginev1.RegisterEngineServiceServer(server, service)
	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()
	peer := &compatibilityPeer{server: server, listener: listener, service: service, done: done}
	t.Cleanup(func() { peer.stop(t) })
	return peer
}

func (peer *compatibilityPeer) stop(t *testing.T) peerStats {
	t.Helper()
	peer.once.Do(func() {
		peer.server.Stop()
		_ = peer.listener.Close()
		select {
		case <-peer.done:
		case <-time.After(testTimeout):
			t.Fatal("compatibility peer did not stop")
		}
	})
	return peer.service.stats()
}

func authorizeUnary(expected string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		values := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(expected)) != 1 {
			return nil, status.Error(codes.Unauthenticated, "local endpoint authentication failed")
		}
		return handler(ctx, request)
	}
}

type peerService struct {
	enginev1.UnimplementedEngineServiceServer

	mu       sync.Mutex
	observed peerStats
}

func (service *peerService) Initialize(
	_ context.Context,
	request *enginev1.InitializeRequest,
) (*enginev1.InitializeResponse, error) {
	service.mu.Lock()
	service.observed.Initialize++
	service.mu.Unlock()
	negotiation, failure := enginev1.PreflightInitialize(
		request,
		&commonv1.ProtocolRange{
			Minimum: &commonv1.ProtocolVersion{Major: 1, Minor: 0},
			Maximum: &commonv1.ProtocolVersion{Major: 1, Minor: 3},
		},
		&commonv1.BuildIdentity{
			Component: "released-semantic-peer", Version: "matrix", Commit: "public-module", GoVersion: "go1.26.5",
		},
		&commonv1.CapabilitySet{},
		protocolLimits(),
		&commonv1.Health{State: commonv1.HealthState_HEALTH_STATE_READY, Limits: protocolLimits()},
		&enginev1.DefinitionSet{Revision: "matrix-1", Definitions: []*enginev1.Definition{{
			Id: "default", Revision: "matrix-1", Model: "scripted", MaxTurns: 2,
		}}},
	)
	if failure != nil {
		return failure, nil
	}
	return enginev1.CompleteInitialize(negotiation, "released-semantic-client", 1), nil
}

func (service *peerService) StartRun(
	_ context.Context,
	request *enginev1.StartRunRequest,
) (*enginev1.StartRunResponse, error) {
	if err := enginev1.ValidateStartRunRequest(request, protocolLimits()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid semantic start")
	}
	if failure := enginev1.CheckClientOwnership(
		request.GetClientId(), request.GetOwnershipEpoch(), "released-semantic-client", 1,
	); failure != nil {
		return &enginev1.StartRunResponse{Status: failure}, nil
	}
	service.mu.Lock()
	service.observed.Start++
	service.mu.Unlock()
	return &enginev1.StartRunResponse{
		Status: commonv1.OKStatus(), RunId: "run-1", InitialSequence: 1, PlanId: "plan-1",
	}, nil
}

func (service *peerService) RespondInteraction(
	_ context.Context,
	request *enginev1.RespondInteractionRequest,
) (*enginev1.RespondInteractionResponse, error) {
	if err := enginev1.ValidateRespondInteractionRequest(request, protocolLimits()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid semantic response")
	}
	service.mu.Lock()
	service.observed.Respond++
	service.mu.Unlock()
	return &enginev1.RespondInteractionResponse{Status: commonv1.OKStatus(), Accepted: true}, nil
}

func (service *peerService) CancelRun(
	_ context.Context,
	request *enginev1.CancelRunRequest,
) (*enginev1.CancelRunResponse, error) {
	if err := enginev1.ValidateCancelRunRequest(request, protocolLimits()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid semantic cancellation")
	}
	service.mu.Lock()
	service.observed.Cancel++
	service.mu.Unlock()
	return &enginev1.CancelRunResponse{Status: commonv1.OKStatus(), CancellationRequested: true}, nil
}

func (service *peerService) stats() peerStats {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.observed
}

func protocolLimits() *commonv1.Limits {
	return &commonv1.Limits{
		MaxMessageBytes: 1 << 20, MaxCollectionItems: 64, MaxReplayEvents: 64,
		MaxReplayBytes: 1 << 20, MaxConcurrentStreams: 4, MaxActiveRuns: 4,
	}
}

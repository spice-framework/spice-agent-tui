package agentcompat

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"sync"

	commonv1 "github.com/spice-framework/spice-agent/common/v1"
	"github.com/spice-framework/spice-agent/daemon/endpoint"
	enginev1 "github.com/spice-framework/spice-agent/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PeerStats prove the exact semantic profile and mutations observed by the
// source-built compatibility peer.
type PeerStats struct {
	MaximumMinor   uint32 `json:"maximum_minor"`
	AttemptPresent bool   `json:"attempt_present"`
	Initialize     int    `json:"initialize"`
	Start          int    `json:"start"`
	Respond        int    `json:"respond"`
	Cancel         int    `json:"cancel"`
}

// ServePeer runs one bounded compatibility peer on a caller-owned local
// listener. Stop closes the server; Stats returns a race-safe snapshot.
func ServePeer(
	listener net.Listener,
	token endpoint.Token,
	maximumMinor uint32,
) (stop func(), stats func() PeerStats, err error) {
	if listener == nil || (maximumMinor != 2 && maximumMinor != 3) {
		return nil, nil, errors.New("compatibility peer configuration is invalid")
	}
	authorization, err := token.AuthorizationValue()
	if err != nil {
		return nil, nil, errors.New("compatibility peer credential is invalid")
	}
	service := &peerService{maximumMinor: maximumMinor}
	server := grpc.NewServer(grpc.UnaryInterceptor(authorizeUnary(authorization)))
	enginev1.RegisterEngineServiceServer(server, service)
	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			server.Stop()
			_ = listener.Close()
			<-done
		})
	}
	return stop, service.stats, nil
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

	mu           sync.Mutex
	maximumMinor uint32
	observed     PeerStats
}

func (service *peerService) Initialize(
	_ context.Context,
	request *enginev1.InitializeRequest,
) (*enginev1.InitializeResponse, error) {
	service.mu.Lock()
	service.observed.Initialize++
	service.observed.AttemptPresent = len(request.GetInitializationAttemptId()) != 0
	service.mu.Unlock()
	negotiation, failure := enginev1.PreflightInitialize(
		request,
		protocolRange(service.maximumMinor),
		serverBuild(),
		&commonv1.CapabilitySet{},
		protocolLimits(),
		protocolHealth(),
		definitionSet(),
	)
	if failure != nil {
		return failure, nil
	}
	return enginev1.CompleteInitialize(negotiation, "semantic-shell-client", 1), nil
}

func (service *peerService) StartRun(
	_ context.Context,
	request *enginev1.StartRunRequest,
) (*enginev1.StartRunResponse, error) {
	if err := enginev1.ValidateStartRunRequest(request, protocolLimits()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conformance start")
	}
	if failure := enginev1.CheckClientOwnership(
		request.GetClientId(), request.GetOwnershipEpoch(), "semantic-shell-client", 1,
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
		return nil, status.Error(codes.InvalidArgument, "invalid conformance response")
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
		return nil, status.Error(codes.InvalidArgument, "invalid conformance cancellation")
	}
	service.mu.Lock()
	service.observed.Cancel++
	service.mu.Unlock()
	return &enginev1.CancelRunResponse{Status: commonv1.OKStatus(), CancellationRequested: true}, nil
}

func (service *peerService) stats() PeerStats {
	service.mu.Lock()
	defer service.mu.Unlock()
	result := service.observed
	result.MaximumMinor = service.maximumMinor
	return result
}

func protocolRange(maximumMinor uint32) *commonv1.ProtocolRange {
	return &commonv1.ProtocolRange{
		Minimum: &commonv1.ProtocolVersion{Major: 1, Minor: 0},
		Maximum: &commonv1.ProtocolVersion{Major: 1, Minor: maximumMinor},
	}
}

func protocolLimits() *commonv1.Limits {
	return &commonv1.Limits{
		MaxMessageBytes: 1 << 20, MaxCollectionItems: 64, MaxReplayEvents: 64,
		MaxReplayBytes: 1 << 20, MaxConcurrentStreams: 4, MaxActiveRuns: 4,
	}
}

func protocolHealth() *commonv1.Health {
	return &commonv1.Health{State: commonv1.HealthState_HEALTH_STATE_READY, Limits: protocolLimits()}
}

func serverBuild() *commonv1.BuildIdentity {
	return &commonv1.BuildIdentity{
		Component: "semantic-shell-peer", Version: "source-built", Commit: "conformance", GoVersion: "go1.26.5",
	}
}

func definitionSet() *enginev1.DefinitionSet {
	return &enginev1.DefinitionSet{
		Revision: "fixture-1",
		Definitions: []*enginev1.Definition{{
			Id: "default", Revision: "fixture-1", Model: "scripted", MaxTurns: 2,
		}},
	}
}

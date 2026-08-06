package annotationtool

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spice-framework/spice-agent-tui/annotation/uitool"
	"github.com/spice-framework/spice/annotation/sdk"
	"github.com/spice-framework/spice/annotation/sdk/protocol"
)

func TestToolIdentitySortedDescriptionAndTypedDispatch(t *testing.T) {
	t.Parallel()
	implementation := New()
	identity, err := implementation.Initialize(t.Context(), protocol.InitializeParams{
		Protocol: sdk.ProtocolV1Alpha2, ToolPath: uitool.Path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Protocol != sdk.ProtocolV1Alpha2 || identity.ToolPath != uitool.Path || identity.ModulePath != modulePath {
		t.Fatalf("identity = %#v", identity)
	}
	description, err := implementation.Describe(t.Context(), protocol.DescribeParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(description.DescriptorPackages, []string{descriptorPackage}) || len(description.Handlers) != 2 {
		t.Fatalf("description = %#v", description)
	}
	names := make([]string, len(description.Handlers))
	for index, handler := range description.Handlers {
		names[index] = handler.Descriptor.Name
		if handler.Descriptor.Package != descriptorPackage ||
			!slices.Equal(handler.Capabilities, []string{metadataCapability, providerCapability}) {
			t.Fatalf("handler = %#v", handler)
		}
	}
	if !slices.Equal(names, []string{"UIRenderer", "UIShell"}) {
		t.Fatalf("handler order = %v", names)
	}
	description.Handlers[0].Capabilities[0] = "mutated"
	again, err := implementation.Describe(t.Context(), protocol.DescribeParams{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Handlers[0].Capabilities[0] == "mutated" {
		t.Fatal("Describe exposed mutable state")
	}
	result, err := implementation.Analyze(t.Context(), protocol.AnalyzeParams{
		Descriptor: sdk.Symbol{Package: descriptorPackage, Name: "UIShell"},
		Invocation: shellInvocation(t),
	})
	if err != nil || len(result.Contributions) != 2 {
		t.Fatalf("Analyze = %#v, %v", result, err)
	}
	for index, wire := range result.Contributions {
		decoded, decodeErr := protocol.DecodeContribution(wire)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if decoded.Kind != []sdk.ContributionKind{sdk.ContributionProvider, sdk.ContributionBeanMetadata}[index] {
			t.Fatalf("contribution %d kind = %s", index, decoded.Kind)
		}
	}
	if err := implementation.Shutdown(t.Context(), protocol.ShutdownParams{}); err != nil {
		t.Fatal(err)
	}
}

func TestToolFailsClosedOnIdentityDispatchAndCancellation(t *testing.T) {
	t.Parallel()
	var nilTool *Tool
	var nilContext context.Context
	if _, err := nilTool.Initialize(t.Context(), protocol.InitializeParams{}); err == nil {
		t.Fatal("nil tool Initialize succeeded")
	}
	if _, err := New().Describe(nilContext, protocol.DescribeParams{}); err == nil {
		t.Fatal("nil context Describe succeeded")
	}
	if _, err := nilTool.Analyze(t.Context(), protocol.AnalyzeParams{}); err == nil {
		t.Fatal("nil tool Analyze succeeded")
	}
	if err := nilTool.Shutdown(t.Context(), protocol.ShutdownParams{}); err == nil {
		t.Fatal("nil tool Shutdown succeeded")
	}
	implementation := New()
	for _, params := range []protocol.InitializeParams{
		{Protocol: sdk.ProtocolVersion("spice.annotation/v1alpha1"), ToolPath: uitool.Path},
		{Protocol: sdk.ProtocolV1Alpha2, ToolPath: "example.com/wrong"},
	} {
		if _, err := implementation.Initialize(t.Context(), params); err == nil {
			t.Fatalf("Initialize(%#v) succeeded", params)
		}
	}
	invocation := shellInvocation(t)
	if _, err := implementation.Analyze(t.Context(), protocol.AnalyzeParams{
		Descriptor: sdk.Symbol{Package: descriptorPackage, Name: "UIRenderer"}, Invocation: invocation,
	}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched dispatch = %v", err)
	}
	missing := invocation
	missing.DescriptorPackage = "example.com/missing"
	missing.DescriptorSymbol = "Missing"
	if _, err := implementation.Analyze(t.Context(), protocol.AnalyzeParams{
		Descriptor: sdk.Symbol{Package: "example.com/missing", Name: "Missing"}, Invocation: missing,
	}); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("missing dispatch = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := implementation.Analyze(ctx, protocol.AnalyzeParams{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Analyze cancellation = %v", err)
	}
	if err := implementation.Shutdown(ctx, protocol.ShutdownParams{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown cancellation = %v", err)
	}
}

func TestHandlerRegistrationsAreDeterministicAndValid(t *testing.T) {
	t.Parallel()
	first := handlerRegistrations()
	second := handlerRegistrations()
	if len(first) != len(second) {
		t.Fatal("handler registration count changed")
	}
	seen := make(map[string]struct{}, len(first))
	for index, registration := range first {
		if registration.description.Descriptor != second[index].description.Descriptor ||
			!slices.Equal(registration.description.Capabilities, second[index].description.Capabilities) {
			t.Fatalf("registration %d changed", index)
		}
		key := symbolKey(registration.description.Descriptor)
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate registration %q", key)
		}
		seen[key] = struct{}{}
		if registration.handle == nil {
			t.Fatalf("registration %q has nil handler", key)
		}
	}
}

func shellInvocation(t *testing.T) sdk.Invocation {
	t.Helper()
	name, err := json.Marshal("terminal")
	if err != nil {
		t.Fatal(err)
	}
	return sdk.Invocation{
		DescriptorPackage: descriptorPackage, DescriptorSymbol: "UIShell", CanonicalName: "ui.UIShell",
		Arguments: []sdk.InvocationArgument{{Name: "name", Kind: sdk.KindString, Value: name}},
		Declaration: sdk.Declaration{
			Target: sdk.TargetFunction, SymbolID: "example.com/application.NewTerminal", Name: "NewTerminal",
			PackagePath: "example.com/application", TypeID: "func() (github.com/spice-framework/spice-agent-tui.Shell, error)",
		},
		Facts: map[string]string{"symbol_kind": "function"},
	}
}

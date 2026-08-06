package ui_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	uiannotation "github.com/spice-framework/spice-agent-tui/annotation/ui"
	"github.com/spice-framework/spice-agent-tui/annotation/uitool"
	"github.com/spice-framework/spice/annotation/sdk"
	"github.com/spice-framework/spice/annotation/sdk/sdktest"
)

const descriptorPackage = "github.com/spice-framework/spice-agent-tui/annotation/ui"

func TestDescriptorsExposeCanonicalTypedHandlers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		symbol     string
		definition sdk.Definition
	}{
		{symbol: "UIRenderer", definition: uiannotation.UIRenderer()},
		{symbol: "UIShell", definition: uiannotation.UIShell()},
	} {
		t.Run(test.symbol, func(t *testing.T) {
			t.Parallel()
			if err := test.definition.Validate(); err != nil {
				t.Fatal(err)
			}
			if test.definition.Name != "ui."+test.symbol ||
				test.definition.Implementation.Tool != uitool.Path ||
				test.definition.Implementation.Protocol != sdk.ProtocolV1Alpha2 ||
				test.definition.Implementation.Handler == nil ||
				!slices.Equal(test.definition.Targets, []sdk.Target{sdk.TargetFunction}) {
				t.Fatalf("descriptor = %#v", test.definition)
			}
			if len(test.definition.Arguments) != 6 || test.definition.Arguments[0].Name != "name" ||
				!test.definition.Arguments[0].Required || len(test.definition.Examples) != 1 ||
				!strings.Contains(test.definition.Examples[0].Code, "func New") {
				t.Fatal("descriptor omitted explicit bean metadata or factory documentation")
			}
		})
	}
}

func TestHandlersContributeOnlyGenericProviderMetadata(t *testing.T) {
	for _, test := range []struct {
		symbol     string
		typeID     string
		definition sdk.Definition
	}{
		{symbol: "UIRenderer", typeID: "func(config example.com/ui.Config) github.com/spice-framework/spice-agent-tui.Renderer", definition: uiannotation.UIRenderer()},
		{symbol: "UIShell", typeID: "func(config example.com/ui.Config) github.com/spice-framework/spice-agent-tui.Shell", definition: uiannotation.UIShell()},
	} {
		t.Run(test.symbol, func(t *testing.T) {
			invocation := validInvocation(
				t, test.symbol, test.typeID,
				argument(t, "name", sdk.KindString, "terminal.default"),
				argument(t, "aliases", sdk.KindList, []string{"interactive"}),
				argument(t, "qualifiers", sdk.KindList, []string{"terminal", "default"}),
				argument(t, "primary", sdk.KindBoolean, true),
				argument(t, "order", sdk.KindInteger, int64(-100)),
			)
			sdktest.RunHandlerCases(
				t, test.definition,
				sdktest.HandlerCase{
					Name:       "generic contributions",
					Invocation: invocation,
					WantKinds: []sdk.ContributionKind{
						sdk.ContributionProvider,
						sdk.ContributionBeanMetadata,
					},
					Check: func(t *testing.T, result sdk.Result) {
						t.Helper()
						provider := result.Contributions[0].Provider
						metadata := result.Contributions[1].BeanMetadata
						if provider.Name != "terminal.default" || !slices.Equal(provider.Aliases, []string{"interactive"}) {
							t.Fatalf("provider = %#v", provider)
						}
						if !metadata.Primary || metadata.Fallback || metadata.Order == nil || *metadata.Order != -100 ||
							!slices.Equal(metadata.Qualifiers, []string{"terminal", "default"}) {
							t.Fatalf("metadata = %#v", metadata)
						}
					},
				},
				sdktest.HandlerCase{
					Name: "cancellation", Invocation: invocation, Canceled: true,
					WantErrorContains: context.Canceled.Error(),
				},
			)
		})
	}
}

func TestHandlersPreserveCompilerOwnedAliasCleanupAndErrorForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		symbol  string
		handler sdk.Handler
		typeID  string
	}{
		{symbol: "UIShell", handler: uiannotation.UIShellHandler, typeID: "func() github.com/spice-framework/spice-agent-tui.Shell"},
		{symbol: "UIShell", handler: uiannotation.UIShellHandler, typeID: "func() (github.com/spice-framework/spice-agent-tui.Shell, error)"},
		{symbol: "UIShell", handler: uiannotation.UIShellHandler, typeID: "func() (result github.com/spice-framework/spice-agent-tui.Shell, cleanup github.com/spice-framework/spice/lifecycle.Cleanup, err error)"},
		{symbol: "UIShell", handler: uiannotation.UIShellHandler, typeID: "func() example.com/application.ShellAlias"},
		{symbol: "UIRenderer", handler: uiannotation.UIRendererHandler, typeID: "func() (github.com/spice-framework/spice-agent-tui.Renderer, github.com/spice-framework/spice/lifecycle.Cleanup)"},
		{symbol: "UIRenderer", handler: uiannotation.UIRendererHandler, typeID: "func() example.com/application.RendererAlias"},
	} {
		invocation := validInvocation(t, test.symbol, test.typeID, argument(t, "name", sdk.KindString, "ui"))
		result, err := test.handler(t.Context(), invocation)
		if err != nil {
			t.Fatalf("%s(%q) = %v", test.symbol, test.typeID, err)
		}
		if len(result.Contributions) != 2 || result.Contributions[0].Kind != sdk.ContributionProvider ||
			result.Contributions[1].Kind != sdk.ContributionBeanMetadata {
			t.Fatalf("%s(%q) contributions = %#v", test.symbol, test.typeID, result.Contributions)
		}
	}
}

func TestHandlersNeverInterpretTypeIdentityStrings(t *testing.T) {
	t.Parallel()
	for _, typeID := range []string{"func()", "not a Go type", "func() example.com/application.Unrelated"} {
		invocation := validInvocation(t, "UIShell", typeID, argument(t, "name", sdk.KindString, "terminal"))
		result, err := uiannotation.UIShellHandler(t.Context(), invocation)
		if err != nil {
			t.Fatalf("handler interpreted TypeID %q: %v", typeID, err)
		}
		if len(result.Contributions) != 2 || result.Contributions[0].Kind != sdk.ContributionProvider ||
			result.Contributions[1].Kind != sdk.ContributionBeanMetadata {
			t.Fatalf("TypeID %q contributions = %#v", typeID, result.Contributions)
		}
	}
}

func TestHandlersRejectInvalidTargetAndSelectionMetadata(t *testing.T) {
	valid := validInvocation(
		t, "UIShell", "func() github.com/spice-framework/spice-agent-tui.Shell",
		argument(t, "name", sdk.KindString, "terminal"),
	)
	for _, test := range []struct {
		name       string
		invocation sdk.Invocation
		contains   string
	}{
		{name: "wrong descriptor", invocation: withDescriptor(valid, "UIRenderer"), contains: "received descriptor"},
		{name: "unexported", invocation: withDeclarationName(valid, "newShell"), contains: "must be exported"},
		{name: "method", invocation: withFact(valid, "receiver", "*Factory"), contains: "must not be a method"},
		{name: "wrong symbol kind", invocation: withFact(valid, "symbol_kind", "variable"), contains: "resolve to a function"},
		{name: "missing name", invocation: validInvocation(t, "UIShell", "func() example.com/Shell"), contains: "name\" is required"},
		{name: "noncanonical name", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "Terminal Shell")), contains: "not canonical"},
		{name: "duplicate alias", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "aliases", sdk.KindList, []string{"shell", "shell"})), contains: "duplicated"},
		{name: "alias equals name", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "aliases", sdk.KindList, []string{"terminal"})), contains: "duplicates its name"},
		{name: "duplicate qualifier", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "qualifiers", sdk.KindList, []string{"ui", "ui"})), contains: "duplicated"},
		{name: "primary fallback", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "primary", sdk.KindBoolean, true), argument(t, "fallback", sdk.KindBoolean, true)), contains: "both primary and fallback"},
		{name: "order high", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "order", sdk.KindInteger, int64(1_000_001))), contains: "between -1000000 and 1000000"},
		{name: "unsupported", invocation: withArguments(valid, argument(t, "name", sdk.KindString, "terminal"), argument(t, "mystery", sdk.KindString, "x")), contains: "unsupported argument"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := uiannotation.UIShellHandler(t.Context(), test.invocation)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want %q", err, test.contains)
			}
		})
	}
	var nilContext context.Context
	if _, err := uiannotation.UIShellHandler(nilContext, valid); err == nil {
		t.Fatal("nil handler context succeeded")
	}
}

func TestOrderBoundsAndCancellationIdentity(t *testing.T) {
	t.Parallel()
	for _, order := range []int64{-1_000_000, 1_000_000} {
		invocation := validInvocation(
			t, "UIRenderer", "func() example.com/RendererAlias",
			argument(t, "name", sdk.KindString, "renderer"), argument(t, "order", sdk.KindInteger, order),
		)
		result, err := uiannotation.UIRendererHandler(t.Context(), invocation)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Contributions[1].BeanMetadata.Order; got == nil || *got != order {
			t.Fatalf("order %d contribution = %v", order, got)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := uiannotation.UIRendererHandler(ctx, validInvocation(
		t, "UIRenderer", "func() example.com/Renderer",
		argument(t, "name", sdk.KindString, "renderer"),
	))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func FuzzUIShellHandler(f *testing.F) {
	f.Add("terminal", int64(0))
	f.Add("terminal.default", int64(-1_000_000))
	f.Fuzz(func(t *testing.T, name string, order int64) {
		invocation := validInvocation(
			t, "UIShell", "func() example.com/ShellAlias",
			argument(t, "name", sdk.KindString, name), argument(t, "order", sdk.KindInteger, order),
		)
		result, err := uiannotation.UIShellHandler(t.Context(), invocation)
		if err != nil {
			return
		}
		for index, contribution := range result.Contributions {
			if validationErr := contribution.Validate(); validationErr != nil {
				t.Fatalf("contribution %d: %v", index, validationErr)
			}
		}
	})
}

func validInvocation(t *testing.T, symbol, typeID string, arguments ...sdk.InvocationArgument) sdk.Invocation {
	t.Helper()
	return sdk.Invocation{
		DescriptorPackage: descriptorPackage,
		DescriptorSymbol:  symbol,
		CanonicalName:     "ui." + symbol,
		Arguments:         arguments,
		Declaration: sdk.Declaration{
			Target: sdk.TargetFunction, SymbolID: "example.com/application.New" + symbol,
			Name: "New" + symbol, PackagePath: "example.com/application", TypeID: typeID,
		},
		Facts: map[string]string{"symbol_kind": "function"},
	}
}

func argument(t *testing.T, name string, kind sdk.Kind, value any) sdk.InvocationArgument {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return sdk.InvocationArgument{Name: name, Kind: kind, Value: encoded}
}

func withDescriptor(invocation sdk.Invocation, symbol string) sdk.Invocation {
	invocation.DescriptorSymbol = symbol
	return invocation
}

func withDeclarationName(invocation sdk.Invocation, name string) sdk.Invocation {
	invocation.Declaration.Name = name
	return invocation
}

func withFact(invocation sdk.Invocation, name, value string) sdk.Invocation {
	invocation.Facts = map[string]string{name: value}
	return invocation
}

func withArguments(invocation sdk.Invocation, arguments ...sdk.InvocationArgument) sdk.Invocation {
	invocation.Arguments = arguments
	return invocation
}

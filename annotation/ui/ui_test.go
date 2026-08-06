package ui_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
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
		canonical := publicResultTypeID(test.symbol)
		invocation = withFunctionResults(t, invocation, []sdk.FunctionResultFact{
			{
				TypeID:             "example.com/application." + test.symbol + "Alias",
				CanonicalTypeID:    canonical,
				Kind:               sdk.GoTypeInterface,
				NamedOriginPackage: "github.com/spice-framework/spice-agent-tui",
				NamedOriginName:    strings.TrimPrefix(test.symbol, "UI"),
			},
			{
				TypeID:             "github.com/spice-framework/spice/lifecycle.Cleanup",
				CanonicalTypeID:    "github.com/spice-framework/spice/lifecycle.Cleanup",
				Kind:               sdk.GoTypeSignature,
				NamedOriginPackage: "github.com/spice-framework/spice/lifecycle",
				NamedOriginName:    "Cleanup",
			},
			{
				TypeID:          "error",
				CanonicalTypeID: "error",
				Kind:            sdk.GoTypeInterface,
				NamedOriginName: "error",
			},
		})
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

func TestHandlersFailClosedOnGenericProviderResultFacts(t *testing.T) {
	valid := validInvocation(
		t,
		"UIShell",
		"this declaration TypeID is deliberately not parsed",
		argument(t, "name", sdk.KindString, "terminal"),
	)
	wrongInterface := sdk.FunctionResultFact{
		TypeID:             "example.com/application.ShellWrapper",
		CanonicalTypeID:    "example.com/application.ShellWrapper",
		Kind:               sdk.GoTypeInterface,
		NamedOriginPackage: "example.com/application",
		NamedOriginName:    "ShellWrapper",
	}
	tests := []struct {
		name       string
		invocation sdk.Invocation
		contains   string
	}{
		{
			name:       "absent",
			invocation: withoutFunctionResultFacts(valid),
			contains:   "requires generic function result facts",
		},
		{
			name: "malformed count",
			invocation: withRawFunctionResultFacts(
				valid,
				map[string]string{sdk.FunctionResultCountFact: "not-a-count"},
			),
			contains: "canonical non-negative integer",
		},
		{
			name: "unknown reserved fact",
			invocation: withRawFunctionResultFacts(valid, map[string]string{
				sdk.FunctionResultCountFact:                    "0",
				sdk.FunctionResultFactNamespace + "unexpected": "value",
			}),
			contains: "too many reserved entries",
		},
		{
			name:       "no results",
			invocation: withFunctionResults(t, valid, nil),
			contains:   "one provider value",
		},
		{
			name:       "defined wrapper interface",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{wrongInterface}),
			contains:   "exact canonical type",
		},
		{
			name: "anonymous interface",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{{
				TypeID:          "interface{ Run(context.Context) error }",
				CanonicalTypeID: "interface{ Run(context.Context) error }",
				Kind:            sdk.GoTypeInterface,
			}}),
			contains: "exact canonical type",
		},
		{
			name: "wrong named interface",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{{
				TypeID:             "example.com/application.OtherShell",
				CanonicalTypeID:    "example.com/application.OtherShell",
				Kind:               sdk.GoTypeInterface,
				NamedOriginPackage: "example.com/application",
				NamedOriginName:    "OtherShell",
			}}),
			contains: "exact canonical type",
		},
		{
			name: "forged origin",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{{
				TypeID:             "github.com/spice-framework/spice-agent-tui.Shell",
				CanonicalTypeID:    "github.com/spice-framework/spice-agent-tui.Shell",
				Kind:               sdk.GoTypeInterface,
				NamedOriginPackage: "example.com/forged",
				NamedOriginName:    "Shell",
			}}),
			contains: "must originate from",
		},
		{
			name: "concrete struct",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{{
				TypeID:             "example.com/application.TerminalShell",
				CanonicalTypeID:    "example.com/application.TerminalShell",
				Kind:               sdk.GoTypeStruct,
				NamedOriginPackage: "example.com/application",
				NamedOriginName:    "TerminalShell",
			}}),
			contains: "effective Go kind interface",
		},
		{
			name: "invalid auxiliary result",
			invocation: withFunctionResults(t, valid, []sdk.FunctionResultFact{
				{
					TypeID:             "github.com/spice-framework/spice-agent-tui.Shell",
					CanonicalTypeID:    "github.com/spice-framework/spice-agent-tui.Shell",
					Kind:               sdk.GoTypeInterface,
					NamedOriginPackage: "github.com/spice-framework/spice-agent-tui",
					NamedOriginName:    "Shell",
				},
				{TypeID: "string", CanonicalTypeID: "string", Kind: sdk.GoTypeBasic},
			}),
			contains: "factory results must be",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := uiannotation.UIShellHandler(t.Context(), test.invocation)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want %q", err, test.contains)
			}
		})
	}
}

func TestHandlersAcceptAliasesByCanonicalIdentity(t *testing.T) {
	for _, symbol := range []string{"UIShell", "UIRenderer"} {
		invocation := validInvocation(
			t,
			symbol,
			"not parsed",
			argument(t, "name", sdk.KindString, "ui"),
		)
		invocation = withFunctionResults(t, invocation, []sdk.FunctionResultFact{{
			TypeID:             "example.com/application." + symbol + "Alias",
			CanonicalTypeID:    publicResultTypeID(symbol),
			Kind:               sdk.GoTypeInterface,
			NamedOriginPackage: "github.com/spice-framework/spice-agent-tui",
			NamedOriginName:    strings.TrimPrefix(symbol, "UI"),
		}})
		definition := uiannotation.UIShell()
		if symbol == "UIRenderer" {
			definition = uiannotation.UIRenderer()
		}
		result, err := definition.Implementation.Handler(t.Context(), invocation)
		if err != nil || len(result.Contributions) != 2 {
			t.Fatalf("%s alias result = %#v, %v", symbol, result, err)
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
	invocation := sdk.Invocation{
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
	return withFunctionResults(t, invocation, []sdk.FunctionResultFact{{
		TypeID:             publicResultTypeID(symbol),
		CanonicalTypeID:    publicResultTypeID(symbol),
		Kind:               sdk.GoTypeInterface,
		NamedOriginPackage: "github.com/spice-framework/spice-agent-tui",
		NamedOriginName:    strings.TrimPrefix(symbol, "UI"),
	}})
}

func publicResultTypeID(symbol string) string {
	return "github.com/spice-framework/spice-agent-tui." + strings.TrimPrefix(symbol, "UI")
}

func withFunctionResults(
	t *testing.T,
	invocation sdk.Invocation,
	results []sdk.FunctionResultFact,
) sdk.Invocation {
	t.Helper()
	facts, err := sdk.EncodeFunctionResultFacts(results)
	if err != nil {
		t.Fatal(err)
	}
	return withRawFunctionResultFacts(invocation, facts)
}

func withRawFunctionResultFacts(
	invocation sdk.Invocation,
	resultFacts map[string]string,
) sdk.Invocation {
	invocation = withoutFunctionResultFacts(invocation)
	maps.Copy(invocation.Facts, resultFacts)
	return invocation
}

func withoutFunctionResultFacts(invocation sdk.Invocation) sdk.Invocation {
	facts := make(map[string]string, len(invocation.Facts))
	for name, value := range invocation.Facts {
		if !strings.HasPrefix(name, sdk.FunctionResultFactNamespace) {
			facts[name] = value
		}
	}
	invocation.Facts = facts
	return invocation
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
	facts := make(map[string]string, len(invocation.Facts)+1)
	maps.Copy(facts, invocation.Facts)
	facts[name] = value
	invocation.Facts = facts
	return invocation
}

func withArguments(invocation sdk.Invocation, arguments ...sdk.InvocationArgument) sdk.Invocation {
	invocation.Arguments = arguments
	return invocation
}

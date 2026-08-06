package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spice-framework/spice-agent-tui/annotation/uitool"
	"github.com/spice-framework/spice/annotation/sdk"
	"github.com/spice-framework/spice/annotation/sdk/protocol"
)

func TestRunServesOnlyFramedProtocolOnStdout(t *testing.T) {
	requests := new(bytes.Buffer)
	writeRequest(t, requests, 1, "initialize", protocol.InitializeParams{Protocol: sdk.ProtocolV1Alpha2, ToolPath: uitool.Path})
	writeRequest(t, requests, 2, "describe", protocol.DescribeParams{})
	name, err := json.Marshal("terminal")
	if err != nil {
		t.Fatal(err)
	}
	writeRequest(t, requests, 3, "analyze", protocol.AnalyzeParams{
		Descriptor: sdk.Symbol{Package: "github.com/spice-framework/spice-agent-tui/annotation/ui", Name: "UIShell"},
		Invocation: sdk.Invocation{
			DescriptorPackage: "github.com/spice-framework/spice-agent-tui/annotation/ui",
			DescriptorSymbol:  "UIShell", CanonicalName: "ui.UIShell",
			Arguments: []sdk.InvocationArgument{{Name: "name", Kind: sdk.KindString, Value: name}},
			Declaration: sdk.Declaration{
				Target: sdk.TargetFunction, SymbolID: "example.com/application.NewTerminal", Name: "NewTerminal",
				PackagePath: "example.com/application", TypeID: "func() github.com/spice-framework/spice-agent-tui.Shell",
			},
			Facts: protocolFunctionResultFacts(t),
		},
	})
	writeRequest(t, requests, 4, "shutdown", protocol.ShutdownParams{})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if code := run([]string{"spice-agent-tui-annotations", "--spice-stdio"}, requests, stdout, stderr); code != 0 {
		t.Fatalf("run code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("successful stderr = %q", stderr.String())
	}
	reader := bufio.NewReader(stdout)
	for id := uint64(1); id <= 4; id++ {
		var response protocol.Response
		if err := protocol.ReadMessage(reader, &response); err != nil {
			t.Fatal(err)
		}
		if response.JSONRPC != "2.0" || response.ID != id || response.Error != nil || len(response.Result) == 0 {
			t.Fatalf("response %d = %#v", id, response)
		}
	}
	if trailing, err := io.ReadAll(reader); err != nil || len(trailing) != 0 {
		t.Fatalf("unframed stdout = %q, %v", trailing, err)
	}
}

func protocolFunctionResultFacts(t *testing.T) map[string]string {
	t.Helper()
	facts, err := sdk.EncodeFunctionResultFacts([]sdk.FunctionResultFact{{
		TypeID:             "example.com/application.ShellAlias",
		CanonicalTypeID:    "github.com/spice-framework/spice-agent-tui.Shell",
		Kind:               sdk.GoTypeInterface,
		NamedOriginPackage: "github.com/spice-framework/spice-agent-tui",
		NamedOriginName:    "Shell",
	}})
	if err != nil {
		t.Fatal(err)
	}
	facts["symbol_kind"] = "function"
	return facts
}

func TestRunRejectsWrongModeAndProtocolFailureWithoutStdoutNoise(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if code := run([]string{"spice-agent-tui-annotations"}, bytes.NewReader(nil), stdout, stderr); code != 2 {
		t.Fatalf("wrong-mode code = %d", code)
	}
	if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("requires --spice-stdio")) {
		t.Fatalf("wrong-mode stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"spice-agent-tui-annotations", "--spice-stdio"}, bytes.NewBufferString("Content-Length: 1\r\n\r\n{"), stdout, stderr); code != 1 {
		t.Fatalf("invalid-protocol code = %d", code)
	}
	if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("spice-agent-tui-annotations:")) {
		t.Fatalf("invalid-protocol stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if code := run(nil, nil, io.Discard, errorWriter{}); code != 1 {
		t.Fatalf("stderr failure code = %d", code)
	}
}

func TestRunFramesMissingResultFactFailure(t *testing.T) {
	requests := new(bytes.Buffer)
	writeRequest(t, requests, 1, "initialize", protocol.InitializeParams{
		Protocol: sdk.ProtocolV1Alpha2,
		ToolPath: uitool.Path,
	})
	name, err := json.Marshal("terminal")
	if err != nil {
		t.Fatal(err)
	}
	writeRequest(t, requests, 2, "analyze", protocol.AnalyzeParams{
		Descriptor: sdk.Symbol{
			Package: "github.com/spice-framework/spice-agent-tui/annotation/ui",
			Name:    "UIShell",
		},
		Invocation: sdk.Invocation{
			DescriptorPackage: "github.com/spice-framework/spice-agent-tui/annotation/ui",
			DescriptorSymbol:  "UIShell",
			CanonicalName:     "ui.UIShell",
			Arguments: []sdk.InvocationArgument{{
				Name: "name", Kind: sdk.KindString, Value: name,
			}},
			Declaration: sdk.Declaration{
				Target: sdk.TargetFunction, SymbolID: "example.com/application.NewTerminal",
				Name: "NewTerminal", PackagePath: "example.com/application",
			},
			Facts: map[string]string{"symbol_kind": "function"},
		},
	})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if code := run(
		[]string{"spice-agent-tui-annotations", "--spice-stdio"},
		requests,
		stdout,
		stderr,
	); code != 0 {
		t.Fatalf("run code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	reader := bufio.NewReader(stdout)
	var initialized protocol.Response
	if err := protocol.ReadMessage(reader, &initialized); err != nil {
		t.Fatal(err)
	}
	var failed protocol.Response
	if err := protocol.ReadMessage(reader, &failed); err != nil {
		t.Fatal(err)
	}
	if failed.ID != 2 || failed.Error == nil ||
		!strings.Contains(failed.Error.Message, "requires generic function result facts") {
		t.Fatalf("failure response = %#v", failed)
	}
	if trailing, readErr := io.ReadAll(reader); readErr != nil || len(trailing) != 0 {
		t.Fatalf("unframed stdout = %q, %v", trailing, readErr)
	}
}

func writeRequest(t *testing.T, destination io.Writer, id uint64, method string, params any) {
	t.Helper()
	content, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteMessage(destination, protocol.Request{JSONRPC: "2.0", ID: id, Method: method, Params: content}); err != nil {
		t.Fatal(err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

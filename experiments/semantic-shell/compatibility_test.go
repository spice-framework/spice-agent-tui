package semanticshell

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const (
	releasedModuleVersion = "v0.1.0-preview.1"
	releasedModuleSum     = "h1:r7MbWvF6UrvAV+CZwk6LNSbrZEV/VVIE6B0pmOmsbOA="
	releasedGoModSum      = "h1:ZbrQXPLB1QWPuSa8fW/PsUu/LeAhFjKF4VyEL3LyrzI="
	releasedOriginCommit  = "0e6cfb1a58b8bb2cf711fd36dfa66a8cd4e9867f"
	conformanceVersion    = "v0.1.0-preview.5.0.20260810055539-b205307d3b5f"
	conformanceSum        = "h1:QveBVnwI0IBh/vxAfJpsvX6ktCH/PXAOF9+CXW7QTAs="
	conformanceGoModSum   = "h1:pbhYOeNgn4pCIhEmcdbjnFjJijY4ZSLM8ZHxaF2dxz0="
	conformanceCommit     = "b205307d3b5fb262401c77d1af902b1ce926d49a"
)

type compatibilityManifest struct {
	Schema                int                     `json:"schema"`
	Status                string                  `json:"status"`
	Module                string                  `json:"module"`
	Go                    string                  `json:"go"`
	Toolchain             string                  `json:"toolchain"`
	Dependency            compatibilityDependency `json:"dependency"`
	ConformanceDependency compatibilityDependency `json:"conformance_dependency"`
	EngineProtocol        engineProtocolContract  `json:"engine_protocol_contract"`
	PublicSeams           []string                `json:"public_seams"`
	ForbiddenDependencies []string                `json:"forbidden_dependencies"`
	RuntimeNetwork        bool                    `json:"runtime_network"`
	ReplaceDirectives     bool                    `json:"replace_directives"`
	PromotionGate         string                  `json:"promotion_gate"`
	Deletion              string                  `json:"deletion"`
}

type engineProtocolContract struct {
	Schema               string                  `json:"schema"`
	Profiles             []engineProtocolProfile `json:"profiles"`
	Platforms            []string                `json:"platforms"`
	Transport            string                  `json:"transport"`
	ReleasedBinaryMatrix string                  `json:"released_binary_matrix"`
}

type engineProtocolProfile struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	Initialization string `json:"initialization"`
}

type compatibilityDependency struct {
	Module       string `json:"module"`
	Version      string `json:"version"`
	Sum          string `json:"sum"`
	GoModSum     string `json:"go_mod_sum"`
	OriginCommit string `json:"origin_commit"`
}

func TestCompatibilityManifestPinsReleasedPublicSurface(t *testing.T) {
	t.Parallel()
	directory := experimentDirectory(t)
	content, err := os.ReadFile(filepath.Join(directory, "compatibility.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest compatibilityManifest
	if err = decoder.Decode(&manifest); err != nil {
		t.Fatalf("decode compatibility manifest: %v", err)
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("compatibility manifest has trailing JSON: %v", err)
	}
	wantDependency := compatibilityDependency{
		Module:       "github.com/spice-framework/spice-agent-tui",
		Version:      releasedModuleVersion,
		Sum:          releasedModuleSum,
		GoModSum:     releasedGoModSum,
		OriginCommit: releasedOriginCommit,
	}
	wantConformance := compatibilityDependency{
		Module:       "github.com/spice-framework/spice-agent",
		Version:      conformanceVersion,
		Sum:          conformanceSum,
		GoModSum:     conformanceGoModSum,
		OriginCommit: conformanceCommit,
	}
	wantProtocol := engineProtocolContract{
		Schema: "spice.agent.engine.compatibility/v1alpha1",
		Profiles: []engineProtocolProfile{
			{Name: "previous-semantics", Version: "1.2.0", Initialization: "legacy"},
			{Name: "current", Version: "1.3.0", Initialization: "exact-replay"},
		},
		Platforms:            []string{"linux/amd64", "windows/amd64"},
		Transport:            "real-local-ipc",
		ReleasedBinaryMatrix: "not-claimed",
	}
	wantSeams := []string{"Intent", "Session", "Session.Perform", "Session.Receive", "SessionUpdate"}
	wantForbidden := []string{"github.com/charmbracelet/bubbletea/v2", "terminal plugin APIs"}
	if manifest.Schema != 1 || manifest.Status != "experimental" ||
		manifest.Module != "github.com/spice-framework/spice-agent-tui/experiments/semantic-shell" ||
		manifest.Go != "1.26.0" || manifest.Toolchain != "go1.26.5" ||
		manifest.Dependency != wantDependency || manifest.ConformanceDependency != wantConformance ||
		!protocolContractEqual(manifest.EngineProtocol, wantProtocol) || !slices.Equal(manifest.PublicSeams, wantSeams) ||
		!slices.Equal(manifest.ForbiddenDependencies, wantForbidden) || manifest.RuntimeNetwork ||
		manifest.ReplaceDirectives || manifest.PromotionGate == "" || manifest.Deletion == "" {
		t.Fatalf("compatibility manifest drifted: %#v", manifest)
	}
}

func protocolContractEqual(left, right engineProtocolContract) bool {
	return left.Schema == right.Schema && slices.Equal(left.Profiles, right.Profiles) &&
		slices.Equal(left.Platforms, right.Platforms) && left.Transport == right.Transport &&
		left.ReleasedBinaryMatrix == right.ReleasedBinaryMatrix
}

func TestModuleGraphHasNoReplaceAndNoBubbleTea(t *testing.T) {
	t.Parallel()
	directory := experimentDirectory(t)
	goMod, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(goMod)
	if strings.Contains(text, "replace ") || strings.Contains(text, "github.com/charmbracelet/bubbletea") ||
		!strings.Contains(text, "github.com/spice-framework/spice-agent-tui "+releasedModuleVersion) ||
		!strings.Contains(text, "github.com/spice-framework/spice-agent "+conformanceVersion) {
		t.Fatalf("unexpected experiment module graph:\n%s", text)
	}
	modules, err := os.ReadFile(filepath.Join(directory, "vendor", "modules.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modules), "# github.com/spice-framework/spice-agent-tui "+releasedModuleVersion) ||
		!strings.Contains(string(modules), "# github.com/spice-framework/spice-agent "+conformanceVersion) ||
		strings.Contains(string(modules), "github.com/charmbracelet/bubbletea") {
		t.Fatalf("unexpected vendored module graph:\n%s", modules)
	}
}

func TestProductionShellDoesNotOwnAgentTransport(t *testing.T) {
	t.Parallel()
	directory := experimentDirectory(t)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(content), "github.com/spice-framework/spice-agent/") {
			t.Fatalf("production semantic shell imports Agent transport in %s", entry.Name())
		}
	}
}

func experimentDirectory(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate compatibility test")
	}
	return filepath.Dir(file)
}

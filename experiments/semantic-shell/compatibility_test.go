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
)

type compatibilityManifest struct {
	Schema                int                     `json:"schema"`
	Status                string                  `json:"status"`
	Module                string                  `json:"module"`
	Go                    string                  `json:"go"`
	Toolchain             string                  `json:"toolchain"`
	Dependency            compatibilityDependency `json:"dependency"`
	PublicSeams           []string                `json:"public_seams"`
	ForbiddenDependencies []string                `json:"forbidden_dependencies"`
	RuntimeNetwork        bool                    `json:"runtime_network"`
	ReplaceDirectives     bool                    `json:"replace_directives"`
	PromotionGate         string                  `json:"promotion_gate"`
	Deletion              string                  `json:"deletion"`
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
	wantSeams := []string{"Intent", "Session", "Session.Perform", "Session.Receive", "SessionUpdate"}
	wantForbidden := []string{"github.com/charmbracelet/bubbletea/v2", "terminal plugin APIs"}
	if manifest.Schema != 1 || manifest.Status != "experimental" ||
		manifest.Module != "github.com/spice-framework/spice-agent-tui/experiments/semantic-shell" ||
		manifest.Go != "1.26.0" || manifest.Toolchain != "go1.26.5" ||
		manifest.Dependency != wantDependency || !slices.Equal(manifest.PublicSeams, wantSeams) ||
		!slices.Equal(manifest.ForbiddenDependencies, wantForbidden) || manifest.RuntimeNetwork ||
		manifest.ReplaceDirectives || manifest.PromotionGate == "" || manifest.Deletion == "" {
		t.Fatalf("compatibility manifest drifted: %#v", manifest)
	}
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
		!strings.Contains(text, "github.com/spice-framework/spice-agent-tui "+releasedModuleVersion) {
		t.Fatalf("unexpected experiment module graph:\n%s", text)
	}
	modules, err := os.ReadFile(filepath.Join(directory, "vendor", "modules.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modules), "# github.com/spice-framework/spice-agent-tui "+releasedModuleVersion) ||
		strings.Contains(string(modules), "github.com/charmbracelet/bubbletea") {
		t.Fatalf("unexpected vendored module graph:\n%s", modules)
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

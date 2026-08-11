package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNetworkAllowedOnlyForBootstrap(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"fast", "check", "fmt", "benchmark", "verify", "unknown"} {
		if networkAllowed(mode) {
			t.Fatalf("networkAllowed(%q) = true", mode)
		}
	}
	if !networkAllowed("tools-bootstrap") {
		t.Fatal("networkAllowed(tools-bootstrap) = false")
	}
}

func TestBenchmarkArgumentsAreDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	want := []string{
		"test",
		"-run=^$",
		"-bench=^Benchmark(SessionEventIngestionAndScreen|RenderScreen|ScriptSessionReceiveCanceled|VirtualTerminalFrameCapture|ModelSnapshotUpdateAndView|FixedRendererRender)$",
		"-benchmem",
		"-benchtime=500x",
		"-count=5",
		"-cpu=1",
		"./tuittest",
		"./internal/presentation",
	}
	if got := benchmarkArguments(); !slices.Equal(got, want) {
		t.Fatalf("benchmark arguments = %q, want %q", got, want)
	}
}

func TestFuzzArgumentsAreDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	want := []string{
		"test", "-run=^$", "-fuzz=^FuzzTraceCanonicalReplay$", "-fuzztime=1s", "-parallel=1", "./tuittest",
	}
	if got := fuzzArguments("FuzzTraceCanonicalReplay"); !slices.Equal(got, want) {
		t.Fatalf("fuzz arguments = %q, want %q", got, want)
	}
}

func TestSpiceCompositionVerificationRetainsRootModulithBoundary(t *testing.T) {
	t.Parallel()
	valid := spiceCompositionVerifyArguments()
	if err := validateSpiceCompositionVerifyArguments(valid); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		arguments []string
	}{
		{name: "missing root module", arguments: []string{"tool", spiceTool, "verify", "./internal/acceptance/composition"}},
		{name: "missing fixture", arguments: []string{"tool", spiceTool, "verify", "."}},
		{name: "substituted fixture", arguments: []string{"tool", spiceTool, "verify", ".", "./internal/other"}},
		{name: "profile override", arguments: append(slices.Clone(valid), "--profile=none")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := validateSpiceCompositionVerifyArguments(test.arguments); err == nil ||
				!strings.Contains(err.Error(), "root Modulith boundary") {
				t.Fatalf("validateSpiceCompositionVerifyArguments() error = %v", err)
			}
		})
	}
}

func TestSemanticShellBenchmarkArgumentsAreDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	want := []string{
		"test", "-run=^$", "-bench=^Benchmark", "-benchmem", "-benchtime=500x", "-count=5", "-cpu=1", ".",
	}
	if got := semanticShellBenchmarkArguments(); !slices.Equal(got, want) {
		t.Fatalf("semantic shell benchmark arguments = %q, want %q", got, want)
	}
}

func TestRepositoryPortabilityRequiresLFAndExplicitToolBootstrap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".gitattributes", requiredGitAttributes)
	writeFile(t, root, ".github/workflows/ci.yml", `steps:
  - run: go run ./internal/qualitygate -mode=tools-bootstrap
  - run: go run ./internal/qualitygate -mode=verify
  - env:
      SPICE_TUI_VISUAL_ARTIFACT_DIR: artifacts
    run: go test -run TestWriteDeterministicVisualArtifacts ./tuittest
  - uses: actions/upload-artifact@`+uploadArtifactCommit+`
    with:
      name: spice-tui-visuals
`)
	if err := checkRepositoryPortability(root); err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".github/workflows/ci.yml", strings.Replace(
		string(workflow), uploadArtifactCommit, strings.Repeat("0", 40), 1,
	))
	if err := checkRepositoryPortability(root); err == nil || !strings.Contains(err.Error(), "upload") {
		t.Fatalf("floating artifact action error = %v", err)
	}

	writeFile(t, root, ".github/workflows/ci.yml", `steps:
  - run: go run ./internal/qualitygate -mode=verify
`)
	if err := checkRepositoryPortability(root); err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("missing bootstrap error = %v", err)
	}
}

func TestReleaseWorkflowRequiresExactKeylessBoundary(t *testing.T) {
	t.Parallel()
	const immediatePriorWorkflowCommit = "0fcd43dc8b41fad56c231d0e136ad8c762276ed5"
	tests := []struct {
		name     string
		workflow string
		wantErr  string
		omit     bool
	}{
		{name: "valid", workflow: validReleaseWorkflow()},
		{name: "missing", wantErr: "read release workflow", omit: true},
		{
			name:     "immediate prior authority",
			workflow: strings.ReplaceAll(validReleaseWorkflow(), releaseWorkflowCommit, immediatePriorWorkflowCommit),
			wantErr:  "uses:",
		},
		{
			name:     "wrong reusable pin",
			workflow: strings.Replace(validReleaseWorkflow(), releaseWorkflowCommit, strings.Repeat("0", 40), 1),
			wantErr:  "uses:",
		},
		{
			name: "wrong attested workflow pin",
			workflow: strings.Replace(
				validReleaseWorkflow(),
				"workflow_commit: "+releaseWorkflowCommit,
				"workflow_commit: "+strings.Repeat("0", 40),
				1,
			),
			wantErr: "workflow_commit:",
		},
		{
			name:     "wrong module",
			workflow: strings.Replace(validReleaseWorkflow(), modulePath, "example.com/wrong", 1),
			wantErr:  "module:",
		},
		{
			name:     "legacy workflow",
			workflow: strings.Replace(validReleaseWorkflow(), "go-module-release.yml", "library-release.yml", 1),
			wantErr:  "go-module-release.yml",
		},
		{
			name:     "inherited secrets",
			workflow: validReleaseWorkflow() + "    secrets: inherit\n",
			wantErr:  "secrets:",
		},
		{
			name:     "named signing secret",
			workflow: validReleaseWorkflow() + "    secrets:\n      SPICE_LIBRARY_RELEASE_SIGNING_KEY: value\n",
			wantErr:  "secrets:",
		},
		{
			name: "extra permission",
			workflow: strings.Replace(
				validReleaseWorkflow(),
				"      contents: write\n",
				"      contents: write\n      packages: write\n",
				1,
			),
			wantErr: "permission ceiling",
		},
		{
			name:     "missing permission",
			workflow: strings.Replace(validReleaseWorkflow(), "      attestations: write\n", "", 1),
			wantErr:  "attestations: write",
		},
		{
			name:     "extra permission block",
			workflow: validReleaseWorkflow() + "permissions: read-all\n",
			wantErr:  "permission blocks",
		},
		{
			name:     "extra job",
			workflow: validReleaseWorkflow() + "  publish-again:\n    uses: example.invalid/workflow.yml@deadbeef\n",
			wantErr:  "only the release job",
		},
		{
			name:     "local steps",
			workflow: validReleaseWorkflow() + "    steps:\n      - run: echo unsafe\n",
			wantErr:  "steps:",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if !test.omit {
				writeFile(t, root, ".github/workflows/release.yml", test.workflow)
			}
			err := checkReleaseWorkflow(root)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("checkReleaseWorkflow() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestExactGoExecutable(t *testing.T) {
	t.Parallel()
	if goExecutableName("windows") != "go.exe" || goExecutableName("linux") != "go" {
		t.Fatal("go executable name is not platform-correct")
	}
	actualName := filepath.Base(exactGoExecutable())
	if (actualName != "go" && actualName != "go.exe") || filepath.Base(filepath.Dir(exactGoExecutable())) != "bin" ||
		qualityExecutable("go") != exactGoExecutable() || qualityExecutable("gofumpt") != "gofumpt" {
		t.Fatalf("exact Go executable = %q", exactGoExecutable())
	}
}

func TestBootstrapDownloadArguments(t *testing.T) {
	t.Parallel()
	moduleFile := filepath.Join("private", "graph.mod")
	want := "mod download -modfile=" + moduleFile + " all"
	if got := strings.Join(bootstrapDownloadArguments(moduleFile), " "); got != want {
		t.Fatalf("bootstrapDownloadArguments() = %q, want %q", got, want)
	}
}

func TestBootstrapPreservesRepositoryOnSuccessFailureAndCancellation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		runnerErr error
	}{
		{name: "success"},
		{name: "failure", runnerErr: errors.New("download failed")},
		{name: "cancellation", runnerErr: context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := bootstrapFixture(t, true)
			before, err := sourceTreeDigests(root)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if errors.Is(test.runnerErr, context.Canceled) {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			var calls [][]string
			runner := func(callContext context.Context, directory string, arguments ...string) error {
				if directory != root && directory != filepath.Join(root, "tools") &&
					directory != filepath.Join(root, "experiments", "semantic-shell") {
					t.Fatalf("unexpected directory %q", directory)
				}
				calls = append(calls, append([]string(nil), arguments...))
				if errors.Is(test.runnerErr, context.Canceled) {
					return callContext.Err()
				}
				return test.runnerErr
			}
			err = bootstrapDependencies(ctx, root, runner)
			if !errors.Is(err, test.runnerErr) {
				t.Fatalf("bootstrapDependencies() error = %v, want %v", err, test.runnerErr)
			}
			after, digestErr := sourceTreeDigests(root)
			if digestErr != nil || !maps.Equal(before, after) {
				t.Fatalf("repository changed: %v", digestErr)
			}
			wantCalls := 3
			if test.runnerErr != nil {
				wantCalls = 1
			}
			if len(calls) != wantCalls {
				t.Fatalf("bootstrap calls = %d, want %d", len(calls), wantCalls)
			}
			for _, arguments := range calls {
				if len(arguments) != 4 || arguments[0] != "mod" || arguments[1] != "download" ||
					!strings.HasPrefix(arguments[2], "-modfile=") || arguments[3] != "all" {
					t.Fatalf("unexpected bootstrap arguments: %q", arguments)
				}
				if strings.HasPrefix(strings.TrimPrefix(arguments[2], "-modfile="), root) {
					t.Fatalf("temporary modfile is inside repository: %q", arguments[2])
				}
			}
		})
	}
}

func TestBootstrapDetectsRepositoryMutation(t *testing.T) {
	t.Parallel()
	root := bootstrapFixture(t, false)
	err := bootstrapDependencies(context.Background(), root, func(_ context.Context, directory string, _ ...string) error {
		return os.WriteFile(filepath.Join(directory, "unexpected"), []byte("mutation"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "modified the repository") {
		t.Fatalf("bootstrapDependencies() error = %v", err)
	}
}

func TestBootstrapAllowsMissingToolsModule(t *testing.T) {
	t.Parallel()
	root := bootstrapFixture(t, false)
	calls := 0
	err := bootstrapDependencies(context.Background(), root, func(_ context.Context, _ string, _ ...string) error {
		calls++
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("bootstrapDependencies() = calls %d, error %v", calls, err)
	}
}

func TestBootstrapRequiresSemanticShellModule(t *testing.T) {
	t.Parallel()
	root := bootstrapFixture(t, false)
	if err := os.Remove(filepath.Join(root, "experiments", "semantic-shell", "go.mod")); err != nil {
		t.Fatal(err)
	}
	err := bootstrapDependencies(context.Background(), root, func(context.Context, string, ...string) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "semantic-shell") {
		t.Fatalf("bootstrapDependencies() error = %v", err)
	}
}

func TestSemanticShellCoverageAndVendorFailClosed(t *testing.T) {
	t.Parallel()
	if err := validateSemanticShellCoverage(minimumSemanticShellCoverage); err != nil {
		t.Fatal(err)
	}
	if err := validateSemanticShellCoverage(minimumSemanticShellCoverage - 0.1); err == nil {
		t.Fatal("semantic shell coverage below the floor was accepted")
	}
	digest := sha256.Sum256([]byte("exact vendor"))
	exact := map[string][sha256.Size]byte{"modules.txt": digest}
	if err := validateSemanticShellVendor(exact, maps.Clone(exact)); err != nil {
		t.Fatal(err)
	}
	mutated := maps.Clone(exact)
	mutated["modules.txt"] = sha256.Sum256([]byte("mutated vendor"))
	if err := validateSemanticShellVendor(exact, mutated); err == nil {
		t.Fatal("semantic shell vendor drift was accepted")
	}
}

func TestBootstrapEnvironmentRejectsCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "must-not-leak")
	t.Setenv("SPICE_TEST_TOKEN", "must-not-leak")
	environment := strings.Join(environment(true, nil), "\n")
	for _, required := range []string{
		"GOAUTH=off", "GOPROXY=https://proxy.golang.org", "GOSUMDB=sum.golang.org",
	} {
		if !strings.Contains(environment, required) {
			t.Fatalf("bootstrap environment lacks %q:\n%s", required, environment)
		}
	}
	if strings.Contains(environment, "must-not-leak") {
		t.Fatalf("bootstrap environment contains an application credential:\n%s", environment)
	}
}

func bootstrapFixture(t *testing.T, tools bool) string {
	t.Helper()
	root := t.TempDir()
	modules := []string{root, filepath.Join(root, "experiments", "semantic-shell")}
	if tools {
		modules = append(modules, filepath.Join(root, "tools"))
	}
	for _, directory := range modules {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "go.sum"), []byte("fixture sum\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestValidateCompatibility(t *testing.T) {
	t.Parallel()
	valid := validCompatibilityJSON()
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "valid", content: valid},
		{name: "malformed", content: `{`, wantErr: "decode"},
		{name: "unknown", content: strings.Replace(valid, `}`, `,"extra":true}`, 1), wantErr: "unknown field"},
		{name: "trailing", content: valid + `{}`, wantErr: "trailing"},
		{name: "wrong Go", content: strings.Replace(valid, "1.26.5", "1.26.4", 1), wantErr: "local UI values"},
		{name: "premature client", content: strings.Replace(valid, `"spice_agent_client":null`, `"spice_agent_client":"v1"`, 1), wantErr: "null client"},
		{name: "wrong UI values", content: strings.Replace(valid, `"spice_agent_ui_values":"v0.1.0-dev"`, `"spice_agent_ui_values":"v1"`, 1), wantErr: "local UI values"},
		{name: "stale Spice core", content: strings.Replace(valid, coreVersion, "v0.1.0-preview.2", 1), wantErr: "core/toolchain"},
		{name: "stale toolchain", content: strings.Replace(valid, toolchainVersion, "v0.1.0-preview.1.0.20260806203056-d0b9ac086bd6", 1), wantErr: "core/toolchain"},
		{name: "wrong Spice core", content: strings.Replace(valid, `"spice_core":"`+coreVersion+`"`, `"spice_core":null`, 1), wantErr: "core/toolchain"},
		{name: "wrong toolchain", content: strings.Replace(valid, `"spice_toolchain":"`+toolchainVersion+`"`, `"spice_toolchain":null`, 1), wantErr: "core/toolchain"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateCompatibility([]byte(test.content))
			if test.wantErr == "" && err != nil {
				t.Fatalf("validateCompatibility() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("validateCompatibility() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestCheckIdentityAndToolPins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	validMod := validIdentityGoMod()
	writeFile(t, root, "go.mod", validMod)
	writeFile(t, root, "compatibility.json", validCompatibilityJSON())
	writeFile(t, root, ".gitattributes", requiredGitAttributes)
	writeFile(t, root, ".github/workflows/ci.yml", `steps:
  - run: go run ./internal/qualitygate -mode=tools-bootstrap
  - run: go run ./internal/qualitygate -mode=verify
  - env:
      SPICE_TUI_VISUAL_ARTIFACT_DIR: artifacts
    run: go test -run TestWriteDeterministicVisualArtifacts ./tuittest
  - uses: actions/upload-artifact@`+uploadArtifactCommit+`
    with:
      name: spice-tui-visuals
`)
	writeFile(t, root, ".github/workflows/release.yml", validReleaseWorkflow())
	writeStyleContractFixture(t, root)
	writeFile(t, root, "tools/go.mod", strings.Join([]string{
		"github.com/golangci/golangci-lint/v2 v2.12.2",
		"github.com/securego/gosec/v2 v2.28.0",
		"go.uber.org/nilaway v0.0.0-20260724203407-f4f8ac24c032",
		"golang.org/x/tools v0.48.0",
		"golang.org/x/vuln v1.1.4",
		"mvdan.cc/gofumpt v0.10.0",
		"\t" + styleTool + "\n",
		toolchainModule + " " + toolchainVersion,
	}, "\n"))
	if err := checkIdentity(root); err != nil {
		t.Fatalf("checkIdentity() error = %v", err)
	}
	writeFile(
		t,
		root,
		"go.mod",
		strings.Replace(validMod, "charm.land/bubbletea/v2", "github.com/charmbracelet/bubbletea/v2", 1),
	)
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "charm.land/bubbletea") {
		t.Fatalf("checkIdentity(noncanonical Bubble Tea) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", strings.Replace(validMod, "3755ebad01b1", "000000000000", 1))
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "github.com/charmbracelet/x/vt") {
		t.Fatalf("checkIdentity(stale virtual terminal) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", strings.Replace(validMod, "github.com/Kodecable/crosspty v1.1.0", "github.com/Kodecable/crosspty v1.0.0", 1))
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "github.com/Kodecable/crosspty") {
		t.Fatalf("checkIdentity(stale native terminal) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", strings.Replace(validMod, "golang.org/x/image v0.39.0", "golang.org/x/image v0.38.0", 1))
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "golang.org/x/image") {
		t.Fatalf("checkIdentity(stale image renderer) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", strings.Replace(validMod, "golang.org/x/text v0.36.0", "golang.org/x/text v0.35.0", 1))
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "golang.org/x/text") {
		t.Fatalf("checkIdentity(stale font parser) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", validMod+"\nreplace charm.land/bubbletea/v2 => ../local\n")
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "unreplaced") {
		t.Fatalf("checkIdentity(replaced Bubble Tea) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", validMod)
	writeFile(t, root, "tools/go.mod", strings.Replace(
		strings.Join([]string{
			"github.com/golangci/golangci-lint/v2 v2.12.2",
			"github.com/securego/gosec/v2 v2.28.0",
			"go.uber.org/nilaway v0.0.0-20260724203407-f4f8ac24c032",
			"golang.org/x/tools v0.48.0",
			"golang.org/x/vuln v1.1.4",
			"mvdan.cc/gofumpt v0.10.0",
			"\t" + styleTool + "\n",
			toolchainModule + " " + toolchainVersion,
		}, "\n"),
		styleTool,
		"github.com/spice-framework/toolchain/cmd/other",
		1,
	))
	if err := checkIdentity(root); err == nil || !strings.Contains(err.Error(), "spicestyle") {
		t.Fatalf("checkIdentity(stale style tool) error = %v", err)
	}
	writeFile(t, root, "tools/go.mod", "module missing")
	if err := checkIdentity(root); err == nil || !strings.Contains(err.Error(), "missing exact pin") {
		t.Fatalf("checkIdentity() error = %v, want pin diagnostic", err)
	}
}

func writeStyleContractFixture(t *testing.T, root string) {
	t.Helper()
	source, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"CODE_STYLE.md", ".spice/style.json"} {
		content, readErr := os.ReadFile(filepath.Join(source, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		writeFile(t, root, path, string(content))
	}
	writeFile(t, root, "internal/acceptance/composition/doc.go", "package composition\n")
	writeFile(
		t,
		root,
		"internal/spicegen/compositionproof/generated.go",
		"// Code generated by Spice. DO NOT EDIT.\npackage compositionproof\n",
	)
}

func validCompatibilityJSON() string {
	return `{"schema":1,"go":"1.26.5","spice_agent_client":null,` +
		`"spice_agent_ui_values":"v0.1.0-dev","spice_core":"` + coreVersion +
		`","spice_toolchain":"` + toolchainVersion + `"}`
}

func validIdentityGoMod() string {
	return "module " + modulePath + "\n\ngo 1.26.0\n\ntoolchain go1.26.5\n\n" +
		"tool (\n\t" + annotationTool + "\n\t" + coreAnnotationTool + "\n\t" + spiceTool + "\n)\n\n" +
		"require (\n\tcharm.land/bubbletea/v2 v2.0.8\n" +
		"\tgithub.com/Kodecable/crosspty v1.1.0\n" +
		"\tgithub.com/charmbracelet/x/ansi v0.11.7\n" +
		"\tgithub.com/charmbracelet/x/term v0.2.2\n" +
		"\tgithub.com/charmbracelet/x/vt v0.0.0-20260803091719-3755ebad01b1\n\t" + coreModule + " " + coreVersion + "\n" +
		"\tgolang.org/x/image v0.39.0\n)\n\n" +
		"require (\n\t" + toolchainModule + " " + toolchainVersion + " // indirect\n" +
		"\tgolang.org/x/text v0.36.0 // indirect\n)\n"
}

func validReleaseWorkflow() string {
	return `name: Release

on:
  push:
    tags:
      - "v[0-9]*.[0-9]*.[0-9]*"

permissions: {}

jobs:
  release:
    name: Keylessly attest and publish
    permissions:
      contents: write
      id-token: write
      attestations: write
      artifact-metadata: write
    uses: spice-framework/.github/.github/workflows/go-module-release.yml@` + releaseWorkflowCommit + `
    with:
      module: ` + modulePath + `
      workflow_commit: ` + releaseWorkflowCommit + `
`
}

func TestGoFilesAndTreeDigests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "main.go", "package fixture")
	writeFile(t, root, "internal/value.go", "package internal")
	writeFile(t, root, "tools/ignored.go", "package ignored")
	writeFile(t, root, "vendor/ignored.go", "package ignored")
	files, err := goFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || !slices.IsSorted(files) {
		t.Fatalf("goFiles() = %v", files)
	}
	first, err := treeDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := treeDigests(root)
	if err != nil || !mapsEqual(first, second) {
		t.Fatalf("treeDigests() deterministic = %v, %v", mapsEqual(first, second), err)
	}
	missing, err := treeDigests(filepath.Join(root, "missing"))
	if err != nil || len(missing) != 0 {
		t.Fatalf("treeDigests(missing) = %v, %v", missing, err)
	}
}

func TestCoverageParsingAndModes(t *testing.T) {
	t.Parallel()
	packages := []string{
		modulePath,
		modulePath + "/internal/spicegen",
		modulePath + "/internal/spicegen/compositionproof",
		modulePath + "/terminal",
	}
	if got, want := handwrittenCoveragePackages(packages), []string{modulePath, modulePath + "/terminal"}; !slices.Equal(got, want) {
		t.Fatalf("handwrittenCoveragePackages() = %v, want %v", got, want)
	}
	percentage, err := totalCoverage("total: (statements) 91.5%")
	if err != nil || percentage != 91.5 {
		t.Fatalf("totalCoverage() = %v, %v", percentage, err)
	}
	if _, err := totalCoverage("invalid"); err == nil {
		t.Fatal("totalCoverage(invalid) error = nil")
	}
	if err := run(t.Context(), t.TempDir(), "unknown"); err == nil || !strings.Contains(err.Error(), "unknown mode") {
		t.Fatalf("run(unknown) error = %v", err)
	}
}

func TestEnvironmentIsolationAndCancellation(t *testing.T) {
	t.Parallel()
	offline := environment(false, map[string]string{"GOFLAGS": "-mod=vendor"})
	if !slices.Contains(offline, "GOPROXY=off") || !slices.Contains(offline, "GOWORK=off") ||
		!slices.Contains(offline, "GOFLAGS=-mod=vendor") {
		t.Fatalf("offline environment = %v", offline)
	}
	online := environment(true, nil)
	if slices.Contains(online, "GOPROXY=off") {
		t.Fatalf("online environment unexpectedly disables proxy: %v", online)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := command(ctx, t.TempDir(), nil, "go", "version")
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("command(cancelled) error = %v", err)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mapsEqual(left, right map[string][sha256.Size]byte) bool {
	return maps.Equal(left, right)
}

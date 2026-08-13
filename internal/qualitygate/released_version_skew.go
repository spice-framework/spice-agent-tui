package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	releasedVersionSkewManifest = "compatibility/released-client-matrix.json"
	releasedVersionSkewSchema   = "spice.agent.tui.released-client-skew/v1alpha1"
	releasedVersionSkewWorkflow = ".github/workflows/released-version-skew.yml"
	releasedVersionSkewRunner   = "internal/qualitygate/testdata/releasedversionskew"
	grpcVersion                 = "v1.83.0"
	checkoutActionCommit        = "d23441a48e516b6c34aea4fa41551a30e30af803"
	setupGoActionCommit         = "924ae3a1cded613372ab5595356fb5720e22ba16"
)

type releasedVersionSkewManifestValue struct {
	Schema                       string                          `json:"schema"`
	Status                       string                          `json:"status"`
	Go                           string                          `json:"go"`
	ArtifactKind                 string                          `json:"artifact_kind"`
	PrebuiltExecutableMatrix     bool                            `json:"prebuilt_executable_matrix"`
	BuildSource                  string                          `json:"build_source"`
	Isolation                    releasedVersionSkewIsolation    `json:"isolation"`
	Clients                      []releasedVersionSkewGeneration `json:"clients"`
	Peers                        []releasedVersionSkewGeneration `json:"peers"`
	RunnerDependencies           []releasedVersionSkewDependency `json:"runner_dependencies"`
	Platforms                    []string                        `json:"platforms"`
	Lanes                        []releasedVersionSkewLane       `json:"lanes"`
	RequiredCases                []string                        `json:"required_cases"`
	PreservedSpecializedEvidence []string                        `json:"preserved_specialized_evidence"`
	RunnerSource                 string                          `json:"runner_source"`
	RunnerSourceSHA256           string                          `json:"runner_source_sha256"`
	Workflow                     string                          `json:"workflow"`
}

type releasedVersionSkewIsolation struct {
	GoWorkOff        bool `json:"gowork_off"`
	FreshModuleCache bool `json:"fresh_module_caches"`
	NoReplace        bool `json:"no_replace"`
	PublicProxy      bool `json:"public_proxy"`
	PublicSumDB      bool `json:"public_sumdb"`
}

type releasedVersionSkewGeneration struct {
	Role      string `json:"role"`
	Module    string `json:"module"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	ModuleSum string `json:"module_sum"`
	GoModSum  string `json:"go_mod_sum"`
}

type releasedVersionSkewDependency struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

type releasedVersionSkewLane struct {
	ID     string `json:"id"`
	Client string `json:"client"`
	Peer   string `json:"peer"`
}

type downloadedModule struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Sum      string `json:"Sum"`
	GoModSum string `json:"GoModSum"`
	Origin   struct {
		Hash string `json:"Hash"`
	} `json:"Origin"`
}

func checkReleasedVersionSkewContract(root string) error {
	manifest, content, err := readReleasedVersionSkewManifest(root)
	if err != nil {
		return err
	}
	if err = validateReleasedVersionSkewManifest(root, manifest, content); err != nil {
		return err
	}
	return checkReleasedVersionSkewWorkflow(root, manifest)
}

func readReleasedVersionSkewManifest(
	root string,
) (releasedVersionSkewManifestValue, []byte, error) {
	path := filepath.Join(root, filepath.FromSlash(releasedVersionSkewManifest))
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository-owned manifest path.
	if err != nil {
		return releasedVersionSkewManifestValue{}, nil, fmt.Errorf("read released version-skew manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest releasedVersionSkewManifestValue
	if err = decoder.Decode(&manifest); err != nil {
		return releasedVersionSkewManifestValue{}, nil, fmt.Errorf("decode released version-skew manifest: %w", err)
	}
	var trailing json.RawMessage
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return releasedVersionSkewManifestValue{}, nil, errors.New("released version-skew manifest has trailing JSON")
	}
	return manifest, content, nil
}

func validateReleasedVersionSkewManifest(
	root string,
	manifest releasedVersionSkewManifestValue,
	content []byte,
) error {
	if err := validateReleasedVersionSkewIdentity(manifest); err != nil {
		return err
	}
	if err := validateReleasedVersionSkewGenerations(manifest); err != nil {
		return err
	}
	if err := validateReleasedVersionSkewEvidence(root, manifest); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(content, canonical) {
		return errors.New("released version-skew manifest is not canonical deterministic JSON")
	}
	return nil
}

func validateReleasedVersionSkewIdentity(manifest releasedVersionSkewManifestValue) error {
	if manifest.Schema != releasedVersionSkewSchema || manifest.Status != "hosted-linux-windows-gate" ||
		manifest.Go != requiredGoVersion || manifest.ArtifactKind != "public-go-modules-source-built-test" ||
		manifest.PrebuiltExecutableMatrix || manifest.BuildSource != "public-proxy-and-sumdb" {
		return errors.New("released version-skew manifest identity drifted")
	}
	if manifest.Isolation != (releasedVersionSkewIsolation{
		GoWorkOff: true, FreshModuleCache: true, NoReplace: true, PublicProxy: true, PublicSumDB: true,
	}) {
		return errors.New("released version-skew isolation must require fresh public unreplaced module builds")
	}
	return nil
}

func validateReleasedVersionSkewGenerations(manifest releasedVersionSkewManifestValue) error {
	wantClients := []releasedVersionSkewGeneration{
		{
			Role: "previous", Module: modulePath, Version: "v0.1.0-preview.1",
			Commit:    "0e6cfb1a58b8bb2cf711fd36dfa66a8cd4e9867f",
			ModuleSum: "h1:r7MbWvF6UrvAV+CZwk6LNSbrZEV/VVIE6B0pmOmsbOA=",
			GoModSum:  "h1:ZbrQXPLB1QWPuSa8fW/PsUu/LeAhFjKF4VyEL3LyrzI=",
		},
		{
			Role: "current", Module: modulePath, Version: "v0.1.0-preview.2",
			Commit:    "11187714648fcd7832d18d70bca74e3079a38ecc",
			ModuleSum: "h1:WOyasmHWrLQHfXxFG20D/AufbJ9RnPrZvemaa+/mmac=",
			GoModSum:  "h1:2EvMOqKnzX4wztrURPg7Q9pXiqQR2wsE0vv6a93BQt4=",
		},
	}
	wantPeers := []releasedVersionSkewGeneration{
		{
			Role: "previous", Module: "github.com/spice-framework/spice-agent", Version: "v0.1.0-preview.5",
			Commit:    "3e8fe6406171a7e7f1765311a4fa7fc3b878e425",
			ModuleSum: "h1:rGND9DYx3pssliD1tZQOvPDOZ5GVfQLDc7VJQI3HLOM=",
			GoModSum:  "h1:pbhYOeNgn4pCIhEmcdbjnFjJijY4ZSLM8ZHxaF2dxz0=",
		},
		{
			Role: "current", Module: "github.com/spice-framework/spice-agent", Version: "v0.1.0-preview.6",
			Commit:    "f771caa3b150d87845417c4e26938e2a889441a6",
			ModuleSum: "h1:XJKJge+xWP/FLNoL1/rXq8z8tdu/5iEkKfmu1dTgFms=",
			GoModSum:  "h1:pbhYOeNgn4pCIhEmcdbjnFjJijY4ZSLM8ZHxaF2dxz0=",
		},
	}
	wantLanes := []releasedVersionSkewLane{
		{ID: "previous-client-previous-peer", Client: "previous", Peer: "previous"},
		{ID: "previous-client-current-peer", Client: "previous", Peer: "current"},
		{ID: "current-client-previous-peer", Client: "current", Peer: "previous"},
		{ID: "current-client-current-peer", Client: "current", Peer: "current"},
	}
	if !slices.Equal(manifest.Clients, wantClients) || !slices.Equal(manifest.Peers, wantPeers) ||
		!slices.Equal(manifest.RunnerDependencies, []releasedVersionSkewDependency{{
			Module: "google.golang.org/grpc", Version: grpcVersion,
		}}) || !slices.Equal(manifest.Platforms, []string{"linux/amd64", "windows/amd64"}) ||
		!slices.Equal(manifest.Lanes, wantLanes) {
		return errors.New("released version-skew generations, platforms, dependencies, or lanes drifted")
	}
	if !slices.Equal(manifest.RequiredCases, []string{
		"authenticated-initialize", "semantic-submit", "semantic-respond", "semantic-cancel",
		"wrong-token-refusal", "bounded-cleanup",
	}) {
		return errors.New("released version-skew required cases drifted")
	}
	return nil
}

func validateReleasedVersionSkewEvidence(root string, manifest releasedVersionSkewManifestValue) error {
	wantEvidence := []string{
		"experiments/semantic-shell/protocol_adapter_test.go:TestSemanticShellAgentProtocolCompatibility",
		"internal/presentation/model_test.go:TestModelAppliesRevisionedStreamingSnapshotsAndBoundsActivity",
		"tuittest/trace_test.go:TestLifecycleTraceIsCanonicalDeterministicAndGolden",
		"tuittest/accessibility_lifecycle_test.go:TestKeyboardOnlyLifecycleCoversEditingHistoryAndEveryIntent",
	}
	if !slices.Equal(manifest.PreservedSpecializedEvidence, wantEvidence) ||
		manifest.RunnerSource != releasedVersionSkewRunner || manifest.Workflow != releasedVersionSkewWorkflow {
		return errors.New("released version-skew evidence ownership drifted")
	}
	for _, evidence := range manifest.PreservedSpecializedEvidence {
		path, testName, found := strings.Cut(evidence, ":")
		if !found || path == "" || testName == "" {
			return fmt.Errorf("invalid specialized evidence reference %q", evidence)
		}
		evidenceContent, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) // #nosec G304 -- manifest is frozen above.
		if err != nil || !bytes.Contains(evidenceContent, []byte("func "+testName+"(")) {
			return fmt.Errorf("specialized evidence %q is missing", evidence)
		}
	}
	digest, err := releasedVersionSkewSourceDigest(filepath.Join(root, filepath.FromSlash(manifest.RunnerSource)))
	if err != nil {
		return err
	}
	if manifest.RunnerSourceSHA256 != digest {
		return fmt.Errorf("released version-skew runner digest = %s, want %s", digest, manifest.RunnerSourceSHA256)
	}
	if err = validateReleasedVersionSkewRunnerSource(root, manifest.RunnerSource); err != nil {
		return err
	}
	return nil
}

func validateReleasedVersionSkewRunnerSource(root, relative string) error {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative), "matrix_test.go")) // #nosec G304 -- frozen relative path.
	if err != nil {
		return fmt.Errorf("read released version-skew runner: %w", err)
	}
	for _, required := range []string{
		`github.com/spice-framework/spice-agent-tui`,
		`github.com/spice-framework/spice-agent/client`,
		`github.com/spice-framework/spice-agent/client/grpcclient`,
		`github.com/spice-framework/spice-agent/daemon/localipc`,
		`github.com/spice-framework/spice-agent/engine/v1`,
		"TestReleasedClientPeerSemanticContract",
	} {
		if !bytes.Contains(content, []byte(required)) {
			return fmt.Errorf("released version-skew runner lacks %q", required)
		}
	}
	for _, forbidden := range []string{
		"github.com/spice-framework/spice-agent/internal/", "replace ", "go.work", "os/exec",
	} {
		if bytes.Contains(content, []byte(forbidden)) {
			return fmt.Errorf("released version-skew runner contains forbidden %q", forbidden)
		}
	}
	return nil
}

func releasedVersionSkewSourceDigest(root string) (string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk released version-skew runner: %w", err)
	}
	slices.Sort(paths)
	digest := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path) // #nosec G304 -- path came from the bounded runner tree.
		if err != nil {
			return "", err
		}
		if _, err = io.WriteString(digest, filepath.ToSlash(relative)); err != nil {
			return "", err
		}
		if _, err = digest.Write([]byte{0}); err != nil {
			return "", err
		}
		if _, err = digest.Write(content); err != nil {
			return "", err
		}
		if _, err = digest.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func checkReleasedVersionSkewWorkflow(
	root string,
	manifest releasedVersionSkewManifestValue,
) error {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Workflow))) // #nosec G304 -- frozen workflow path.
	if err != nil {
		return fmt.Errorf("read released version-skew workflow: %w", err)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	for _, required := range []string{
		"permissions:\n  contents: read",
		"uses: actions/checkout@" + checkoutActionCommit,
		"uses: actions/setup-go@" + setupGoActionCommit,
		"go-version: 1.26.5",
		"os: [ubuntu-latest, windows-latest]",
		"go run ./internal/qualitygate -mode=released-version-skew -lane=${{ matrix.lane }}",
	} {
		if strings.Count(text, required) != 1 {
			return fmt.Errorf("released version-skew workflow must contain exactly one %q", required)
		}
	}
	for _, lane := range manifest.Lanes {
		if strings.Count(text, "          - "+lane.ID+"\n") != 1 {
			return fmt.Errorf("released version-skew workflow must contain lane %q exactly once", lane.ID)
		}
	}
	if strings.Count(text, "uses:") != 2 || strings.Contains(text, "pull_request_target") ||
		strings.Contains(text, "secrets:") || strings.Contains(text, "replace ") {
		return errors.New("released version-skew workflow exceeds its read-only public-module boundary")
	}
	return nil
}

func runReleasedVersionSkew(ctx context.Context, root, laneID string) (returnErr error) {
	manifest, content, err := readReleasedVersionSkewManifest(root)
	if err != nil {
		return err
	}
	if err = validateReleasedVersionSkewManifest(root, manifest, content); err != nil {
		return err
	}
	if err = checkReleasedVersionSkewWorkflow(root, manifest); err != nil {
		return err
	}
	lane, clientGeneration, peerGeneration, err := selectReleasedVersionSkewLane(manifest, laneID)
	if err != nil {
		return err
	}
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if !slices.Contains(manifest.Platforms, platform) {
		return fmt.Errorf("released version-skew platform %q is not supported", platform)
	}
	temporary, err := os.MkdirTemp("", "spice-agent-tui-released-skew-*")
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, removeReleasedVersionSkewTree(temporary)) }()
	moduleRoot := filepath.Join(temporary, "module")
	if err = copyReleasedVersionSkewRunner(
		filepath.Join(root, filepath.FromSlash(manifest.RunnerSource)), moduleRoot,
	); err != nil {
		return err
	}
	moduleContent := releasedVersionSkewGoMod(clientGeneration, peerGeneration)
	if strings.Contains(moduleContent, "replace ") {
		return errors.New("released version-skew generated module contains a replacement")
	}
	if err = os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte(moduleContent), 0o600); err != nil {
		return err
	}
	environment := releasedVersionSkewEnvironment(temporary)
	if err = verifyReleasedDownload(ctx, moduleRoot, environment, clientGeneration); err != nil {
		return err
	}
	if err = verifyReleasedDownload(ctx, moduleRoot, environment, peerGeneration); err != nil {
		return err
	}
	if err = releasedVersionSkewCommand(
		ctx, moduleRoot, environment,
		"test", "-mod=mod", "-trimpath", "-count=1", "-timeout=2m", ".",
	); err != nil {
		return err
	}
	_, err = fmt.Fprintf(
		output,
		"released version-skew lane %s passed on %s (%s, %s)\n",
		lane.ID,
		platform,
		clientGeneration.Version,
		peerGeneration.Version,
	)
	return err
}

func removeReleasedVersionSkewTree(path string) error {
	root, err := os.OpenRoot(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		mode := fs.FileMode(0o600)
		if entry.IsDir() {
			mode = 0o700
		}
		// #nosec G122 -- this root is a private freshly created temporary tree; root-scoped access prevents escape.
		return root.Chmod(name, mode)
	})
	closeErr := root.Close()
	return errors.Join(walkErr, closeErr, os.RemoveAll(path))
}

func selectReleasedVersionSkewLane(
	manifest releasedVersionSkewManifestValue,
	laneID string,
) (releasedVersionSkewLane, releasedVersionSkewGeneration, releasedVersionSkewGeneration, error) {
	for _, lane := range manifest.Lanes {
		if lane.ID != laneID {
			continue
		}
		clientGeneration, clientFound := generationByRole(manifest.Clients, lane.Client)
		peerGeneration, peerFound := generationByRole(manifest.Peers, lane.Peer)
		if !clientFound || !peerFound {
			break
		}
		return lane, clientGeneration, peerGeneration, nil
	}
	return releasedVersionSkewLane{}, releasedVersionSkewGeneration{}, releasedVersionSkewGeneration{},
		fmt.Errorf("unknown released version-skew lane %q", laneID)
}

func generationByRole(
	generations []releasedVersionSkewGeneration,
	role string,
) (releasedVersionSkewGeneration, bool) {
	for _, generation := range generations {
		if generation.Role == role {
			return generation, true
		}
	}
	return releasedVersionSkewGeneration{}, false
}

func releasedVersionSkewGoMod(
	clientGeneration releasedVersionSkewGeneration,
	peerGeneration releasedVersionSkewGeneration,
) string {
	return fmt.Sprintf(`module spice.local/released-version-skew

go 1.26.0

toolchain go1.26.5

require (
	%s %s
	%s %s
	google.golang.org/grpc %s
)
`, clientGeneration.Module, clientGeneration.Version, peerGeneration.Module, peerGeneration.Version, grpcVersion)
}

func copyReleasedVersionSkewRunner(source, destination string) (returnErr error) {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, sourceRoot.Close()) }()
	destinationRoot, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, destinationRoot.Close()) }()
	return fs.WalkDir(sourceRoot.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return destinationRoot.MkdirAll(path, 0o700)
		}
		content, err := sourceRoot.ReadFile(path)
		if err != nil {
			return err
		}
		return destinationRoot.WriteFile(path, content, 0o600)
	})
}

func releasedVersionSkewEnvironment(temporary string) []string {
	return environment(true, map[string]string{
		"GOCACHE":     filepath.Join(temporary, "build-cache"),
		"GOFLAGS":     "",
		"GOMODCACHE":  filepath.Join(temporary, "module-cache"),
		"GOPATH":      filepath.Join(temporary, "go-path"),
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
	})
}

func verifyReleasedDownload(
	ctx context.Context,
	directory string,
	environmentValues []string,
	generation releasedVersionSkewGeneration,
) error {
	content, err := releasedVersionSkewCapture(
		ctx, directory, environmentValues, "mod", "download", "-json", generation.Module+"@"+generation.Version,
	)
	if err != nil {
		return err
	}
	var downloaded downloadedModule
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err = decoder.Decode(&downloaded); err != nil {
		return fmt.Errorf("decode public module metadata for %s: %w", generation.Module, err)
	}
	if downloaded.Path != generation.Module || downloaded.Version != generation.Version ||
		downloaded.Sum != generation.ModuleSum || downloaded.GoModSum != generation.GoModSum ||
		downloaded.Origin.Hash != generation.Commit {
		return fmt.Errorf("public module metadata for %s@%s drifted: %#v", generation.Module, generation.Version, downloaded)
	}
	return nil
}

func releasedVersionSkewCapture(
	ctx context.Context,
	directory string,
	environmentValues []string,
	arguments ...string,
) ([]byte, error) {
	// #nosec G204,G702 -- executable is exact Go and arguments are frozen module coordinates.
	command := exec.CommandContext(ctx, exactGoExecutable(), arguments...)
	command.Dir = directory
	command.Env = environmentValues
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func releasedVersionSkewCommand(
	ctx context.Context,
	directory string,
	environmentValues []string,
	arguments ...string,
) error {
	// #nosec G204,G702 -- executable is exact Go and arguments are repository-owned.
	command := exec.CommandContext(ctx, exactGoExecutable(), arguments...)
	command.Dir = directory
	command.Env = environmentValues
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(arguments, " "), err)
	}
	return nil
}

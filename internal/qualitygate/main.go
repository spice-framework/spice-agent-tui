// Command qualitygate runs the repository-owned cross-platform verification.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	requiredGoVersion  = "go1.26.5"
	modulePath         = "github.com/spice-framework/spice-agent-tui"
	annotationTool     = modulePath + "/cmd/spice-agent-tui-annotations"
	coreModule         = "github.com/spice-framework/spice"
	coreVersion        = "v0.1.0-preview.1.0.20260806200749-524424a04df0"
	toolchainModule    = "github.com/spice-framework/toolchain"
	toolchainVersion   = "v0.1.0-preview.1.0.20260806203056-d0b9ac086bd6"
	spiceTool          = toolchainModule + "/cmd/spice"
	coreAnnotationTool = toolchainModule + "/cmd/spice-annotation-core"
	minimumCoverage    = 85.0
)

var output io.Writer = os.Stdout

func main() {
	os.Exit(execute()) // Entrypoint exception: propagate verification failure.
}

func execute() int {
	mode := flag.String("mode", "verify", "verification mode: tools-bootstrap, fast, check, fmt, or verify")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	root, err := repositoryRoot()
	if err == nil {
		err = run(ctx, root, *mode)
	}
	if err != nil {
		if _, writeErr := fmt.Fprintf(output, "quality gate failed: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}
	return 0
}

type step struct {
	name string
	run  func() error
}

func run(ctx context.Context, root, mode string) error {
	if runtime.Version() != requiredGoVersion {
		return fmt.Errorf("go version is %s; require exactly %s", runtime.Version(), requiredGoVersion)
	}
	identity := step{"repository identity", func() error {
		if err := checkIdentity(root); err != nil {
			return err
		}
		return checkReleaseMetadata(root)
	}}
	bootstrap := step{"explicit dependency bootstrap", func() error { return bootstrapDependencies(ctx, root, networkCommand) }}
	formatting := step{"formatting", func() error { return format(ctx, root, false) }}
	modules := step{"module and vendor", func() error { return checkModule(ctx, root) }}
	composition := step{"Spice composition", func() error { return checkSpiceComposition(ctx, root) }}
	vet := step{"go vet", func() error { return command(ctx, root, nil, "go", "vet", "./...") }}
	test := step{"shuffled tests", func() error { return tests(ctx, root, false) }}
	var steps []step
	if networkAllowed(mode) {
		steps = []step{identity, bootstrap}
	} else {
		switch mode {
		case "fast":
			steps = []step{identity, test}
		case "check":
			steps = []step{identity, formatting, modules, composition, vet, test}
		case "fmt":
			steps = []step{identity, {"formatting write", func() error { return format(ctx, root, true) }}}
		case "verify":
			steps = []step{
				identity, formatting, modules, composition, vet,
				{"lint and nil safety", func() error { return lint(ctx, root) }},
				{"security", func() error { return security(ctx, root) }},
				test,
				{"race tests", func() error { return tests(ctx, root, true) }},
				{"coverage", func() error { return coverage(ctx, root) }},
				{"offline vendor", func() error { return offline(ctx, root) }},
			}
		default:
			return fmt.Errorf("unknown mode %q", mode)
		}
	}
	for _, current := range steps {
		started := time.Now()
		if _, err := fmt.Fprintf(output, "==> %s\n", current.name); err != nil {
			return err
		}
		if err := current.run(); err != nil {
			return fmt.Errorf("%s (%s): %w", current.name, time.Since(started).Round(time.Millisecond), err)
		}
		if _, err := fmt.Fprintf(output, "<== %s passed in %s\n", current.name, time.Since(started).Round(time.Millisecond)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(output, "==> all verification passed")
	return err
}

func checkSpiceComposition(ctx context.Context, root string) error {
	environment := map[string]string{
		"GOFLAGS": "-mod=vendor", "GOPROXY": "off", "GOTOOLCHAIN": "local", "GOWORK": "off",
	}
	const fixture = "./internal/acceptance/composition"
	if err := command(ctx, root, environment, "go", "tool", spiceTool, "verify", fixture); err != nil {
		return err
	}
	return command(
		ctx, root, environment, "go", "tool", spiceTool,
		"generate", "--target", "compositionproof", "--check", fixture,
	)
}

func networkAllowed(mode string) bool { return mode == "tools-bootstrap" }

func checkIdentity(root string) error {
	content, err := os.ReadFile(filepath.Join(root, "go.mod")) // #nosec G304 -- root is repository-owned.
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	for _, required := range []string{
		"module " + modulePath + "\n",
		"\ngo 1.26.0\n",
		"\ntoolchain go1.26.5\n",
		"charm.land/bubbletea/v2 v2.0.8",
		"github.com/charmbracelet/x/ansi v0.11.7",
		coreModule + " " + coreVersion,
		toolchainModule + " " + toolchainVersion,
		"\t" + annotationTool + "\n",
		"\t" + spiceTool + "\n",
		"\t" + coreAnnotationTool + "\n",
	} {
		if strings.Count(normalized, required) != 1 {
			return fmt.Errorf("go.mod must contain exactly one %q", strings.TrimSpace(required))
		}
	}
	if strings.Contains(normalized, "github.com/charmbracelet/bubbletea/v2") ||
		strings.Contains(normalized, "\nreplace ") || strings.Contains(normalized, "\nreplace (") {
		return errors.New("go.mod must use canonical, unreplaced charm.land/bubbletea/v2 v2.0.8")
	}
	compatibilityContent, err := os.ReadFile(filepath.Join(root, "compatibility.json")) // #nosec G304 -- fixed file under repository root.
	if err != nil {
		return fmt.Errorf("read compatibility metadata: %w", err)
	}
	if err := validateCompatibility(compatibilityContent); err != nil {
		return err
	}
	return validateToolPins(filepath.Join(root, "tools", "go.mod"))
}

type compatibility struct {
	Schema             int     `json:"schema"`
	Go                 string  `json:"go"`
	SpiceAgentClient   *string `json:"spice_agent_client"`
	SpiceAgentUIValues *string `json:"spice_agent_ui_values"`
	SpiceCore          *string `json:"spice_core"`
	SpiceToolchain     *string `json:"spice_toolchain"`
}

func validateCompatibility(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var value compatibility
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode compatibility metadata: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("compatibility metadata has trailing JSON values")
	}
	if value.Schema != 1 || value.Go != "1.26.5" || value.SpiceAgentClient != nil ||
		value.SpiceAgentUIValues == nil || *value.SpiceAgentUIValues != "v0.1.0-dev" ||
		value.SpiceCore == nil || *value.SpiceCore != coreVersion ||
		value.SpiceToolchain == nil || *value.SpiceToolchain != toolchainVersion {
		return errors.New(
			"compatibility metadata must record Go 1.26.5, local UI values v0.1.0-dev, exact result-facts core/toolchain revisions, and an explicit null client contract",
		)
	}
	return nil
}

func validateToolPins(path string) error {
	content, err := os.ReadFile(path) // #nosec G304 -- fixed tools module under repository root.
	if err != nil {
		return fmt.Errorf("read tools module: %w", err)
	}
	for _, pin := range []string{
		"github.com/golangci/golangci-lint/v2 v2.12.2",
		"github.com/securego/gosec/v2 v2.28.0",
		"go.uber.org/nilaway v0.0.0-20260724203407-f4f8ac24c032",
		"golang.org/x/tools v0.48.0",
		"golang.org/x/vuln v1.1.4",
		"mvdan.cc/gofumpt v0.10.0",
	} {
		if !bytes.Contains(content, []byte(pin)) {
			return fmt.Errorf("tools module is missing exact pin %s", pin)
		}
	}
	return nil
}

type bootstrapRunner func(context.Context, string, ...string) error

type moduleGraph struct {
	directory string
	optional  bool
}

func bootstrapDependencies(ctx context.Context, root string, runner bootstrapRunner) (returnErr error) {
	before, err := sourceTreeDigests(root)
	if err != nil {
		return fmt.Errorf("snapshot repository before bootstrap: %w", err)
	}
	defer func() {
		after, snapshotErr := sourceTreeDigests(root)
		if snapshotErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("snapshot repository after bootstrap: %w", snapshotErr))
			return
		}
		if !maps.Equal(before, after) {
			returnErr = errors.Join(returnErr, errors.New("dependency bootstrap modified the repository"))
		}
	}()

	graphs := []moduleGraph{{directory: root}, {directory: filepath.Join(root, "tools"), optional: true}}
	for _, graph := range graphs {
		if err := bootstrapModuleGraph(ctx, graph, runner); err != nil {
			return err
		}
	}
	return nil
}

func bootstrapModuleGraph(ctx context.Context, graph moduleGraph, runner bootstrapRunner) (returnErr error) {
	moduleFile := filepath.Join(graph.directory, "go.mod")
	moduleContent, err := os.ReadFile(moduleFile) // #nosec G304 -- repository-owned module graph.
	if errors.Is(err, os.ErrNotExist) && graph.optional {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", moduleFile, err)
	}
	temporary, err := os.MkdirTemp("", "spice-tools-bootstrap-*")
	if err != nil {
		return fmt.Errorf("create dependency bootstrap directory: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, os.RemoveAll(temporary)) }()
	temporaryRoot, err := os.OpenRoot(temporary)
	if err != nil {
		return fmt.Errorf("open dependency bootstrap directory: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, temporaryRoot.Close()) }()

	temporaryModule := filepath.Join(temporary, "graph.mod")
	if writeErr := temporaryRoot.WriteFile("graph.mod", moduleContent, 0o600); writeErr != nil {
		return fmt.Errorf("write temporary module file: %w", writeErr)
	}
	sumFile := filepath.Join(graph.directory, "go.sum")
	sumContent, err := os.ReadFile(sumFile) // #nosec G304 -- repository-owned module graph.
	if err == nil {
		if writeErr := temporaryRoot.WriteFile("graph.sum", sumContent, 0o600); writeErr != nil {
			return fmt.Errorf("write temporary checksum file: %w", writeErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", sumFile, err)
	}
	return runner(ctx, graph.directory, bootstrapDownloadArguments(temporaryModule)...)
}

func bootstrapDownloadArguments(moduleFile string) []string {
	return []string{"mod", "download", "-modfile=" + moduleFile, "all"}
}

func format(ctx context.Context, root string, write bool) error {
	files, err := goFiles(root)
	if err != nil {
		return err
	}
	for _, name := range []string{"goimports", "gofumpt"} {
		executable, err := toolPath(ctx, root, name)
		if err != nil {
			return err
		}
		option := "-l"
		if write {
			option = "-w"
		}
		result, err := capture(ctx, root, executable, append([]string{option}, files...)...)
		if err != nil {
			return err
		}
		if !write && strings.TrimSpace(result) != "" {
			return fmt.Errorf("%s requires formatting: %s", name, strings.Join(strings.Fields(result), ", "))
		}
	}
	return nil
}

func goFiles(root string) ([]string, error) {
	result := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && slices.Contains([]string{".git", "tools", "vendor"}, entry.Name()) {
			return filepath.SkipDir
		}
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			result = append(result, path)
		}
		return nil
	})
	slices.Sort(result)
	return result, err
}

func checkModule(ctx context.Context, root string) error {
	if err := command(ctx, root, nil, "go", "mod", "tidy", "-diff"); err != nil {
		return err
	}
	if err := command(ctx, root, nil, "go", "-C", "tools", "mod", "tidy", "-diff"); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "spice-agent-tui-vendor-*")
	if err != nil {
		return fmt.Errorf("create vendor comparison: %w", err)
	}
	defer removeTree(temporary)
	candidate := filepath.Join(temporary, "vendor")
	if vendorErr := command(ctx, root, nil, "go", "mod", "vendor", "-o", candidate); vendorErr != nil {
		return vendorErr
	}
	current, err := treeDigests(filepath.Join(root, "vendor"))
	if err != nil {
		return err
	}
	expected, err := treeDigests(candidate)
	if err != nil {
		return err
	}
	if !maps.Equal(current, expected) {
		return errors.New("vendor differs from a fresh go mod vendor result")
	}
	return nil
}

func treeDigests(root string) (map[string][sha256.Size]byte, error) {
	result := make(map[string][sha256.Size]byte)
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	return digests(root, false)
}

func sourceTreeDigests(root string) (map[string][sha256.Size]byte, error) {
	return digests(root, true)
}

func digests(root string, excludeGit bool) (map[string][sha256.Size]byte, error) {
	result := make(map[string][sha256.Size]byte)
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer opened.Close() //nolint:errcheck // Read-only close cannot affect verification.
	err = fs.WalkDir(opened.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if excludeGit && path == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		content, readErr := opened.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		result[filepath.ToSlash(path)] = sha256.Sum256(content)
		return nil
	})
	return result, err
}

func lint(ctx context.Context, root string) error {
	golangci, err := toolPath(ctx, root, "golangci-lint")
	if err != nil {
		return err
	}
	if lintErr := command(ctx, root, nil, golangci, "run", "--timeout=10m"); lintErr != nil {
		return lintErr
	}
	nilaway, err := toolPath(ctx, root, "nilaway")
	if err != nil {
		return err
	}
	return command(ctx, root, nil, nilaway, "-include-pkgs="+modulePath, "./...")
}

func security(ctx context.Context, root string) error {
	gosec, err := toolPath(ctx, root, "gosec")
	if err != nil {
		return err
	}
	if securityErr := command(ctx, root, nil, gosec, "-quiet", "-exclude-generated", "./..."); securityErr != nil {
		return securityErr
	}
	govulncheck, err := toolPath(ctx, root, "govulncheck")
	if err != nil {
		return err
	}
	return command(ctx, root, nil, govulncheck, "./...")
}

func productPackages(ctx context.Context, root string) ([]string, error) {
	content, err := capture(ctx, root, "go", "list", "-f={{.ImportPath}}", "./...")
	if err != nil {
		return nil, err
	}
	gate := modulePath + "/internal/qualitygate"
	result := make([]string, 0)
	for candidate := range strings.FieldsSeq(content) {
		if candidate != gate {
			result = append(result, candidate)
		}
	}
	slices.Sort(result)
	if len(result) == 0 {
		return nil, errors.New("repository has no product packages")
	}
	return result, nil
}

func tests(ctx context.Context, root string, race bool) error {
	packages, err := productPackages(ctx, root)
	if err != nil {
		return err
	}
	arguments := []string{"test"}
	if race {
		arguments = append(arguments, "-race")
	}
	arguments = append(arguments, "-shuffle=on", "-count=1")
	arguments = append(arguments, packages...)
	if err := command(ctx, root, nil, "go", arguments...); err != nil {
		return err
	}
	arguments = append(arguments[:len(arguments)-len(packages)], "./internal/qualitygate")
	return command(ctx, root, nil, "go", arguments...)
}

func coverage(ctx context.Context, root string) (resultErr error) {
	packages, err := productPackages(ctx, root)
	if err != nil {
		return err
	}
	coveragePackages := handwrittenCoveragePackages(packages)
	if len(coveragePackages) == 0 {
		return errors.New("repository has no handwritten product packages")
	}
	profile, err := os.CreateTemp("", "spice-agent-tui-coverage-*.out")
	if err != nil {
		return err
	}
	path := profile.Name()
	if closeErr := profile.Close(); closeErr != nil {
		return closeErr
	}
	defer func() { resultErr = errors.Join(resultErr, os.Remove(path)) }()
	arguments := []string{"test", "-covermode=atomic", "-coverpkg=" + strings.Join(coveragePackages, ","), "-coverprofile=" + path}
	arguments = append(arguments, packages...)
	if coverageErr := command(ctx, root, nil, "go", arguments...); coverageErr != nil {
		return coverageErr
	}
	profileContent, err := os.ReadFile(path) // #nosec G304 -- path is created above.
	if err != nil {
		return err
	}
	if len(strings.Split(strings.TrimSpace(string(profileContent)), "\n")) == 1 {
		_, err = fmt.Fprintln(output, "product coverage: no executable statements")
		return err
	}
	report, err := capture(ctx, root, "go", "tool", "cover", "-func="+path)
	if err != nil {
		return err
	}
	percentage, err := totalCoverage(report)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "product coverage %.1f%% (minimum %.1f%%)\n", percentage, minimumCoverage); err != nil {
		return err
	}
	if percentage < minimumCoverage {
		return fmt.Errorf("product coverage %.1f%% is below %.1f%%", percentage, minimumCoverage)
	}
	return nil
}

func handwrittenCoveragePackages(packages []string) []string {
	generatedRoot := modulePath + "/internal/spicegen"
	result := make([]string, 0, len(packages))
	for _, candidate := range packages {
		if candidate != generatedRoot && !strings.HasPrefix(candidate, generatedRoot+"/") {
			result = append(result, candidate)
		}
	}
	return result
}

func totalCoverage(report string) (float64, error) {
	fields := strings.Fields(strings.TrimSpace(report))
	if len(fields) == 0 || !strings.HasSuffix(fields[len(fields)-1], "%") {
		return 0, errors.New("coverage report has no total percentage")
	}
	return strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
}

func offline(ctx context.Context, root string) error {
	packages, err := productPackages(ctx, root)
	if err != nil {
		return err
	}
	environment := map[string]string{"GOFLAGS": "-mod=vendor"}
	if err := command(ctx, root, environment, "go", append([]string{"test", "-count=1"}, packages...)...); err != nil {
		return err
	}
	if err := command(ctx, root, environment, "go", append([]string{"build", "-trimpath"}, packages...)...); err != nil {
		return err
	}
	return command(ctx, root, environment, "go", "tool", annotationTool, "--spice-stdio")
}

func toolPath(ctx context.Context, root, name string) (string, error) {
	content, err := capture(ctx, root, "go", "tool", "-C", "tools", "-n", name)
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(content)
	if path == "" {
		return "", fmt.Errorf("resolve tool %q: empty path", name)
	}
	return path, nil
}

func repositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		content, readErr := os.ReadFile(filepath.Join(current, "go.mod")) // #nosec G304 -- bounded ancestor search.
		if readErr == nil && bytes.Contains(content, []byte("module "+modulePath+"\n")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("find repository root: go.mod not found")
		}
		current = parent
	}
}

func command(ctx context.Context, directory string, overrides map[string]string, executable string, arguments ...string) error {
	executable = qualityExecutable(executable)
	// #nosec G204,G702 -- the gate constructs executable paths and passes discrete arguments without a shell.
	cmd := exec.CommandContext(ctx, executable, arguments...)
	cmd.Dir = directory
	cmd.Env = environment(false, overrides)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", executable, strings.Join(arguments, " "), err)
	}
	return nil
}

func networkCommand(ctx context.Context, directory string, arguments ...string) error {
	// #nosec G204,G702 -- only the exact copied module graphs are downloaded.
	cmd := exec.CommandContext(ctx, exactGoExecutable(), arguments...)
	cmd.Dir = directory
	cmd.Env = environment(true, nil)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(arguments, " "), err)
	}
	return nil
}

func capture(ctx context.Context, directory, executable string, arguments ...string) (string, error) {
	executable = qualityExecutable(executable)
	// #nosec G204,G702 -- the gate constructs executable paths and passes discrete arguments without a shell.
	cmd := exec.CommandContext(ctx, executable, arguments...)
	cmd.Dir = directory
	cmd.Env = environment(false, nil)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", executable, strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func qualityExecutable(executable string) string {
	if executable == "go" {
		return exactGoExecutable()
	}
	return executable
}

func exactGoExecutable() string {
	return filepath.Join(runtime.GOROOT(), "bin", goExecutableName(runtime.GOOS)) //nolint:staticcheck // Gate runs in place under the selected exact toolchain.
}

func goExecutableName(goos string) string {
	if goos == "windows" {
		return "go.exe"
	}
	return "go"
}

func environment(network bool, overrides map[string]string) []string {
	values := map[string]string{"GOFLAGS": "", "GOTOOLCHAIN": "local", "GOWORK": "off"}
	if network {
		values["GOAUTH"] = "off"
		values["GONOPROXY"] = ""
		values["GONOSUMDB"] = ""
		values["GOPRIVATE"] = ""
		values["GOPROXY"] = "https://proxy.golang.org"
		values["GOSUMDB"] = "sum.golang.org"
	} else {
		values["GOPROXY"] = "off"
	}
	maps.Copy(values, overrides)
	result := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if found {
			upperName := strings.ToUpper(name)
			if strings.Contains(upperName, "TOKEN") || strings.Contains(upperName, "SECRET") ||
				strings.Contains(upperName, "PASSWORD") || strings.HasSuffix(upperName, "_KEY") {
				continue
			}
			if _, replaced := values[upperName]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	slices.Sort(result)
	return result
}

func removeTree(path string) {
	if err := os.RemoveAll(path); err != nil {
		fmt.Fprintf(output, "warning: remove temporary tree %q: %v\n", path, err) //nolint:errcheck // Best-effort cleanup warning.
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReleasedVersionSkewContractIsFrozenAndOfflineValidatable(t *testing.T) {
	root := repositoryRootForTest(t)
	if err := checkReleasedVersionSkewContract(root); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := readReleasedVersionSkewManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, laneID := range []string{
		"previous-client-previous-peer",
		"previous-client-current-peer",
		"current-client-previous-peer",
		"current-client-current-peer",
	} {
		lane, clientGeneration, peerGeneration, selectErr := selectReleasedVersionSkewLane(manifest, laneID)
		if selectErr != nil || lane.ID != laneID || clientGeneration.Module != modulePath ||
			peerGeneration.Module != "github.com/spice-framework/spice-agent" {
			t.Fatalf("select lane %q = %#v, %#v, %#v, %v", laneID, lane, clientGeneration, peerGeneration, selectErr)
		}
		module := releasedVersionSkewGoMod(clientGeneration, peerGeneration)
		if strings.Contains(module, "replace ") || !strings.Contains(module, clientGeneration.Version) ||
			!strings.Contains(module, peerGeneration.Version) || !strings.Contains(module, grpcVersion) {
			t.Fatalf("generated module for %s = %q", laneID, module)
		}
	}
	if _, _, _, err = selectReleasedVersionSkewLane(manifest, "unknown"); err == nil {
		t.Fatal("unknown released version-skew lane was accepted")
	}
}

func TestReleasedVersionSkewManifestRejectsUnknownAndNoncanonicalValues(t *testing.T) {
	root := repositoryRootForTest(t)
	manifest, content, err := readReleasedVersionSkewManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Status = "claimed-without-hosted-proof"
	mutated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mutated = append(mutated, '\n')
	if err = validateReleasedVersionSkewManifest(root, manifest, mutated); err == nil {
		t.Fatal("invalid status was accepted")
	}
	manifest.Status = "hosted-linux-windows-gate"
	if err = validateReleasedVersionSkewManifest(root, manifest, append([]byte(" \n"), content...)); err == nil {
		t.Fatal("noncanonical manifest was accepted")
	}
}

func TestReleasedVersionSkewSourceDigestUsesNamesAndContent(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeFile(t, first, "one.go", "package proof\n")
	writeFile(t, second, "one.go", "package proof\n")
	left, err := releasedVersionSkewSourceDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := releasedVersionSkewSourceDigest(second)
	if err != nil || left != right {
		t.Fatalf("equal tree digests = %q, %q, %v", left, right, err)
	}
	writeFile(t, second, "two.go", "package proof\n")
	right, err = releasedVersionSkewSourceDigest(second)
	if err != nil || left == right {
		t.Fatalf("distinct tree digests = %q, %q, %v", left, right, err)
	}
}

func TestReleasedVersionSkewEnvironmentIsFreshPublicAndSecretSafe(t *testing.T) {
	t.Setenv("SPICE_PRIVATE_TOKEN", "secret-canary")
	temporary := t.TempDir()
	environmentValues := releasedVersionSkewEnvironment(temporary)
	want := map[string]string{
		"GOCACHE":    filepath.Join(temporary, "build-cache"),
		"GOMODCACHE": filepath.Join(temporary, "module-cache"),
		"GOPATH":     filepath.Join(temporary, "go-path"),
		"GOPROXY":    "https://proxy.golang.org",
		"GOSUMDB":    "sum.golang.org",
		"GOWORK":     "off",
	}
	for name, value := range want {
		if !slices.Contains(environmentValues, name+"="+value) {
			t.Fatalf("released environment lacks %s=%s", name, value)
		}
	}
	joined := strings.Join(environmentValues, "\n")
	if strings.Contains(joined, "secret-canary") || strings.Contains(joined, "SPICE_PRIVATE_TOKEN") {
		t.Fatalf("released environment contains secret: %s", joined)
	}
}

func TestCopyReleasedVersionSkewRunnerIsExact(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "nested", "runner")
	writeFile(t, source, "nested/proof.go", "package proof\n")
	if err := copyReleasedVersionSkewRunner(source, destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "nested", "proof.go"))
	if err != nil || string(content) != "package proof\n" {
		t.Fatalf("copied runner = %q, %v", content, err)
	}
}

func repositoryRootForTest(t *testing.T) string {
	t.Helper()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

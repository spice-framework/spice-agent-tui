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

func TestValidateCompatibility(t *testing.T) {
	t.Parallel()
	valid := `{"schema":1,"go":"1.26.5","spice_agent_client":null,"spice_agent_ui_values":"v0.1.0-dev","spice_core":null,"spice_toolchain":null}`
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
		{name: "premature client", content: strings.Replace(valid, `"spice_agent_client":null`, `"spice_agent_client":"v1"`, 1), wantErr: "external contracts"},
		{name: "wrong UI values", content: strings.Replace(valid, `"spice_agent_ui_values":"v0.1.0-dev"`, `"spice_agent_ui_values":"v1"`, 1), wantErr: "local UI values"},
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
	writeFile(t, root, "go.mod", "module "+modulePath+"\n\ngo 1.26.0\n\ntoolchain go1.26.5\n\nrequire (\n\tcharm.land/bubbletea/v2 v2.0.8\n\tgithub.com/charmbracelet/x/ansi v0.11.7\n)\n")
	writeFile(t, root, "compatibility.json", `{"schema":1,"go":"1.26.5","spice_agent_client":null,"spice_agent_ui_values":"v0.1.0-dev","spice_core":null,"spice_toolchain":null}`)
	writeFile(t, root, "tools/go.mod", strings.Join([]string{
		"github.com/golangci/golangci-lint/v2 v2.12.2",
		"github.com/securego/gosec/v2 v2.28.0",
		"go.uber.org/nilaway v0.0.0-20260724203407-f4f8ac24c032",
		"golang.org/x/tools v0.48.0",
		"golang.org/x/vuln v1.1.4",
		"mvdan.cc/gofumpt v0.10.0",
	}, "\n"))
	if err := checkIdentity(root); err != nil {
		t.Fatalf("checkIdentity() error = %v", err)
	}
	writeFile(t, root, "go.mod", "module "+modulePath+"\n\ngo 1.26.0\n\ntoolchain go1.26.5\n\nrequire (\n\tgithub.com/charmbracelet/bubbletea/v2 v2.0.8\n\tgithub.com/charmbracelet/x/ansi v0.11.7\n)\n")
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "charm.land/bubbletea") {
		t.Fatalf("checkIdentity(noncanonical Bubble Tea) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", "module "+modulePath+"\n\ngo 1.26.0\n\ntoolchain go1.26.5\n\nrequire (\n\tcharm.land/bubbletea/v2 v2.0.8\n\tgithub.com/charmbracelet/x/ansi v0.11.7\n)\n\nreplace charm.land/bubbletea/v2 => ../local\n")
	if identityErr := checkIdentity(root); identityErr == nil || !strings.Contains(identityErr.Error(), "unreplaced") {
		t.Fatalf("checkIdentity(replaced Bubble Tea) error = %v", identityErr)
	}
	writeFile(t, root, "go.mod", "module "+modulePath+"\n\ngo 1.26.0\n\ntoolchain go1.26.5\n\nrequire (\n\tcharm.land/bubbletea/v2 v2.0.8\n\tgithub.com/charmbracelet/x/ansi v0.11.7\n)\n")
	writeFile(t, root, "tools/go.mod", "module missing")
	if err := checkIdentity(root); err == nil || !strings.Contains(err.Error(), "missing exact pin") {
		t.Fatalf("checkIdentity() error = %v, want pin diagnostic", err)
	}
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

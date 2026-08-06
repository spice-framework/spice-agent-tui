package agenttui_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const spiceTool = "github.com/spice-framework/toolchain/cmd/spice"

func TestRealSpiceToolEnforcesTUIResultContracts(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "aliases", pattern: "./internal/acceptance/resultfacts/alias"},
		{
			name:    "defined wrapper",
			pattern: "./internal/acceptance/resultfacts/wrapper",
			want:    "must have exact canonical type github.com/spice-framework/spice-agent-tui.Shell",
		},
		{
			name:    "anonymous interface",
			pattern: "./internal/acceptance/resultfacts/anonymous",
			want:    "must have exact canonical type github.com/spice-framework/spice-agent-tui.Shell",
		},
		{
			name:    "concrete renderer",
			pattern: "./internal/acceptance/resultfacts/concrete",
			want:    "must have effective Go kind interface",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			command := exec.CommandContext(
				ctx,
				goExecutable(),
				"tool",
				spiceTool,
				"verify",
				".",
				test.pattern,
			)
			command.Env = append(
				os.Environ(),
				"GOPROXY=off",
				"GOTOOLCHAIN=local",
				"GOWORK=off",
			)
			output, err := command.CombinedOutput()
			if test.want == "" {
				if err != nil {
					t.Fatalf("Spice verify aliases: %v\n%s", err, output)
				}
				return
			}
			if err == nil {
				t.Fatalf("Spice verify unexpectedly succeeded:\n%s", output)
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatalf("Spice verify timed out:\n%s", output)
			}
			text := filepath.ToSlash(string(output))
			if !strings.Contains(text, test.want) ||
				!strings.Contains(
					text,
					filepath.ToSlash(strings.TrimPrefix(test.pattern, "./")),
				) {
				t.Fatalf("Spice verify output = %q, want source path and %q", text, test.want)
			}
		})
	}
}

func goExecutable() string {
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	return filepath.Join(
		runtime.GOROOT(), //nolint:staticcheck // Acceptance must use the selected exact Go toolchain.
		"bin",
		name,
	)
}

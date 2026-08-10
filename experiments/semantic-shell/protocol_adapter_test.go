package semanticshell

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spice-framework/spice-agent-tui/experiments/semantic-shell/internal/agentcompat"
	"github.com/spice-framework/spice-agent/daemon/endpoint"
	"github.com/spice-framework/spice-agent/daemon/localipc"
)

const agentCompatibilityChildEnvironment = "SPICE_AGENT_TUI_COMPATIBILITY_CHILD"

type agentCompatibilityConfig struct {
	Address       string `json:"address"`
	Authorization string `json:"authorization"`
	MaximumMinor  uint32 `json:"maximum_minor"`
}

type agentCompatibilityStatus struct {
	State string `json:"state"`
	agentcompat.PeerStats
}

func TestSemanticShellAgentProtocolCompatibility(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("the Agent source-built compatibility contract is gated on Linux and Windows")
	}
	for _, test := range []struct {
		name       string
		minor      uint32
		hasAttempt bool
	}{
		{name: "previous semantics 1.2", minor: 2, hasAttempt: false},
		{name: "current semantics 1.3", minor: 3, hasAttempt: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer := startAgentCompatibilityChild(t, test.minor)
			adapter, facts, err := agentcompat.Dial(t.Context(), peer.address, peer.token, test.minor)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = adapter.Close() })
			if facts.ProtocolMinor != test.minor || facts.AttemptID != test.hasAttempt {
				t.Fatalf("adapter facts = %#v", facts)
			}

			harness := startShell(t, adapter)
			initial := harness.nextRecord(t)
			if initial.Type != "view" || initial.Revision != 1 || initial.View == nil ||
				!strings.Contains(initial.View.Status.Message, fmt.Sprintf("protocol 1.%d", test.minor)) {
				t.Fatalf("initial compatibility view = %#v", initial)
			}
			for _, command := range []string{"submit hello", "respond approved", "cancel"} {
				harness.write(t, command+"\n")
				result := nextCompatibilityResult(t, harness, strings.Fields(command)[0])
				if result.Type != "result" || result.Message == "" {
					t.Fatalf("%s result = %#v", command, result)
				}
			}
			harness.write(t, "quit\n")
			quit := nextCompatibilityResult(t, harness, "quit")
			if quit.Message != "semantic shell stopped" {
				t.Fatalf("quit result = %#v", quit)
			}
			if err = harness.wait(t); err != nil {
				t.Fatal(err)
			}
			if err = adapter.Close(); err != nil {
				t.Fatal(err)
			}
			stats := peer.stop(t)
			if stats.MaximumMinor != test.minor || stats.AttemptPresent != test.hasAttempt ||
				stats.Initialize != 1 || stats.Start != 1 || stats.Respond != 1 || stats.Cancel != 1 {
				t.Fatalf("compatibility peer stats = %#v", stats)
			}
		})
	}
}

func nextCompatibilityResult(t *testing.T, harness *shellHarness, command string) wireRecord {
	t.Helper()
	for range 8 {
		record := harness.nextRecord(t)
		if record.Type == "error" {
			t.Fatalf("compatibility command %s failed: %#v", command, record)
		}
		if record.Type == "result" && record.Command == command {
			return record
		}
	}
	t.Fatalf("compatibility command %s produced no result", command)
	return wireRecord{}
}

func TestAgentProtocolCompatibilityChild(t *testing.T) {
	if os.Getenv(agentCompatibilityChildEnvironment) != "1" {
		t.Skip("compatibility child entrypoint")
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var config agentCompatibilityConfig
	if err = json.Unmarshal(bytes.TrimSpace(line), &config); err != nil {
		t.Fatal(err)
	}
	token, err := endpoint.ParseAuthorizationValue(config.Authorization)
	if err != nil {
		t.Fatal("compatibility child authorization is invalid")
	}
	listener, err := localipc.Listen(config.Address)
	if err != nil {
		t.Fatal(err)
	}
	stop, stats, err := agentcompat.ServePeer(listener, token, config.MaximumMinor)
	if err != nil {
		t.Fatal(err)
	}
	writeAgentCompatibilityStatus(t, agentCompatibilityStatus{
		State: "ready", PeerStats: agentcompat.PeerStats{MaximumMinor: config.MaximumMinor},
	})
	command, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(command) != "STOP" {
		t.Fatalf("compatibility child stop command = %q, %v", command, err)
	}
	stop()
	writeAgentCompatibilityStatus(t, agentCompatibilityStatus{State: "stopped", PeerStats: stats()})
}

func writeAgentCompatibilityStatus(t *testing.T, status agentCompatibilityStatus) {
	t.Helper()
	content, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fmt.Fprintln(os.Stdout, string(content)); err != nil {
		t.Fatal(err)
	}
}

type agentCompatibilityChild struct {
	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  *bytes.Buffer
	address string
	token   endpoint.Token
	stopped bool
}

func startAgentCompatibilityChild(t *testing.T, maximumMinor uint32) *agentCompatibilityChild {
	t.Helper()
	token, err := endpoint.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := token.AuthorizationValue()
	if err != nil {
		t.Fatal(err)
	}
	address := agentCompatibilityAddress(t)
	// #nosec G204 -- this launches the exact source-built conformance test binary.
	command := exec.Command(os.Args[0], "-test.run=^TestAgentProtocolCompatibilityChild$", "-test.count=1")
	command.Env = append(os.Environ(), agentCompatibilityChildEnvironment+"=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	command.Stderr = stderr
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	child := &agentCompatibilityChild{
		process: command, stdin: stdin, stdout: bufio.NewReader(stdout), stderr: stderr,
		address: address, token: token,
	}
	t.Cleanup(func() {
		if !child.stopped {
			_ = child.process.Process.Kill()
			_ = child.process.Wait()
		}
	})
	content, err := json.Marshal(agentCompatibilityConfig{
		Address: address, Authorization: authorization, MaximumMinor: maximumMinor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = child.stdin.Write(append(content, '\n')); err != nil {
		t.Fatal(err)
	}
	ready := child.readStatus(t)
	if ready.State != "ready" || ready.MaximumMinor != maximumMinor {
		t.Fatalf("compatibility child ready = %#v", ready)
	}
	return child
}

func (child *agentCompatibilityChild) stop(t *testing.T) agentcompat.PeerStats {
	t.Helper()
	if child.stopped {
		t.Fatal("compatibility child stopped twice")
	}
	child.stopped = true
	if _, err := io.WriteString(child.stdin, "STOP\n"); err != nil {
		t.Fatal(err)
	}
	status := child.readStatus(t)
	if status.State != "stopped" {
		t.Fatalf("compatibility child stopped = %#v", status)
	}
	if err := child.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.process.Wait(); err != nil {
		t.Fatalf("compatibility child exit: %v; stderr: %s", err, child.stderr.String())
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(child.address); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("compatibility socket remains after shutdown: %v", err)
		}
	}
	return status.PeerStats
}

func (child *agentCompatibilityChild) readStatus(t *testing.T) agentCompatibilityStatus {
	t.Helper()
	type outcome struct {
		line []byte
		err  error
	}
	result := make(chan outcome, 1)
	go func() {
		line, err := child.stdout.ReadBytes('\n')
		result <- outcome{line: line, err: err}
	}()
	select {
	case observed := <-result:
		if observed.err != nil {
			t.Fatalf("read compatibility child status: %v; stderr: %s", observed.err, child.stderr.String())
		}
		var status agentCompatibilityStatus
		if err := json.Unmarshal(bytes.TrimSpace(observed.line), &status); err != nil {
			t.Fatalf("decode compatibility child status %q: %v", observed.line, err)
		}
		return status
	case <-time.After(10 * time.Second):
		t.Fatalf("compatibility child timed out; stderr: %s", child.stderr.String())
	}
	return agentCompatibilityStatus{}
}

func agentCompatibilityAddress(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\spice-agent-tui-compat-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, "engine.sock")
}

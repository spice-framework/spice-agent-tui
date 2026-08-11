package nativeacceptance

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Kodecable/crosspty"
	"github.com/charmbracelet/x/term"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

const (
	nativeTerminalHelperEnvironment = "SPICE_TUI_NATIVE_TERMINAL_HELPER"
	nativeTerminalHelperValue       = "native-terminal-v1"
)

func TestNativeTerminalRendersResizesAndExitsCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip("native PTY acceptance is excluded from short feedback")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := tuittest.NewVirtualTerminal(tuittest.VirtualTerminalOptions{
		Width:              80,
		Height:             24,
		MaxTranscriptBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	process, err := crosspty.Start(crosspty.CommandConfig{
		Argv: []string{executable, "-test.run=^TestNativeTerminalHelper$"},
		Env:  nativeTerminalEnvironment(),
		Size: crosspty.TermSize{Rows: 24, Cols: 80},
		CloseConfig: crosspty.CloseConfig{
			CloseTimeout: 5 * time.Second,
			KillDelay:    2 * time.Second,
			KillMode:     crosspty.KillModeKillGroupOnSubProcessExit,
		},
	})
	if err != nil {
		closeVirtualTerminal(t, terminal)
		t.Fatal(err)
	}
	readerDone := copyTerminalOutput(terminal, process)
	t.Cleanup(func() {
		if closeErr := process.Close(); closeErr != nil {
			t.Errorf("close native terminal process: %v", closeErr)
		}
		closeVirtualTerminal(t, terminal)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ready, err := terminal.WaitFor(ctx, "native-ready", nativeTerminalReady)
	if err != nil {
		t.Fatalf("wait for native terminal readiness: %v\n%s", err, terminalReport(terminal))
	}
	if !ready.AlternateScreen() {
		t.Fatalf("native terminal did not enter alternate screen:\n%s", ready.AgentReport())
	}
	if _, _, visible := ready.Cursor(); visible {
		t.Fatalf("native terminal cursor remained visible:\n%s", ready.AgentReport())
	}
	if resizeErr := process.Resize(crosspty.TermSize{Rows: 30, Cols: 100}); resizeErr != nil {
		t.Fatal(resizeErr)
	}
	if resizeErr := terminal.Resize(100, 30); resizeErr != nil {
		t.Fatal(resizeErr)
	}
	writeTerminalLine(t, process, "verify")
	verified, err := terminal.WaitFor(ctx, "native-verified", func(screen tuittest.Screen) bool {
		return screen.Contains("received=verify") && screen.Contains("size=100x30")
	})
	if err != nil {
		t.Fatalf("wait for native terminal verification: %v\n%s", err, terminalReport(terminal))
	}
	if verified.Width() != 100 || verified.Height() != 30 {
		t.Fatalf("verified screen = %dx%d, want 100x30", verified.Width(), verified.Height())
	}
	writeTerminalLine(t, process, "quit")
	exit := make(chan int, 1)
	go func() { exit <- process.Wait() }()
	select {
	case code := <-exit:
		if code != 0 {
			t.Fatalf("native terminal helper exit code = %d\n%s", code, terminalReport(terminal))
		}
	case <-ctx.Done():
		t.Fatal(context.Cause(ctx))
	}
	if closeErr := process.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	select {
	case readErr := <-readerDone:
		if readErr != nil {
			t.Fatal(readErr)
		}
	case <-ctx.Done():
		t.Fatal(context.Cause(ctx))
	}
}

func TestNativeTerminalReadinessWaitsForCompleteControlSequence(t *testing.T) {
	terminal, err := tuittest.NewVirtualTerminal(tuittest.VirtualTerminalOptions{
		Width:              80,
		Height:             24,
		MaxTranscriptBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeVirtualTerminal(t, terminal) })

	if _, writeErr := terminal.WriteString("\x1b[?1049h\x1b[2J\x1b[HSpice native terminal ready\r\n"); writeErr != nil {
		t.Fatal(writeErr)
	}
	partial, err := terminal.Screen("partial-native-ready")
	if err != nil {
		t.Fatal(err)
	}
	if nativeTerminalReady(partial) {
		t.Fatal("readiness accepted visible cursor before the complete control sequence")
	}

	if _, writeErr := terminal.WriteString("\x1b[?25l"); writeErr != nil {
		t.Fatal(writeErr)
	}
	complete, err := terminal.Screen("complete-native-ready")
	if err != nil {
		t.Fatal(err)
	}
	if !nativeTerminalReady(complete) {
		t.Fatalf("readiness rejected complete terminal state:\n%s", complete.AgentReport())
	}
}

func nativeTerminalReady(screen tuittest.Screen) bool {
	if !screen.Contains("Spice native terminal ready") || !screen.AlternateScreen() {
		return false
	}
	_, _, visible := screen.Cursor()
	return !visible
}

func TestNativeTerminalEnvironmentExcludesSecrets(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "native-terminal-secret-canary")
	environment := nativeTerminalEnvironment()
	joined := strings.Join(environment, "\n")
	if strings.Contains(joined, "native-terminal-secret-canary") ||
		strings.Contains(strings.ToUpper(joined), "OPENROUTER_API_KEY") {
		t.Fatalf("native terminal environment includes secret canary: %q", environment)
	}
	if !strings.Contains(joined, nativeTerminalHelperEnvironment+"="+nativeTerminalHelperValue) ||
		!strings.Contains(joined, "TERM=xterm-256color") {
		t.Fatalf("native terminal environment lacks fixed helper values: %q", environment)
	}
}

func TestNativeTerminalHelper(t *testing.T) {
	if os.Getenv(nativeTerminalHelperEnvironment) != nativeTerminalHelperValue {
		t.Skip("native terminal helper process only")
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "native terminal helper does not own a TTY")
		os.Exit(20)
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\x1b[?1049h\x1b[2J\x1b[H\x1b[38;2;56;189;248mSpice native terminal ready\x1b[0m\r\n\x1b[?25l")
	command, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "read verification command: %v\n", err)
		os.Exit(21)
	}
	width, height, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		fmt.Fprintf(os.Stderr, "read terminal dimensions: %v\n", err)
		os.Exit(22)
	}
	fmt.Printf("\r\nreceived=%s size=%dx%d\r\n", strings.TrimSpace(command), width, height)
	quit, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(quit) != "quit" {
		fmt.Fprintf(os.Stderr, "read quit command: %v\n", err)
		os.Exit(23)
	}
	fmt.Print("\x1b[?25h\x1b[?1049l")
}

func copyTerminalOutput(terminal *tuittest.VirtualTerminal, process crosspty.Pty) <-chan error {
	done := make(chan error, 1)
	go func() {
		buffer := make([]byte, 4096)
		for {
			count, readErr := process.Read(buffer)
			if count > 0 {
				if _, writeErr := terminal.Write(buffer[:count]); writeErr != nil {
					done <- fmt.Errorf("capture native terminal output: %w", writeErr)
					return
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) || errors.Is(readErr, os.ErrClosed) {
					done <- nil
				} else {
					done <- fmt.Errorf("read native terminal output: %w", readErr)
				}
				return
			}
		}
	}()
	return done
}

func writeTerminalLine(t *testing.T, process crosspty.Pty, value string) {
	t.Helper()
	lineEnding := "\n"
	if runtime.GOOS == "windows" {
		// ConPTY expects the carriage-return key sequence to submit a line.
		lineEnding = "\r\n"
	}
	// A Unix canonical PTY accepts one line-feed. Supplying CRLF there can
	// become two logical line endings and leave an empty command queued.
	line := value + lineEnding
	written, err := io.WriteString(process, line)
	if err != nil {
		t.Fatal(err)
	}
	if written != len(line) {
		t.Fatalf("native terminal write = %d bytes, want %d", written, len(line))
	}
}

func terminalReport(terminal *tuittest.VirtualTerminal) string {
	screen, err := terminal.Screen("native-failure")
	if err != nil {
		return err.Error()
	}
	return screen.AgentReport()
}

func closeVirtualTerminal(t *testing.T, terminal *tuittest.VirtualTerminal) {
	t.Helper()
	if err := terminal.Close(); err != nil {
		t.Errorf("close virtual terminal: %v", err)
	}
}

func nativeTerminalEnvironment() []string {
	environment := []string{
		nativeTerminalHelperEnvironment + "=" + nativeTerminalHelperValue,
		"TERM=xterm-256color",
	}
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		switch strings.ToUpper(name) {
		case "HOME", "LANG", "LC_ALL", "SYSTEMROOT", "TEMP", "TMP", "TMPDIR", "USERPROFILE", "WINDIR":
			environment = append(environment, entry)
		}
	}
	return environment
}

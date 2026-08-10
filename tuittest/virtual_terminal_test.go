package tuittest

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestVirtualTerminalInterpretsAlternateScreenUnicodeAndCursor(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{Width: 12, Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeVirtualTerminal(t, terminal) })
	chunks := []string{"\x1b[?1049", "h\x1b[2J\x1b[H", "界", "x", "\x1b[?25l"}
	for _, chunk := range chunks {
		if _, writeErr := terminal.WriteString(chunk); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	screen, err := terminal.Screen("unicode-alt")
	if err != nil {
		t.Fatal(err)
	}
	if !screen.AlternateScreen() || screen.cursorVisible || !screen.Contains("界x") {
		t.Fatalf("terminal capture:\n%s", screen.AgentReport())
	}
	if x, y, visible := screen.Cursor(); x != 3 || y != 0 || visible {
		t.Fatalf("cursor = %d,%d,%t, want 3,0,false", x, y, visible)
	}
	mainScreen := screen
	mainScreen.altScreen = false
	if screen.EqualStyled(mainScreen) || !strings.Contains(screen.Diff(mainScreen), "alternateScreen") {
		t.Fatal("alternate-screen state did not participate in exact screen comparison")
	}
	for index, line := range screen.Lines() {
		if ansi.StringWidth(line) != 12 {
			t.Fatalf("line %d width = %d, want 12: %q", index, ansi.StringWidth(line), line)
		}
	}
	if got, want := string(terminal.Transcript()), strings.Join(chunks, ""); got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}
}

func TestVirtualTerminalResizeWaitAndCloseAreDeterministic(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{Width: 8, Height: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	result := make(chan Screen, 1)
	failure := make(chan error, 1)
	go func() {
		screen, waitErr := terminal.WaitFor(ctx, "ready", func(screen Screen) bool {
			return screen.Width() == 10 && screen.Contains("ready")
		})
		if waitErr != nil {
			failure <- waitErr
			return
		}
		result <- screen
	}()
	if err := terminal.Resize(10, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.WriteString("ready"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-failure:
		t.Fatal(err)
	case screen := <-result:
		if screen.Height() != 3 || screen.Name() != "ready" {
			t.Fatalf("screen = %#v", screen)
		}
	case <-ctx.Done():
		t.Fatal(context.Cause(ctx))
	}
	if err := terminal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := terminal.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.WriteString("late"); err == nil {
		t.Fatal("closed terminal accepted output")
	}
	if _, err := terminal.Screen("final"); err != nil {
		t.Fatal(err)
	}
}

func TestVirtualTerminalBoundsOverflowAndPredicateFailures(t *testing.T) {
	t.Parallel()
	for _, options := range []VirtualTerminalOptions{
		{Width: -1},
		{Width: maximumVirtualTerminalWidth + 1},
		{Height: maximumVirtualTerminalHeight + 1},
		{MaxTranscriptBytes: maximumVirtualTerminalTranscript + 1},
	} {
		if _, err := NewVirtualTerminal(options); err == nil {
			t.Fatalf("NewVirtualTerminal(%+v) error = nil", options)
		}
	}
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{
		Width:              4,
		Height:             2,
		MaxTranscriptBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeVirtualTerminal(t, terminal) })
	if _, err := terminal.WriteString("good"); err != nil {
		t.Fatal(err)
	}
	before := terminal.Transcript()
	if written, err := terminal.WriteString("!"); written != 0 || err == nil {
		t.Fatalf("overflow = %d, %v", written, err)
	}
	if after := terminal.Transcript(); !bytes.Equal(before, after) {
		t.Fatalf("overflow mutated transcript: %q -> %q", before, after)
	}
	var nilContext context.Context
	if _, waitErr := terminal.WaitFor(nilContext, "nil-context", func(Screen) bool { return true }); waitErr == nil {
		t.Fatal("nil wait context accepted")
	}
	if _, err := terminal.WaitFor(t.Context(), "nil-predicate", nil); err == nil {
		t.Fatal("nil predicate accepted")
	}
	if _, err := terminal.WaitFor(t.Context(), "panic", func(Screen) bool {
		panic("secret predicate detail")
	}); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("panic error = %v", err)
	}
}

func TestVirtualTerminalResizeLimitIsExact(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{Width: 2, Height: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeVirtualTerminal(t, terminal) })
	for index := range maximumVirtualTerminalResizes {
		if resizeErr := terminal.Resize(2+(index%2), 2); resizeErr != nil {
			t.Fatalf("resize %d: %v", index, resizeErr)
		}
	}
	if resizeErr := terminal.Resize(2, 2); resizeErr == nil ||
		!strings.Contains(resizeErr.Error(), "resize limit") {
		t.Fatalf("resize past limit error = %v", resizeErr)
	}
}

func TestVirtualTerminalConcurrentWritesAndCaptures(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{
		Width:              80,
		Height:             24,
		MaxTranscriptBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeVirtualTerminal(t, terminal) })
	var group sync.WaitGroup
	for index := range 32 {
		group.Go(func() {
			if _, writeErr := terminal.WriteString("event\r\n"); writeErr != nil {
				t.Errorf("write %d: %v", index, writeErr)
			}
			if _, captureErr := terminal.Screen("concurrent"); captureErr != nil {
				t.Errorf("capture %d: %v", index, captureErr)
			}
		})
	}
	group.Wait()
	if got := bytes.Count(terminal.Transcript(), []byte("event")); got != 32 {
		t.Fatalf("events = %d, want 32", got)
	}
}

func TestVirtualTerminalClosedWaitFailsWithoutPolling(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := terminal.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	_, err = terminal.WaitFor(t.Context(), "never", func(Screen) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "closed") || errors.Is(err, context.Canceled) {
		t.Fatalf("closed wait error = %v", err)
	}
}

func TestVirtualTerminalCloseWakesActiveWaiter(t *testing.T) {
	t.Parallel()
	terminal, err := NewVirtualTerminal(VirtualTerminalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	started := make(chan struct{})
	result := make(chan error, 1)
	var startOnce sync.Once
	go func() {
		_, waitErr := terminal.WaitFor(ctx, "close-wake", func(Screen) bool {
			startOnce.Do(func() { close(started) })
			return false
		})
		result <- waitErr
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(context.Cause(ctx))
	}
	if closeErr := terminal.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	select {
	case waitErr := <-result:
		if waitErr == nil || !strings.Contains(waitErr.Error(), "closed") {
			t.Fatalf("wait error = %v", waitErr)
		}
	case <-ctx.Done():
		t.Fatal(context.Cause(ctx))
	}
}

func closeVirtualTerminal(t *testing.T, terminal *VirtualTerminal) {
	t.Helper()
	if err := terminal.Close(); err != nil {
		t.Errorf("close virtual terminal: %v", err)
	}
}

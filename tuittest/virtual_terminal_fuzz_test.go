package tuittest

import (
	"testing"
	"unicode/utf8"
)

func FuzzVirtualTerminalChunking(f *testing.F) {
	f.Add("Spice 界👩‍💻 terminal")
	f.Add("e\u0301 👨‍👩‍👧‍👦 🏳️‍🌈 🇺🇳 表")
	f.Add("\x1b[31mred\x1b[0m\x1b[2;4H界\x1b[?25l")
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 2048 || !utf8.ValidString(value) {
			t.Skip()
		}
		whole, err := NewVirtualTerminal(VirtualTerminalOptions{Width: 80, Height: 24})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeVirtualTerminal(t, whole) })
		chunked, err := NewVirtualTerminal(VirtualTerminalOptions{Width: 80, Height: 24})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeVirtualTerminal(t, chunked) })
		payload := "\x1b[2J\x1b[H" + value
		if _, writeErr := whole.WriteString(payload); writeErr != nil {
			t.Fatal(writeErr)
		}
		for _, character := range []byte(payload) {
			if _, writeErr := chunked.Write([]byte{character}); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
		wholeScreen, err := whole.Screen("chunking")
		if err != nil {
			t.Fatal(err)
		}
		chunkedScreen, err := chunked.Screen("chunking")
		if err != nil {
			t.Fatal(err)
		}
		if wholeScreen.Digest() != chunkedScreen.Digest() {
			t.Fatalf("chunked terminal output differs\n%s", wholeScreen.Diff(chunkedScreen))
		}
	})
}

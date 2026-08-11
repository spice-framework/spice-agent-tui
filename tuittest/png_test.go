package tuittest_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

func TestRenderPNGIsDeterministicAndDimensionallyExact(t *testing.T) {
	t.Parallel()
	replay, err := lifecycleTrace(t, false).Replay()
	if err != nil {
		t.Fatal(err)
	}
	screen, exists := replay.Screen("lifecycle-history")
	if !exists {
		t.Fatal("missing lifecycle-history screen")
	}
	first, err := tuittest.RenderPNG(screen, tuittest.PNGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := tuittest.RenderPNG(screen, tuittest.PNGOptions{
		ThemeMode: agenttui.ThemeDark, Scale: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical Screen renders produced different PNG bytes")
	}
	digest := sha256.Sum256(first)
	if got := hex.EncodeToString(digest[:]); got != "6f0a8b7efab8c1db3c06912ddd1446ad3c9b797d952c69d4077e329c28a0aadf" {
		t.Fatalf("PNG digest = %s", got)
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Width != 484 || configuration.Height != 250 {
		t.Fatalf("PNG size = %dx%d, want 484x250", configuration.Width, configuration.Height)
	}
	if !bytes.HasPrefix(first, []byte("\x89PNG\r\n\x1a\n")) ||
		bytes.Contains(first, []byte("tIME")) || bytes.Contains(first, []byte("tEXt")) {
		t.Fatal("PNG signature or metadata contract failed")
	}

	light, err := tuittest.RenderPNG(screen, tuittest.PNGOptions{
		ThemeMode: agenttui.ThemeLight, Scale: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	configuration, err = png.DecodeConfig(bytes.NewReader(light))
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Width != 968 || configuration.Height != 500 || bytes.Equal(first, light) {
		t.Fatalf("scaled light PNG size/equality = %dx%d/%t", configuration.Width, configuration.Height, bytes.Equal(first, light))
	}
	if tuittest.EmbeddedFont != "Go Mono from golang.org/x/image v0.39.0" {
		t.Fatalf("embedded font identity = %q", tuittest.EmbeddedFont)
	}
}

func TestRenderPNGIsConcurrentAndUnicodeDeterministic(t *testing.T) {
	t.Parallel()
	const corpus = "👩‍💻 e\u0301 诊所界 שלום مرحبا"
	view := accessibilityView(t, agenttui.StatusWarning, "unicode warning", corpus, corpus)
	driver, err := tuittest.NewDriver(tuittest.Options{Width: 60, Height: 14, Initial: &view})
	if err != nil {
		t.Fatal(err)
	}
	screen, err := driver.Screen("unicode-png")
	driver.Close()
	if err != nil {
		t.Fatal(err)
	}
	want, err := tuittest.RenderPNG(screen, tuittest.PNGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 12
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			got, renderErr := tuittest.RenderPNG(screen, tuittest.PNGOptions{})
			if renderErr != nil {
				errors <- renderErr
				return
			}
			if !bytes.Equal(got, want) {
				errors <- fmt.Errorf("concurrent PNG bytes differ")
			}
		})
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestWriteDeterministicVisualArtifacts(t *testing.T) {
	t.Parallel()
	type artifact struct {
		Name         string `json:"name"`
		ScreenDigest string `json:"screen_digest"`
		PNGSHA256    string `json:"png_sha256"`
	}
	type manifest struct {
		Schema        int        `json:"schema"`
		Authoritative bool       `json:"authoritative"`
		Font          string     `json:"font"`
		Artifacts     []artifact `json:"artifacts"`
	}

	output := manifest{Schema: 1, Authoritative: false, Font: tuittest.EmbeddedFont}
	files := make(map[string][]byte)
	for _, accessible := range []bool{false, true} {
		replay, err := lifecycleTrace(t, accessible).Replay()
		if err != nil {
			t.Fatal(err)
		}
		for _, checkpoint := range []string{"lifecycle-ready", "lifecycle-streaming", "lifecycle-history"} {
			screen, exists := replay.Screen(checkpoint)
			if !exists {
				t.Fatalf("missing %q screen", checkpoint)
			}
			mode := agenttui.ThemeDark
			name := checkpoint + "-dark.png"
			if checkpoint == "lifecycle-ready" {
				mode = agenttui.ThemeLight
				name = checkpoint + "-light.png"
			}
			if accessible {
				name = "accessible-" + name
			}
			content, renderErr := tuittest.RenderPNG(screen, tuittest.PNGOptions{ThemeMode: mode, Scale: 1})
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			digest := sha256.Sum256(content)
			files[name] = content
			output.Artifacts = append(output.Artifacts, artifact{
				Name: name, ScreenDigest: screen.Digest(), PNGSHA256: hex.EncodeToString(digest[:]),
			})
		}
	}
	manifestJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON = append(manifestJSON, '\n')
	files["manifest.json"] = manifestJSON

	directory := os.Getenv("SPICE_TUI_VISUAL_ARTIFACT_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o750); err != nil { // #nosec G301 -- CI-only synthetic artifact directory.
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, content, 0o600); err != nil { // #nosec G304 -- directory is an explicit test artifact boundary.
			t.Fatal(err)
		}
	}
}

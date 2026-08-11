package tuittest

import "testing"

func TestScreenDigestCoversEveryObservableField(t *testing.T) {
	t.Parallel()
	base := Screen{
		name: "base", width: 2, height: 1, styled: "ab", plain: "ab", plainLines: []string{"ab"},
		cursorX: 1, cursorY: 0, cursorVisible: true, altScreen: true, accessible: false,
		prompt: "p", status: "ready", statusLevel: "ready", activity: []string{"a"}, revision: 1,
	}
	mutations := map[string]func(*Screen){
		"name":           func(screen *Screen) { screen.name = "other" },
		"width":          func(screen *Screen) { screen.width++ },
		"height":         func(screen *Screen) { screen.height++ },
		"styled":         func(screen *Screen) { screen.styled += "x" },
		"plain":          func(screen *Screen) { screen.plain += "x" },
		"plain lines":    func(screen *Screen) { screen.plainLines[0] += "x" },
		"cursor x":       func(screen *Screen) { screen.cursorX++ },
		"cursor y":       func(screen *Screen) { screen.cursorY++ },
		"cursor visible": func(screen *Screen) { screen.cursorVisible = false },
		"alternate":      func(screen *Screen) { screen.altScreen = false },
		"accessible":     func(screen *Screen) { screen.accessible = true },
		"prompt":         func(screen *Screen) { screen.prompt += "x" },
		"status":         func(screen *Screen) { screen.status += "x" },
		"status level":   func(screen *Screen) { screen.statusLevel = "busy" },
		"activity":       func(screen *Screen) { screen.activity[0] += "x" },
		"revision":       func(screen *Screen) { screen.revision++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			candidate := base
			candidate.plainLines = append([]string(nil), base.plainLines...)
			candidate.activity = append([]string(nil), base.activity...)
			mutate(&candidate)
			if candidate.Digest() == base.Digest() {
				t.Fatal("digest did not change")
			}
		})
	}
}

func TestScreenDigestNormalizesNilAndEmptySlices(t *testing.T) {
	t.Parallel()
	first := Screen{}
	second := Screen{plainLines: []string{}, activity: []string{}}
	if first.Digest() != second.Digest() {
		t.Fatal("nil and empty observable slices should have the same digest")
	}
}

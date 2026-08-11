package tuittest_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/tuittest"
)

func TestShippedStatusSemanticsNeverDependOnColor(t *testing.T) {
	t.Parallel()
	levels := []agenttui.StatusLevel{
		agenttui.StatusReady, agenttui.StatusBusy, agenttui.StatusDisconnected,
		agenttui.StatusReconnecting, agenttui.StatusWarning, agenttui.StatusError,
	}
	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			t.Parallel()
			for _, accessible := range []bool{false, true} {
				view := accessibilityView(t, level, "semantic status", "plain body", "")
				driver, err := tuittest.NewDriver(tuittest.Options{
					Width: 48, Height: 12, Accessible: accessible, Initial: &view,
				})
				if err != nil {
					t.Fatal(err)
				}
				screen, err := driver.Screen("status-" + string(level))
				driver.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := screen.ValidateStatusSemantics(); err != nil {
					t.Fatal(err)
				}
				if accessible {
					if err := screen.ValidateAccessibility(); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestAccessibleRendererPreservesComplexUnicodeWithoutControls(t *testing.T) {
	t.Parallel()
	const corpus = "ZWJ 👩‍💻 · combining e\u0301 · CJK 诊所界 · bidi שלום مرحبا"
	view := accessibilityView(t, agenttui.StatusReady, "מוכן ready", corpus+"\nsecond\tcolumn", corpus)
	driver, err := tuittest.NewDriver(tuittest.Options{
		Width: 80, Height: 20, Accessible: true, Initial: &view,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	screen, err := driver.Screen("unicode-accessible")
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.ValidateAccessibility(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"👩‍💻", "e\u0301", "诊所界", "שלום", "مرحبا"} {
		if !screen.Contains(expected) {
			t.Fatalf("accessible screen lost %q:\n%s", expected, screen.AgentReport())
		}
	}
	if strings.ContainsRune(screen.Plain(), '\t') || strings.ContainsRune(screen.Plain(), '�') {
		t.Fatalf("accessible screen contains tab or replacement rune:\n%s", screen.AgentReport())
	}
}

func TestKeyboardOnlyLifecycleCoversEditingHistoryAndEveryIntent(t *testing.T) {
	t.Parallel()
	session := tuittest.NewScriptSession()
	result, err := agenttui.NewCommandResult(mustText(t, "accepted"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if configureErr := session.SetPerformResult(result); configureErr != nil {
		t.Fatal(configureErr)
	}
	driver, err := tuittest.NewDriver(tuittest.Options{Width: 60, Height: 14, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.InjectUpdate(lifecycleSnapshot(
		t, 1, agenttui.StatusReady, "ready", []string{"connected"}, []string{"previous"},
	)); err != nil {
		t.Fatal(err)
	}

	const submitted = "👩‍💻e\u0301界שלום"
	for _, operation := range []func() error{
		func() error { return driver.Type(submitted) },
		func() error { return driver.Key("home") },
		func() error { return driver.Key("right") },
		func() error { return driver.Key("left") },
		func() error { return driver.Key("end") },
		func() error { return driver.Key("backspace") },
		func() error { return driver.Type("ם") },
	} {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	if driver.Prompt() != submitted {
		t.Fatalf("edited prompt = %q, want %q", driver.Prompt(), submitted)
	}
	if err := driver.Key("enter"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("response界"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key("alt+enter"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key("esc"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key("up"); err != nil {
		t.Fatal(err)
	}
	if driver.Prompt() != "response界" {
		t.Fatalf("history previous = %q", driver.Prompt())
	}
	if err := driver.Key("down"); err != nil {
		t.Fatal(err)
	}
	if driver.Prompt() != "" {
		t.Fatalf("history next = %q", driver.Prompt())
	}
	if err := driver.Key("ctrl+q"); err != nil {
		t.Fatal(err)
	}
	if !driver.QuitRequested() {
		t.Fatal("ctrl+q did not request quit")
	}
	intents := session.Intents()
	if len(intents) != 3 {
		t.Fatalf("intents = %d, want 3", len(intents))
	}
	if got := []agenttui.IntentKind{intents[0].Kind(), intents[1].Kind(), intents[2].Kind()}; !slices.Equal(got, []agenttui.IntentKind{
		agenttui.IntentSubmit, agenttui.IntentRespond, agenttui.IntentCancelActiveRun,
	}) {
		t.Fatalf("intent kinds = %v", got)
	}
	if intents[0].Values()[0].String() != submitted || intents[1].Values()[0].String() != "response界" {
		t.Fatalf("intent values = %#v, %#v", intents[0].Values(), intents[1].Values())
	}
}

func TestShippedPaletteContrastMeetsWCAGTextThreshold(t *testing.T) {
	t.Parallel()
	wantRoles := []string{"foreground", "muted", "accent", "warning", "failure"}
	for _, theme := range []agenttui.Theme{agenttui.LightTheme(), agenttui.DarkTheme()} {
		report, err := tuittest.AuditThemeContrast(theme)
		if err != nil {
			t.Fatal(err)
		}
		if report.ThemeName() != theme.Name() || report.Mode() != theme.Mode() {
			t.Fatalf("report identity = %q/%q", report.ThemeName(), report.Mode())
		}
		checks := report.Checks()
		roles := make([]string, 0, len(checks))
		for _, check := range checks {
			roles = append(roles, check.Role())
		}
		if !slices.Equal(roles, wantRoles) {
			t.Fatalf("contrast roles = %v", roles)
		}
		if err := report.Validate(tuittest.MinimumTextContrast); err != nil {
			t.Fatal(err)
		}
		checks[0] = tuittest.ContrastCheck{}
		if report.Checks()[0].Role() != "foreground" {
			t.Fatal("contrast report did not return a defensive copy")
		}
	}
	black, white := agenttui.NewColor(0, 0, 0), agenttui.NewColor(255, 255, 255)
	if ratio := tuittest.ContrastRatio(black, white); math.Abs(ratio-21) > 1e-9 {
		t.Fatalf("black/white ratio = %.12f", ratio)
	}
	if ratio := tuittest.ContrastRatio(white, black); math.Abs(ratio-21) > 1e-9 {
		t.Fatalf("white/black ratio = %.12f", ratio)
	}
	if ratio := tuittest.ContrastRatio(black, black); math.Abs(ratio-1) > 1e-9 {
		t.Fatalf("identical ratio = %.12f", ratio)
	}
}

func TestContrastAuditRejectsInvalidAndLowContrastInputs(t *testing.T) {
	t.Parallel()
	if err := (tuittest.ContrastReport{}).Validate(tuittest.MinimumTextContrast); err == nil {
		t.Fatal("expected empty report error")
	}
	if _, err := tuittest.AuditThemeContrast(nil); err == nil {
		t.Fatal("expected nil theme error")
	}
	white := agenttui.NewColor(255, 255, 255)
	theme, err := agenttui.NewTheme(
		"low-contrast", agenttui.ThemeLight,
		agenttui.NewPalette(white, white, white, white, white),
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := tuittest.AuditThemeContrast(theme)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(tuittest.MinimumTextContrast); err == nil {
		t.Fatal("expected low-contrast error")
	}
	for _, minimum := range []float64{math.NaN(), math.Inf(1), 0.99, 21.01} {
		if err := report.Validate(minimum); err == nil {
			t.Fatalf("expected minimum %v error", minimum)
		}
	}
}

func accessibilityView(
	t *testing.T,
	level agenttui.StatusLevel,
	statusMessage, body, prompt string,
) agenttui.ViewData {
	t.Helper()
	workspace, err := agenttui.NewWorkspace(mustText(t, "Accessibility"), []agenttui.Section{
		mustSection(t, "Corpus", body),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := agenttui.NewStatus(level, mustText(t, statusMessage), []agenttui.Text{mustText(t, "keyboard only")})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := agenttui.NewEditor(prompt)
	if err != nil {
		t.Fatal(err)
	}
	view, err := agenttui.NewViewData(workspace, status, editor, []agenttui.Text{mustText(t, "screen reader safe")})
	if err != nil {
		t.Fatal(err)
	}
	return view
}

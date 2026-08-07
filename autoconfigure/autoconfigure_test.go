package autoconfigure_test

import (
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice-agent-tui/autoconfigure"
)

func TestDefaultsAreCompleteReplaceableAndDoNotInventSession(t *testing.T) {
	t.Parallel()
	descriptor := autoconfigure.SpiceAutoConfiguration()
	if descriptor.Review != "docs/dependency-review.md" || len(descriptor.Beans) != 17 {
		t.Fatalf("SpiceAutoConfiguration() = %#v", descriptor)
	}
	wantNames := []string{
		"fixedRenderer", "darkTheme",
		"submitKeyBinding", "cancelKeyBinding", "respondKeyBinding", "quitKeyBinding",
		"cursorLeftKeyBinding", "cursorRightKeyBinding", "cursorStartKeyBinding", "cursorEndKeyBinding",
		"historyPreviousKeyBinding", "historyNextKeyBinding", "backspaceKeyBinding",
		"connectingView", "osTerminalIO", "terminalConfig", "terminalShell",
	}
	for index, bean := range descriptor.Beans {
		if bean.Name != wantNames[index] || !bean.Fallback || bean.Primary {
			t.Fatalf("Beans[%d] = %#v", index, bean)
		}
		if index >= 2 && index <= 12 && bean.Order != int64(index-2) {
			t.Fatalf("Beans[%d] order = %d, want %d", index, bean.Order, index-2)
		}
	}

	bindings := []func() (agenttui.KeyBinding, error){
		autoconfigure.DefaultSubmitBinding,
		autoconfigure.DefaultCancelBinding,
		autoconfigure.DefaultRespondBinding,
		autoconfigure.DefaultQuitBinding,
		autoconfigure.DefaultCursorLeftBinding,
		autoconfigure.DefaultCursorRightBinding,
		autoconfigure.DefaultCursorStartBinding,
		autoconfigure.DefaultCursorEndBinding,
		autoconfigure.DefaultHistoryPreviousBinding,
		autoconfigure.DefaultHistoryNextBinding,
		autoconfigure.DefaultBackspaceBinding,
	}
	values := make([]agenttui.KeyBinding, 0, len(bindings))
	for index, factory := range bindings {
		binding, err := factory()
		if err != nil {
			t.Fatalf("binding factory %d: %v", index, err)
		}
		values = append(values, binding)
	}
	wantBindings, err := agenttui.StandardKeyBindings()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != len(wantBindings) {
		t.Fatalf("default bindings = %d, want %d", len(values), len(wantBindings))
	}
	for index := range values {
		if values[index].Action() != wantBindings[index].Action() ||
			values[index].Help() != wantBindings[index].Help() {
			t.Fatalf("default binding %d = %#v", index, values[index])
		}
	}
	if view, err := autoconfigure.DefaultConnectingView(); err != nil || view.Status().Level() != agenttui.StatusReconnecting {
		t.Fatalf("DefaultConnectingView() = %#v, %v", view, err)
	}
	if streams, err := autoconfigure.DefaultOSTerminalIO(); err != nil || streams.Input() == nil || streams.Output() == nil {
		t.Fatalf("DefaultOSTerminalIO() = %#v, %v", streams, err)
	}
	if autoconfigure.DefaultDarkTheme().Mode() != agenttui.ThemeDark || autoconfigure.DefaultFixedRenderer() == nil {
		t.Fatal("theme or renderer fallback was not initialized")
	}
}

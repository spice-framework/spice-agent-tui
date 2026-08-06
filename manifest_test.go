package agenttui_test

import (
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice/starter"
)

func TestManifestIsCanonicalExplicitAndCompatible(t *testing.T) {
	t.Parallel()
	spec := agenttui.Manifest.Spec()
	if spec.ID != "github.com/spice-framework/spice-agent-tui" ||
		spec.Activation.Mode != starter.ActivationExplicitConstructor ||
		len(spec.Activation.EntryPoints) != 1 || spec.Activation.EntryPoints[0].Symbol != "NewViewData" ||
		len(spec.Annotations) != 0 || len(spec.ApplicationFeatures) != 0 {
		t.Fatalf("manifest = %#v", spec)
	}
	if err := agenttui.Manifest.Compatible(starter.APIVersion, "go1.26.5"); err != nil {
		t.Fatal(err)
	}
	content, err := agenttui.Manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(content), "\n") {
		t.Fatal("manifest JSON lacks final newline")
	}
}

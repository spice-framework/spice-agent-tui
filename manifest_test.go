package agenttui_test

import (
	"strings"
	"testing"

	agenttui "github.com/spice-framework/spice-agent-tui"
	"github.com/spice-framework/spice/starter"
)

func TestManifestIsCanonicalExplicitAndCompatible(t *testing.T) {
	t.Parallel()
	manifest := agenttui.Manifest()
	spec := manifest.Spec()
	if spec.ID != "github.com/spice-framework/spice-agent-tui" ||
		spec.Activation.Mode != starter.ActivationExplicitConstructor ||
		len(spec.Activation.EntryPoints) != 2 || spec.Activation.EntryPoints[0].Symbol != "NewFixedRenderer" ||
		spec.Activation.EntryPoints[1].Symbol != "NewShell" ||
		len(spec.Annotations) != 0 || len(spec.ApplicationFeatures) != 0 {
		t.Fatalf("manifest = %#v", spec)
	}
	if err := manifest.Compatible(starter.APIVersion, "go1.26.5"); err != nil {
		t.Fatal(err)
	}
	content, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(content), "\n") {
		t.Fatal("manifest JSON lacks final newline")
	}
}

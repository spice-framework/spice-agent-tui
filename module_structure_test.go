package agenttui_test

import (
	"bytes"
	"os"
	"testing"
)

func TestModulithHasOneRootAndOnlyPublicAnnotationsAreNamed(t *testing.T) {
	t.Parallel()
	root, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(root, []byte("// @Module")) != 1 || bytes.Contains(root, []byte("allowedDependencies")) {
		t.Fatalf("root module declaration = %q", root)
	}
	annotations, err := os.ReadFile("annotation/ui/doc.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(annotations, []byte("// @NamedInterface(\"annotations\")")) != 1 ||
		bytes.Contains(annotations, []byte("// @Module")) {
		t.Fatalf("annotation named interface = %q", annotations)
	}
	for path, name := range map[string]string{
		"autoconfigure/doc.go": "autoconfigure",
		"terminal/doc.go":      "terminal",
	} {
		content, readErr := os.ReadFile(path) // #nosec G304 -- fixed repository contract paths.
		if readErr != nil {
			t.Fatal(readErr)
		}
		marker := []byte("// @NamedInterface(\"" + name + "\")")
		if bytes.Count(content, marker) != 1 || bytes.Contains(content, []byte("// @Module")) {
			t.Fatalf("public %s named interface = %q", name, content)
		}
	}
	presentation, err := os.ReadFile("internal/presentation/doc.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(presentation, []byte("@NamedInterface")) || bytes.Contains(presentation, []byte("@Module")) {
		t.Fatalf("internal presentation became exposed = %q", presentation)
	}
}

func TestDescriptorsAndHandlersStayCanonicalAndGeneric(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"annotation/ui/ui_shell.go", "annotation/ui/ui_renderer.go"} {
		content, err := os.ReadFile(path) // #nosec G304 -- fixed repository contract paths.
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(content, []byte("func UI")) != 2 || !bytes.Contains(content, []byte("Handler(ctx context.Context")) {
			t.Fatalf("descriptor and handler are not canonical in %s", path)
		}
	}
	metadata, err := os.ReadFile("annotation/ui/metadata.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		[]byte("ContributionInterface"), []byte("RuntimeGraph"), []byte("reflect."),
		[]byte("agenttui.Shell"), []byte("agenttui.Renderer"),
		[]byte("invocation.Declaration.TypeID"),
	} {
		if bytes.Contains(metadata, forbidden) {
			t.Fatalf("generic metadata handler contains forbidden type logic %q", forbidden)
		}
	}
	for _, required := range [][]byte{
		[]byte("invocation.FunctionResultFacts()"),
		[]byte("providerResult.CanonicalTypeID"),
		[]byte("sdk.GoTypeInterface"),
	} {
		if !bytes.Contains(metadata, required) {
			t.Fatalf("generic metadata handler lacks result-fact contract %q", required)
		}
	}
}

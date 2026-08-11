package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	canonicalCodeStyleSHA256 = "1bdc7d2bd325e114fb05f30925141d62ededbfe5798f8e1f5aa4233ebc7823ec"
	styleConfigurationSHA256 = "2540781d27bde455e2465f785f3e5a32e4b5efa335af1a2a1da2ea74be06ade5"
	styleSourceRoot          = "internal/acceptance/composition"
	styleGeneratedRoot       = "internal/spicegen/compositionproof"
)

func checkStyleContract(root string) error {
	if err := checkStyleContractFile(root, "CODE_STYLE.md", canonicalCodeStyleSHA256); err != nil {
		return err
	}
	configuration, err := os.ReadFile(filepath.Join(root, ".spice", "style.json")) // #nosec G304 -- fixed repository-owned path.
	if err != nil {
		return fmt.Errorf("read .spice/style.json: %w", err)
	}
	if err := validateStyleApplicability(configuration); err != nil {
		return err
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(configuration)); actual != styleConfigurationSHA256 {
		return errors.New(".spice/style.json must match the exact reviewed schema-2 style contract")
	}
	return validateStyleSourceOwnership(root)
}

func checkStyleContractFile(root, path, wantHash string) error {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) // #nosec G304 -- fixed repository-owned path.
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(content)); actual != wantHash {
		return fmt.Errorf("%s must match the exact reviewed schema-2 style contract", path)
	}
	return nil
}

func validateStyleApplicability(content []byte) error {
	var document struct {
		SchemaVersion int                        `json:"schemaVersion"`
		Rules         map[string]json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		return fmt.Errorf("decode style applicability: %w", err)
	}
	if document.SchemaVersion != 2 {
		return errors.New("style applicability requires schemaVersion 2")
	}
	expected := []string{
		"onePrimaryTypePerFile", "methodsInPrimaryTypeFile", "fileNameMatchesType",
		"packageFunctions", "explicitConstructors", "explicitManagedScopes", "banInit",
		"banMutablePackageState", "privateManagedFields", "moduleOwnership",
		"routeClassification", "contextFirst", "errorLast", "maxTypeFileLines",
	}
	if len(document.Rules) != len(expected) {
		return errors.New("style applicability requires the exact schema-2 rule set")
	}
	for _, name := range expected {
		raw, found := document.Rules[name]
		if !found {
			return fmt.Errorf("style applicability is missing rule %s", name)
		}
		value := string(raw)
		switch name {
		case "maxTypeFileLines":
			if value != "500" {
				return errors.New("style applicability requires maxTypeFileLines 500")
			}
		case "moduleOwnership":
			if value == `"error"` {
				return errors.New(
					"toolchain v0.1.0-preview.4 schema-2 selections cannot include the module-root package '.'; moduleOwnership is inapplicable to this nested fixture and remains enforced by the separate root Modulith gate",
				)
			}
			if value != `"off"` {
				return errors.New("moduleOwnership must be the sole explicitly inapplicable style rule")
			}
		default:
			if value != `"error"` {
				return fmt.Errorf("only moduleOwnership may be disabled; rule %s must remain error", name)
			}
		}
	}
	return nil
}

func validateStyleSourceOwnership(root string) error {
	if err := validateHandwrittenStyleSource(root); err != nil {
		return err
	}
	return validateGeneratedStyleSource(root)
}

func validateHandwrittenStyleSource(root string) error {
	handwritten := filepath.Join(root, filepath.FromSlash(styleSourceRoot))
	return walkStyleSource(handwritten, func(path string, entry fs.DirEntry) error {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("style source must not contain symbolic link %s", path)
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".go") {
			return nil
		}
		generated, err := generatedStyleFile(path)
		if err != nil {
			return err
		}
		if generated {
			return fmt.Errorf("handwritten style source %s must not use a generated-file marker", path)
		}
		return nil
	})
}

func validateGeneratedStyleSource(root string) error {
	generatedRoot := filepath.Join(root, filepath.FromSlash(styleGeneratedRoot))
	generatedFiles := 0
	if err := walkStyleSource(filepath.Join(root, "internal", "spicegen"), func(path string, entry fs.DirEntry) error {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated style source must not contain symbolic link %s", path)
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".go") {
			return nil
		}
		relative, err := filepath.Rel(generatedRoot, path)
		if err != nil || relative == "." || !filepath.IsLocal(relative) {
			return fmt.Errorf("generated Go source %s is outside exact root %s", path, styleGeneratedRoot)
		}
		generated, err := generatedStyleFile(path)
		if err != nil {
			return err
		}
		if !generated {
			return fmt.Errorf("generated style source %s is missing the standard generated-file marker", path)
		}
		generatedFiles++
		return nil
	}); err != nil {
		return err
	}
	if generatedFiles == 0 {
		return errors.New("exact style generated root contains no generated Go source")
	}
	return nil
}

func walkStyleSource(root string, visit func(string, fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return visit(path, entry)
	})
}

func generatedStyleFile(path string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse style source %s: %w", path, err)
	}
	return ast.IsGenerated(file), nil
}

func checkStyle(ctx context.Context, root string) error {
	if err := checkStyleContract(root); err != nil {
		return err
	}
	executable, err := toolPath(ctx, root, "spicestyle")
	if err != nil {
		return err
	}
	environment := map[string]string{
		"GOFLAGS": "-mod=vendor", "GOPROXY": "off", "GOSUMDB": "off",
		"GOTOOLCHAIN": "local", "GOWORK": "off",
	}
	return command(ctx, root, environment, executable, styleArguments()...)
}

func styleArguments() []string {
	return []string{"--config=.spice/style.json", "./internal/acceptance/composition"}
}

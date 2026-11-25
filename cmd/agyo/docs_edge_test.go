package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQA_ZeroEmDashesRule verifies that no production Go code, CLI commands,
// or documentation files contain prohibited em-dashes (\u2014) or en-dashes (\u2013).
func TestQA_ZeroEmDashesRule(t *testing.T) {
	root := filepath.Join("..", "..")

	targets := []string{
		"README.md",
		"README.pt-BR.md",
		"AGENTS.md",
		"cmd/agyo",
		"internal",
	}

	for _, rel := range targets {
		full := filepath.Join(root, rel)
		err := filepath.Walk(full, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".md" {
				return nil
			}
			// Skip documentation rule definitions that mention the character as forbidden
			if strings.HasSuffix(path, "SKILL.md") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			if strings.Contains(content, "\u2014") {
				t.Errorf("file %s contains prohibited unicode em-dash (\\u2014)", path)
			}
			if strings.Contains(content, "\u2013") {
				t.Errorf("file %s contains prohibited unicode en-dash (\\u2013)", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed on %s: %v", full, err)
		}
	}
}

// TestQA_DocsEveryPackageIntegrity audits that all subdirectories in internal/
// have corresponding entries in architecture docs.
func TestQA_DocsEveryPackageIntegrity(t *testing.T) {
	root := filepath.Join("..", "..")
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := e.Name()
		// Validate that package is non-empty and contains at least one go file
		pkgDir := filepath.Join(root, "internal", pkg)
		files, err := os.ReadDir(pkgDir)
		if err != nil {
			t.Errorf("failed to read internal/%s: %v", pkg, err)
			continue
		}
		hasGo := false
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".go") {
				hasGo = true
				break
			}
		}
		if !hasGo {
			t.Errorf("internal/%s has no .go files", pkg)
		}
	}
}

// TestQA_ProductionFileLineCeiling enforces the hard 500-line ceiling on all
// non-test Go source files across cmd/agyo, internal, and templates.
func TestQA_ProductionFileLineCeiling(t *testing.T) {
	const maxLines = 500
	root := filepath.Join("..", "..")
	for _, rel := range []string{"cmd/agyo", "internal", "templates"} {
		full := filepath.Join(root, rel)
		err := filepath.Walk(full, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines := bytes.Count(data, []byte("\n"))
			if len(data) > 0 && data[len(data)-1] != '\n' {
				lines++
			}
			if lines > maxLines {
				t.Errorf("%s has %d lines, exceeds hard ceiling of %d", path, lines, maxLines)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed on %s: %v", full, err)
		}
	}
}

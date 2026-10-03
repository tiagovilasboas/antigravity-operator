package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsMapEveryPackage keeps AGENTS.md and both READMEs in step with the
// code: every package under internal/ must appear in the AGENTS.md
// architecture map and in the EN and PT README architecture trees.
func TestDocsMapEveryPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{}
	for _, f := range []string{"AGENTS.md", "README.md", "README.pt-BR.md"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		docs[f] = string(b)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := e.Name()
		if !strings.Contains(docs["AGENTS.md"], "`internal/"+pkg+"/`") {
			t.Errorf("AGENTS.md architecture map is missing internal/%s/", pkg)
		}
		for _, f := range []string{"README.md", "README.pt-BR.md"} {
			if !strings.Contains(docs[f], "── "+pkg+"/") {
				t.Errorf("%s architecture tree is missing %s/", f, pkg)
			}
		}
	}
}

// TestReadmeParity checks that README.md and README.pt-BR.md keep the same
// section structure (## and ### headings, in order of level).
func TestReadmeParity(t *testing.T) {
	levels := func(f string) []string {
		b, err := os.ReadFile(filepath.Join("..", "..", f))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		inCode := false
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				inCode = !inCode
				continue
			}
			if inCode {
				continue
			}
			if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
				out = append(out, strings.SplitN(l, " ", 2)[0])
			}
		}
		return out
	}
	en, pt := levels("README.md"), levels("README.pt-BR.md")
	if strings.Join(en, ",") != strings.Join(pt, ",") {
		t.Fatalf("README section structure differs:\nEN %v\nPT %v", en, pt)
	}
}

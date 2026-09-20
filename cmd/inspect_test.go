package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/analysis"
)

func TestInspectCommandJSON(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	args := []string{
		"inspect", "chunk-C.js",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var result analysis.InspectResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Chunk != "browser/chunk-C.js" || result.Bytes != 204800 || result.Initial {
		t.Fatalf("chunk: %+v", result)
	}
	if len(result.Packages) != 1 || result.Packages[0] != (analysis.Contributor{Name: "pdfjs-dist", Bytes: 184320}) {
		t.Fatalf("packages: %+v", result.Packages)
	}
}

func TestInspectCommandText(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	args := []string{
		"inspect", "chunk-C.js",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for _, want := range []string{"Chunk Inspection", "browser/chunk-C.js", "LAZY", "pdfjs-dist", "node_modules/pdfjs-dist/a.js"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

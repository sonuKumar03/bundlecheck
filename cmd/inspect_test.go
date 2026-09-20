package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/analysis"
)

func findAnyJSChunk(t *testing.T, base string) string {
	entries, err := os.ReadDir(filepath.Join(base, "browser"))
	if err != nil {
		t.Fatalf("read browser dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			return e.Name()
		}
	}
	t.Fatalf("no .js chunks found in %s/browser", base)
	return ""
}

func TestInspectCommandJSON(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	targetChunk := findAnyJSChunk(t, base)
	args := []string{
		"inspect", targetChunk,
		"-s", base,
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
	if !strings.HasSuffix(result.Chunk, targetChunk) || result.Bytes <= 0 {
		t.Fatalf("invalid inspected chunk result: %+v", result)
	}
	if len(result.Packages) == 0 {
		t.Fatalf("expected packages in inspected chunk: %+v", result)
	}
	for _, p := range result.Packages {
		if p.Bytes <= 0 || p.Name == "" {
			t.Fatalf("invalid package contributor in chunk: %+v", p)
		}
	}
}

func TestInspectCommandText(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	targetChunk := findAnyJSChunk(t, base)
	args := []string{
		"inspect", targetChunk,
		"-s", base,
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Chunk Inspection") || !strings.Contains(out.String(), targetChunk) {
		t.Fatalf("output missing chunk inspection details for %s:\n%s", targetChunk, out.String())
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"bundlecheck/internal/advisor"
)

func TestSuggestCommand(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")

	// 1. JSON test for suggest command
	argsJSON := []string{
		"suggest",
		base,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(argsJSON, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	var res advisor.AdvisorResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestions for lazy-import fixture")
	}

	// 2. Text test for suggest command with --gzip
	out.Reset()
	errOut.Reset()
	argsText := []string{
		"suggest",
		base,
		"--gzip",
	}

	if code := Execute(argsText, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	if !strings.Contains(out.String(), "Initial Bundle Culprits") || !strings.Contains(out.String(), "Initial JS Impact") {
		t.Errorf("unexpected text output: %s", out.String())
	}
}

func TestSummaryWithGzipAndSuggest(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")

	args := []string{
		"summary",
		base,
		"--gzip",
		"--suggest",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	if !strings.Contains(out.String(), "gzip") || !strings.Contains(out.String(), "Initial Bundle Culprits") {
		t.Errorf("expected gzip and suggestions in summary output: %s", out.String())
	}
}

func TestSuggestMarkdown(t *testing.T) {
	base := filepath.Join("..", "testdata", "lazy-import")
	args := []string{
		"suggest",
		base,
		"-f", "markdown",
		"--gzip",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "## 🔍 Initial Bundle Culprits & Contributors") {
		t.Errorf("expected suggestions markdown header, got: %s", output)
	}
	if !strings.Contains(output, "### 1.") {
		t.Errorf("expected numbered recommendation item, got: %s", output)
	}
}


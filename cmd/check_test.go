package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCommand(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")

	// Pass: max initial 2KB (minimal fixture is 1KB)
	argsPass := []string{
		"check",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
		"--max-initial", "2KB",
	}
	var out, errOut bytes.Buffer
	if code := Execute(argsPass, &out, &errOut); code != 0 {
		t.Fatalf("expected pass, got exit %d: %s", code, errOut.String())
	}

	// Fail: max initial 500B
	argsFail := []string{
		"check",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
		"--max-initial", "500B",
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(argsFail, &out, &errOut); code == 0 {
		t.Fatal("expected failure on budget exceeded, got exit 0")
	}
}

func TestCheckMarkdown(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")

	// Pass with markdown format
	argsPass := []string{
		"check",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
		"--max-initial", "2KB",
		"-f", "markdown",
	}
	var out, errOut bytes.Buffer
	if code := Execute(argsPass, &out, &errOut); code != 0 {
		t.Fatalf("expected pass, got exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "## ✅ Angular Bundle Budget Check: PASSED") {
		t.Errorf("expected passed markdown header, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "All configured bundle size budgets and delta thresholds passed successfully.") {
		t.Errorf("expected budget pass message, got: %s", out.String())
	}
}


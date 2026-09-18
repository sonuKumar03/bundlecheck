package cmd

import (
	"bytes"
	"os"
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

func TestCheckPositionalAndConfigFile(t *testing.T) {
	origWd, _ := os.Getwd()
	tmpWd := t.TempDir()
	_ = os.Chdir(tmpWd)
	defer func() { _ = os.Chdir(origWd) }()

	base := filepath.Join(origWd, "..", "testdata", "minimal")
	absBase, _ := filepath.Abs(base)
	statsFile := filepath.Join(absBase, "stats.json")
	distDir := filepath.Join(absBase, "browser")

	// 1. Write config file that disallows lodash
	cfgPath := filepath.Join(tmpWd, ".bundlecheck.yml")
	cfgContent := `
budgets:
  initial_js_max: 5MB
rules:
  disallow_packages:
    - lodash
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run check with positional args
	var out, errOut bytes.Buffer
	code := Execute([]string{"check", statsFile, distDir}, &out, &errOut)
	if code == 0 {
		t.Fatal("expected check failure due to disallowed package lodash in .bundlecheck.yml")
	}
	combined := out.String() + "\n" + errOut.String()
	if !strings.Contains(combined, "lodash") {
		t.Errorf("expected lodash violation in output: %s", combined)
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

func TestCheckRejectsMalformedAutoLoadedConfig(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".bundlecheck.yml"), []byte("budgets: [invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	base := filepath.Join(origWd, "..", "testdata", "minimal")
	var out, errOut bytes.Buffer
	code := Execute([]string{"check", "-s", filepath.Join(base, "stats.json"), "-d", filepath.Join(base, "browser"), "--max-total", "1MB"}, &out, &errOut)
	if code == 0 || !strings.Contains(errOut.String(), "config") {
		t.Fatalf("invalid config must fail before checking permissive CLI budgets: exit=%d err=%s", code, errOut.String())
	}
}

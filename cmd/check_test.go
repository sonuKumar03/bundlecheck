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
		base,
		"--max-initial", "2KB",
	}
	var out, errOut bytes.Buffer
	if code := Execute(argsPass, &out, &errOut); code != 0 {
		t.Fatalf("expected pass, got exit %d: %s", code, errOut.String())
	}

	// Fail: max initial 500B
	argsFail := []string{
		"check",
		base,
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
	cfgPath := filepath.Join(tmpWd, ".bundleradar.yml")
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
		t.Fatal("expected check failure due to disallowed package lodash in .bundleradar.yml")
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
	if err := os.WriteFile(filepath.Join(root, ".bundleradar.yml"), []byte("budgets: [invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	base := filepath.Join(origWd, "..", "testdata", "minimal")
	var out, errOut bytes.Buffer
	code := Execute([]string{"check", base, "--max-total", "1MB"}, &out, &errOut)
	if code == 0 || !strings.Contains(errOut.String(), "config") {
		t.Fatalf("invalid config must fail before checking permissive CLI budgets: exit=%d err=%s", code, errOut.String())
	}
}

func TestCheckReportOnlyWhenUnconfigured(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")

	// 1. Text format report-only
	var out, errOut bytes.Buffer
	code := Execute([]string{"check", base}, &out, &errOut)
	if code != 0 {
		t.Fatalf("unconfigured check must succeed in report-only mode, got exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Bundle Budget Check: PASSED") {
		t.Errorf("expected pass message, got: %s", out.String())
	}

	// 2. JSON format report-only
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"check", base, "-f", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("unconfigured check JSON must succeed, got exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"passed": true`) {
		t.Errorf("expected passed: true in JSON, got: %s", out.String())
	}
	if !strings.Contains(out.String(), `"initialJs"`) {
		t.Errorf("expected initialJs in JSON summary, got: %s", out.String())
	}
}

func TestCheckExplicitFlagOverridesConfig(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// Config has strict 10B limit that would fail
	cfgContent := "budgets:\n  initial_js_max: 10B\n"
	if err := os.WriteFile(filepath.Join(root, ".bundleradar.yml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	base := filepath.Join(origWd, "..", "testdata", "minimal")

	// Without CLI flag, it must fail because of config 10B
	var out, errOut bytes.Buffer
	if code := Execute([]string{"check", base}, &out, &errOut); code == 0 {
		t.Fatal("expected failure from auto-loaded config 10B limit")
	}

	// With CLI flag --max-initial 10MB, the CLI flag must override the config and pass
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"check", base, "--max-initial", "10MB"}, &out, &errOut); code != 0 {
		t.Fatalf("CLI flag must override config file, expected pass but got exit %d: %s", code, errOut.String())
	}
}

func TestCheckExplicitConfigFileOverridesAutoLoaded(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// Auto-loaded config in current directory has failing 10B limit
	if err := os.WriteFile(filepath.Join(root, ".bundleradar.yml"), []byte("budgets:\n  initial_js_max: 10B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Explicit custom config has permissive 10MB limit
	customCfg := filepath.Join(root, "custom.yml")
	if err := os.WriteFile(customCfg, []byte("budgets:\n  initial_js_max: 10MB\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	base := filepath.Join(origWd, "..", "testdata", "minimal")

	var out, errOut bytes.Buffer
	code := Execute([]string{"check", base, "-c", customCfg}, &out, &errOut)
	if code != 0 {
		t.Fatalf("explicit --config must take precedence over auto-loaded config, got exit %d: %s", code, errOut.String())
	}
}

func TestCheck_MultiApp_ProjectsAndAppFlags(t *testing.T) {
	tmp := t.TempDir()
	portalDist := filepath.Join(tmp, "dist", "portal", "browser")
	adminDist := filepath.Join(tmp, "dist", "admin", "browser")
	_ = os.MkdirAll(portalDist, 0755)
	_ = os.MkdirAll(adminDist, 0755)

	statsData, _ := os.ReadFile("../testdata/minimal/stats.json")
	indexHTML, _ := os.ReadFile("../testdata/minimal/browser/index.html")
	mainJS, _ := os.ReadFile("../testdata/minimal/browser/main.js")

	_ = os.WriteFile(filepath.Join(tmp, "dist", "portal", "stats.json"), statsData, 0644)
	_ = os.WriteFile(filepath.Join(tmp, "dist", "admin", "stats.json"), statsData, 0644)
	_ = os.WriteFile(filepath.Join(portalDist, "index.html"), indexHTML, 0644)
	_ = os.WriteFile(filepath.Join(adminDist, "index.html"), indexHTML, 0644)
	_ = os.WriteFile(filepath.Join(portalDist, "main.js"), mainJS, 0644)
	_ = os.WriteFile(filepath.Join(adminDist, "main.js"), mainJS, 0644)

	origWd, _ := os.Getwd()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	t.Run("Multi-app check passes with permissive budget", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"check", "--projects", "portal,admin", "--max-initial", "2KB", "-f", "json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected check to pass, got %d: %s", code, errOut.String())
		}
		if !strings.Contains(out.String(), `"passed": true`) {
			t.Errorf("expected passed: true in JSON output: %s", out.String())
		}
		if !strings.Contains(out.String(), `"portal"`) || !strings.Contains(out.String(), `"admin"`) {
			t.Errorf("expected portal and admin in JSON output: %s", out.String())
		}
	})

	t.Run("Multi-app check fails with strict budget", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute([]string{"check", "--projects", "portal,admin", "--max-initial", "500B", "-f", "text"}, &out, &errOut)
		if code != ExitCodePolicyViolation {
			t.Fatalf("expected policy violation exit code 1, got %d: %s", code, errOut.String())
		}
		if !strings.Contains(out.String(), "FAILED") {
			t.Errorf("expected FAILED in text output: %s", out.String())
		}
	})

	t.Run("Multi-app check with explicit --app flags and markdown output", func(t *testing.T) {
		var out, errOut bytes.Buffer
		app1 := "portal=" + filepath.Join(tmp, "dist", "portal", "stats.json") + ":" + portalDist
		app2 := "admin=" + filepath.Join(tmp, "dist", "admin", "stats.json") + ":" + adminDist
		code := Execute([]string{"check", "--app", app1, "--app", app2, "--max-initial", "2KB", "-f", "markdown"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected check to pass, got %d: %s", code, errOut.String())
		}
		if !strings.Contains(out.String(), "BundleRadar Multi-App Budget Report") {
			t.Errorf("expected Multi-App Budget Report header in markdown output: %s", out.String())
		}
		if !strings.Contains(out.String(), "portal") || !strings.Contains(out.String(), "admin") {
			t.Errorf("expected apps in markdown table: %s", out.String())
		}
	})
}

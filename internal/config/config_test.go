package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/budget"
	"bundlecheck/internal/config"
	"bundlecheck/internal/snapshot"
)

func TestConfigLoadAndApply(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".bundlecheck.yml")

	yamlContent := `
budgets:
  initial_js_max: 200KB
  total_max: 1MB
  max_delta_increase: 25KB

rules:
  disallow_packages:
    - moment
    - lodash
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, foundPath, err := config.FindAndLoad(tmp)
	if err != nil {
		t.Fatalf("find and load: %v", err)
	}
	if cfg == nil || foundPath != cfgPath {
		t.Fatalf("expected config found at %q, got %q", cfgPath, foundPath)
	}

	var limits budget.Limits
	if err := cfg.ApplyToLimits(&limits); err != nil {
		t.Fatalf("apply limits: %v", err)
	}

	if limits.MaxInitial == nil || *limits.MaxInitial != 200*1024 {
		t.Errorf("expected MaxInitial 204800, got %v", limits.MaxInitial)
	}
	if limits.MaxTotalDelta == nil || *limits.MaxTotalDelta != 25*1024 {
		t.Errorf("expected MaxTotalDelta 25600, got %v", limits.MaxTotalDelta)
	}

	// Test rule violations
	res := &analysis.AnalysisResult{
		Packages: []snapshot.Package{
			{Name: "moment", TotalBytes: 5000},
			{Name: "rxjs", TotalBytes: 10000},
		},
	}
	violations := cfg.CheckRules(res)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for moment, got %d", len(violations))
	}
}

func TestFindConfigInParentDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "app", "src")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, ".bundlecheck.yml")
	if err := os.WriteFile(configPath, []byte("budgets:\n  initial_js_max: 1B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, found, err := config.FindAndLoad(child)
	if err != nil || cfg == nil || found != configPath {
		t.Fatalf("parent config not loaded: cfg=%v path=%q err=%v", cfg, found, err)
	}
}

func TestRejectUnsupportedConfigFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bundlecheck.yml")
	if err := os.WriteFile(path, []byte("budgets:\n  initial_js_typo: 1B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadFile(path); err == nil {
		t.Fatal("unsupported budget setting must not silently disable enforcement")
	}
}

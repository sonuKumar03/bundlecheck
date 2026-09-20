package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/budget"
	"github.com/sonuKumar03/bundlecheck/internal/config"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
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

func TestDocsConfigLoadsCleanly(t *testing.T) {
	// Exact configuration snippet published on docs/index.html and README.md
	docConfig := `
budgets:
  initial_js_max: 250kb
  total_max: 1.5mb
  max_delta_increase: 50kb

rules:
  disallow_packages:
    - moment
    - lodash
`
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".bundlecheck.yml")
	if err := os.WriteFile(cfgPath, []byte(docConfig), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("published docs config failed to load: %v", err)
	}
	if cfg.Budgets.InitialJSMax != "250kb" {
		t.Errorf("expected initial_js_max 250kb, got %q", cfg.Budgets.InitialJSMax)
	}
	if cfg.Budgets.TotalMax != "1.5mb" {
		t.Errorf("expected total_max 1.5mb, got %q", cfg.Budgets.TotalMax)
	}
	if cfg.Budgets.MaxDeltaIncrease != "50kb" {
		t.Errorf("expected max_delta_increase 50kb, got %q", cfg.Budgets.MaxDeltaIncrease)
	}
	if len(cfg.Rules.DisallowPackages) != 2 || cfg.Rules.DisallowPackages[0] != "moment" || cfg.Rules.DisallowPackages[1] != "lodash" {
		t.Errorf("expected disallow_packages [moment, lodash], got %v", cfg.Rules.DisallowPackages)
	}
}

func TestRenderYAMLAndLoadRoundTrip(t *testing.T) {
	orig := &config.Config{
		Budgets: config.ConfigBudgets{
			InitialJSMax: "500KB",
			TotalMax:     "1.5MB",
		},
		Rules: config.ConfigRules{
			DisallowPackages: []string{"moment", "lodash"},
		},
	}

	yamlData, err := config.RenderYAML(orig)
	if err != nil {
		t.Fatalf("render yaml: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), ".bundlecheck.yml")
	if err := os.WriteFile(tmpFile, yamlData, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.LoadFile(tmpFile)
	if err != nil {
		t.Fatalf("load rendered yaml: %v", err)
	}

	if loaded.Budgets.InitialJSMax != orig.Budgets.InitialJSMax {
		t.Errorf("expected %s, got %s", orig.Budgets.InitialJSMax, loaded.Budgets.InitialJSMax)
	}
	if loaded.Budgets.TotalMax != orig.Budgets.TotalMax {
		t.Errorf("expected %s, got %s", orig.Budgets.TotalMax, loaded.Budgets.TotalMax)
	}
	if len(loaded.Rules.DisallowPackages) != 2 {
		t.Errorf("expected 2 disallowed packages, got %v", loaded.Rules.DisallowPackages)
	}
}

func TestLoadFromAngularJSON(t *testing.T) {
	angularJSON := `{
		"projects": {
			"portal": {
				"architect": {
					"build": {
						"configurations": {
							"production": {
								"budgets": [
									{
										"type": "initial",
										"maximumError": "500kb"
									},
									{
										"type": "all",
										"maximumError": "1.2mb"
									}
								]
							}
						}
					}
				}
			}
		}
	}`

	tmp := t.TempDir()
	path := filepath.Join(tmp, "angular.json")
	if err := os.WriteFile(path, []byte(angularJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromAngularJSON(path, "portal")
	if err != nil {
		t.Fatalf("LoadFromAngularJSON: %v", err)
	}
	if cfg.Budgets.InitialJSMax != "500KB" {
		t.Errorf("expected 500KB, got %q", cfg.Budgets.InitialJSMax)
	}
	if cfg.Budgets.TotalMax != "1.2MB" {
		t.Errorf("expected 1.2MB, got %q", cfg.Budgets.TotalMax)
	}
}

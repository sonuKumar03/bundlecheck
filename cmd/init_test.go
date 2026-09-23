package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/config"
)

func TestInitCommand_Preview(t *testing.T) {
	minimalDir, err := filepath.Abs(filepath.Join("..", "testdata", "minimal"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Execute([]string{"init", minimalDir, "--headroom", "10"}, &stdout, &stderr)
	if code != ExitCodeSuccess {
		t.Fatalf("init preview failed with code %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Proposed configuration with 10.0% headroom") {
		t.Errorf("missing headroom explanation in preview:\n%s", out)
	}
	if !strings.Contains(out, "initial_js_max:") {
		t.Errorf("missing initial_js_max in YAML output:\n%s", out)
	}
	if !strings.Contains(out, "total_max:") {
		t.Errorf("missing total_max in YAML output:\n%s", out)
	}

	// Verify the directory does NOT have .bundleradar.yml written in preview mode
	if _, err := os.Stat(filepath.Join(minimalDir, ".bundleradar.yml")); err == nil {
		t.Errorf("preview mode must NOT write .bundleradar.yml to disk")
	}
}

func TestInitCommand_WriteAndRefusal(t *testing.T) {
	minimalDir, err := filepath.Abs(filepath.Join("..", "testdata", "minimal"))
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	// Copy stats.json and browser into tmpDir
	statsData, err := os.ReadFile(filepath.Join(minimalDir, "stats.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "stats.json"), statsData, 0644); err != nil {
		t.Fatal(err)
	}
	browserDir := filepath.Join(tmpDir, "browser")
	if err := os.MkdirAll(browserDir, 0755); err != nil {
		t.Fatal(err)
	}
	indexData, err := os.ReadFile(filepath.Join(minimalDir, "browser", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(browserDir, "index.html"), indexData, 0644); err != nil {
		t.Fatal(err)
	}
	mainData, err := os.ReadFile(filepath.Join(minimalDir, "browser", "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(browserDir, "main.js"), mainData, 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Initial write should succeed
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"init", tmpDir, "--write", "--headroom", "5"}, &stdout, &stderr)
	if code != ExitCodeSuccess {
		t.Fatalf("init write failed with code %d: %s", code, stderr.String())
	}
	targetFile := filepath.Join(tmpDir, ".bundleradar.yml")
	if _, err := os.Stat(targetFile); err != nil {
		t.Fatalf(".bundleradar.yml was not created on disk: %v", err)
	}

	// Verify written file is strictly valid YAML loadable by config parser
	loaded, err := config.LoadFile(targetFile)
	if err != nil {
		t.Fatalf("written file failed strict YAML parse: %v", err)
	}
	if loaded.Budgets.InitialJSMax == "" {
		t.Errorf("written config has empty initial_js_max")
	}

	// 2. Second write should REFUSE to overwrite
	stdout.Reset()
	stderr.Reset()
	code = Execute([]string{"init", tmpDir, "--write"}, &stdout, &stderr)
	if code != ExitCodeUsage {
		t.Fatalf("expected refusal with exit code 2, got code %d: %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "already exists; refusing to overwrite") {
		t.Errorf("expected refusal error message, got: %s", stderr.String())
	}
}

func TestInitCommand_FromAngularBudgets(t *testing.T) {
	tmpDir := t.TempDir()
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
										"maximumError": "450kb"
									},
									{
										"type": "all",
										"maximumError": "1.5mb"
									}
								]
							}
						}
					}
				}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "angular.json"), []byte(angularJSON), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Execute([]string{"init", tmpDir, "--from-angular-budgets", "--project", "portal"}, &stdout, &stderr)
	if code != ExitCodeSuccess {
		t.Fatalf("init from angular budgets failed: %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "initial_js_max: 450KB") {
		t.Errorf("expected 450KB in preview, got:\n%s", out)
	}
	if !strings.Contains(out, "total_max: 1.5MB") {
		t.Errorf("expected 1.5MB in preview, got:\n%s", out)
	}
}

func TestInitCommand_MissingArtifacts(t *testing.T) {
	emptyDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"init", emptyDir}, &stdout, &stderr)
	if code != ExitCodeExecution {
		t.Fatalf("expected exit code 3 on missing artifacts, got %d: %s", code, stderr.String())
	}
}

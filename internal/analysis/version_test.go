package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestVersionSyncContract verifies that root VERSION, ToolVersion,
// npm/bundlecheck/package.json, docs/index.html, and README.md stay in exact sync.
// If any file drifts, this test fails in local dev and GitHub Actions CI.
func TestVersionSyncContract(t *testing.T) {
	// Locate repository root relative to internal/analysis/
	repoRoot, err := filepath.Abs(filepath.Join(".", "..", ".."))
	if err != nil {
		t.Fatalf("failed to determine repo root: %v", err)
	}

	versionFile := filepath.Join(repoRoot, "VERSION")
	versionBytes, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatalf("failed to read VERSION file at %s: %v", versionFile, err)
	}
	rootVersion := strings.TrimSpace(string(versionBytes))
	if rootVersion == "" {
		t.Fatalf("VERSION file is empty")
	}

	// 1. Check ToolVersion in analysis package
	if ToolVersion != rootVersion {
		t.Errorf("internal/analysis.ToolVersion mismatch: got %q, want %q (from VERSION)", ToolVersion, rootVersion)
	}

	// 2. Check npm/bundlecheck/package.json
	pkgFile := filepath.Join(repoRoot, "npm", "bundlecheck", "package.json")
	pkgBytes, err := os.ReadFile(pkgFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", pkgFile, err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgBytes, &pkg); err != nil {
		t.Fatalf("failed to parse package.json: %v", err)
	}
	if pkg.Version != rootVersion {
		t.Errorf("npm/bundlecheck/package.json version mismatch: got %q, want %q", pkg.Version, rootVersion)
	}

	// 3. Check docs/index.html contains softwareVersion matching rootVersion
	indexHTMLFile := filepath.Join(repoRoot, "docs", "index.html")
	indexHTMLBytes, err := os.ReadFile(indexHTMLFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", indexHTMLFile, err)
	}
	indexHTML := string(indexHTMLBytes)
	softwareVersionPattern := regexp.MustCompile(`"softwareVersion":\s*"([^"]+)"`)
	match := softwareVersionPattern.FindStringSubmatch(indexHTML)
	if len(match) < 2 {
		t.Errorf("docs/index.html missing 'softwareVersion' JSON-LD schema field")
	} else if match[1] != rootVersion {
		t.Errorf("docs/index.html softwareVersion mismatch: got %q, want %q", match[1], rootVersion)
	}

	// 4. Check README.md contains GitHub Action ref @v<rootVersion>
	readmeFile := filepath.Join(repoRoot, "README.md")
	readmeBytes, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmeFile, err)
	}
	readme := string(readmeBytes)
	actionRefPattern := regexp.MustCompile(`uses:\s*sonuKumar03/bundlecheck@v([0-9.]+)`)
	actionMatch := actionRefPattern.FindStringSubmatch(readme)
	if len(actionMatch) < 2 {
		t.Errorf("README.md missing GitHub Action reference 'sonuKumar03/bundlecheck@v...'")
	} else if actionMatch[1] != rootVersion {
		t.Errorf("README.md Action reference version mismatch: got %q, want %q", actionMatch[1], rootVersion)
	}

	// 5. Check go.mod declares the correct module path
	goModFile := filepath.Join(repoRoot, "go.mod")
	goModBytes, err := os.ReadFile(goModFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", goModFile, err)
	}
	expectedModule := "module github.com/sonuKumar03/bundlecheck"
	if !strings.Contains(string(goModBytes), expectedModule) {
		t.Errorf("go.mod missing expected module declaration %q", expectedModule)
	}
}

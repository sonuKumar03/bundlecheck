package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionConsistency(t *testing.T) {
	// 1. Read root VERSION file
	versionFilePath := filepath.Join("..", "..", "VERSION")
	versionBytes, err := os.ReadFile(versionFilePath)
	if err != nil {
		t.Fatalf("failed to read root VERSION file: %v", err)
	}
	rootVersion := strings.TrimSpace(string(versionBytes))

	if rootVersion != ToolVersion {
		t.Errorf("version mismatch: root VERSION file (%q) != analysis.ToolVersion (%q)", rootVersion, ToolVersion)
	}

	// 2. Read npm/bundlecheck/package.json
	pkgFilePath := filepath.Join("..", "..", "npm", "bundlecheck", "package.json")
	pkgBytes, err := os.ReadFile(pkgFilePath)
	if err != nil {
		t.Fatalf("failed to read npm/bundlecheck/package.json: %v", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgBytes, &pkg); err != nil {
		t.Fatalf("failed to unmarshal npm package.json: %v", err)
	}

	if pkg.Version != ToolVersion {
		t.Errorf("version mismatch: npm package.json (%q) != analysis.ToolVersion (%q)", pkg.Version, ToolVersion)
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bundlecheck/internal/analysis"
)

func TestBaselineCommand(t *testing.T) {
	base := filepath.Join("..", "testdata", "minimal")
	targetFile := filepath.Join(t.TempDir(), "baseline.json")

	args := []string{
		"baseline",
		"-s", filepath.Join(base, "stats.json"),
		"-d", filepath.Join(base, "browser"),
		"-o", targetFile,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read written baseline: %v", err)
	}

	var saved analysis.AnalysisResult
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("invalid json in saved baseline: %v", err)
	}
	if saved.Summary.InitialJS != 1024 {
		t.Errorf("expected InitialJS 1024, got %d", saved.Summary.InitialJS)
	}
}

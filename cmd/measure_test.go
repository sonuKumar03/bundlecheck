package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"bundlecheck/internal/comparison"
)

func TestMeasureCommandSuccess(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	args := []string{
		"measure",
		"-s", filepath.Join(afterBase, "stats.json"),
		"-d", filepath.Join(afterBase, "browser"),
		"-b", baselinePath,
		"-f", "json",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	var res comparison.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout not valid json: %v", err)
	}
	if res.Summary.After.InitialJS != 163840 {
		t.Errorf("expected after InitialJS 163840, got %d", res.Summary.After.InitialJS)
	}
}

func TestMeasureRegressionBudget(t *testing.T) {
	baselinePath := filepath.Join("..", "testdata", "comparison", "before.json")
	afterBase := filepath.Join("..", "testdata", "lazy-import")

	// Expect fail: max initial delta set to 0, but lazy-import initial JS is much bigger than before.json (100 vs 163840)
	args := []string{
		"measure",
		"-s", filepath.Join(afterBase, "stats.json"),
		"-d", filepath.Join(afterBase, "browser"),
		"-b", baselinePath,
		"--max-initial-delta", "0B",
	}

	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code == 0 {
		t.Fatal("expected failure on regression limit breached")
	}
}

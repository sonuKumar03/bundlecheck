package baseline_test

import (
	"path/filepath"
	"testing"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/baseline"
	"bundlecheck/internal/snapshot"
)

func TestBaselineSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "sub", "baseline.json")

	initial := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.1.0",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 1024,
			LazyJS:    2048,
			TotalJS:   3072,
		},
		Packages: []snapshot.Package{
			{
				Name:         "lodash",
				InitialBytes: 512,
				LazyBytes:    0,
				TotalBytes:   512,
			},
		},
	}

	if err := baseline.Save(targetPath, initial); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	loaded, err := baseline.Load(targetPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if loaded.Summary.InitialJS != 1024 || loaded.Summary.TotalJS != 3072 {
		t.Errorf("summary mismatch: %+v", loaded.Summary)
	}
	if len(loaded.Packages) != 1 || loaded.Packages[0].Name != "lodash" {
		t.Errorf("packages mismatch: %+v", loaded.Packages)
	}
}

func TestBaselineLoadNotFound(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "nonexistent.json")

	_, err := baseline.Load(targetPath)
	if err == nil {
		t.Fatal("expected error on missing baseline file")
	}
}

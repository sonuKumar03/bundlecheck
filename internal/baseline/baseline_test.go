package baseline_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/baseline"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
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

func TestBaselineNamedAndLifecycle(t *testing.T) {
	tmp := t.TempDir()

	r1 := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.1.2",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 2000,
			LazyJS:    3000,
			TotalJS:   5000,
		},
		Packages: []snapshot.Package{},
	}

	meta1 := &baseline.Metadata{
		GitRef:    "release/v1.0",
		CommitSHA: "abc1234",
		BuildCmd:  "npm run build",
		CreatedAt: time.Now().UTC(),
	}

	path1, err := baseline.SaveNamed(tmp, "release-v1", r1, meta1)
	if err != nil {
		t.Fatalf("save named: %v", err)
	}
	if path1 == "" {
		t.Fatal("expected non-empty path")
	}

	// List
	list, active, err := baseline.List(tmp)
	if err != nil {
		t.Fatalf("list baselines: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 baseline, got %d", len(list))
	}
	if list[0].Name != "release-v1" || list[0].GitRef != "release/v1.0" {
		t.Errorf("unexpected list entry: %+v", list[0])
	}
	if active != "release-v1" {
		t.Errorf("expected active 'release-v1', got %q", active)
	}

	// Save another
	r2 := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.1.2",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 1500,
			LazyJS:    2500,
			TotalJS:   4000,
		},
		Packages: []snapshot.Package{},
	}
	_, err = baseline.SaveNamed(tmp, "release-v2", r2, &baseline.Metadata{GitRef: "release/v2.0"})
	if err != nil {
		t.Fatalf("save release-v2: %v", err)
	}

	// Switch active
	if err := baseline.SetActive(tmp, "release-v1"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	curActive, _ := baseline.GetActive(tmp)
	if curActive != "release-v1" {
		t.Errorf("expected curActive 'release-v1', got %q", curActive)
	}

	// Delete release-v2
	if err := baseline.Delete(tmp, "release-v2"); err != nil {
		t.Fatalf("delete release-v2: %v", err)
	}

	listAfter, _, _ := baseline.List(tmp)
	if len(listAfter) != 1 {
		t.Fatalf("expected 1 baseline after delete, got %d", len(listAfter))
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

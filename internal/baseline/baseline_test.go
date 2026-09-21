package baseline_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestBaselineMetadataEntry(t *testing.T) {
	tmp := t.TempDir()

	// 1. Existing / legacy baseline JSON without "entry" in metadata loads cleanly
	legacyJSON := `{
  "schemaVersion": "1",
  "toolVersion": "0.4.2",
  "command": "summary",
  "summary": {
    "initialJs": 1000,
    "lazyJs": 2000,
    "totalJs": 3000
  },
  "packages": [],
  "metadata": {
    "name": "legacy",
    "gitRef": "main"
  }
}`
	legacyPath := filepath.Join(tmp, "legacy.json")
	if err := os.WriteFile(legacyPath, []byte(legacyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	loadedLegacy, err := baseline.LoadSnapshot(legacyPath)
	if err != nil {
		t.Fatalf("load legacy baseline snapshot: %v", err)
	}
	if loadedLegacy.Metadata == nil {
		t.Fatal("expected metadata not nil")
	}
	if loadedLegacy.Metadata.Entry != "" {
		t.Errorf("expected empty Entry for legacy baseline, got %q", loadedLegacy.Metadata.Entry)
	}

	// 2. Saving with non-empty Entry round-trips and appears in JSON
	res := &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.4.2",
		Command:       "summary",
		Summary: snapshot.Totals{
			InitialJS: 1000,
			LazyJS:    2000,
			TotalJS:   3000,
		},
		Packages: []snapshot.Package{},
	}
	metaWithEntry := &baseline.Metadata{
		Name:   "worker-entry",
		GitRef: "main",
		Entry:  "src/worker.ts",
	}
	withEntryPath := filepath.Join(tmp, "worker.json")
	if err := baseline.SaveWithMetadata(withEntryPath, res, metaWithEntry); err != nil {
		t.Fatalf("save with entry: %v", err)
	}

	dataWithEntry, err := os.ReadFile(withEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dataWithEntry), `"entry": "src/worker.ts"`) {
		t.Errorf("expected JSON to contain '\"entry\": \"src/worker.ts\"', got:\n%s", string(dataWithEntry))
	}

	loadedWorker, err := baseline.LoadSnapshot(withEntryPath)
	if err != nil {
		t.Fatalf("load worker baseline: %v", err)
	}
	if loadedWorker.Metadata == nil || loadedWorker.Metadata.Entry != "src/worker.ts" {
		t.Errorf("expected Entry 'src/worker.ts', got %+v", loadedWorker.Metadata)
	}

	// 3. Saving with empty Entry omits "entry" from JSON
	metaEmptyEntry := &baseline.Metadata{
		Name:   "no-entry",
		GitRef: "main",
		Entry:  "",
	}
	emptyEntryPath := filepath.Join(tmp, "no-entry.json")
	if err := baseline.SaveWithMetadata(emptyEntryPath, res, metaEmptyEntry); err != nil {
		t.Fatalf("save with empty entry: %v", err)
	}

	dataEmptyEntry, err := os.ReadFile(emptyEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dataEmptyEntry), `"entry"`) {
		t.Errorf("expected JSON to omit 'entry', but found it in:\n%s", string(dataEmptyEntry))
	}
}

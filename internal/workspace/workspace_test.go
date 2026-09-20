package workspace

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestFreshnessReportsNewerBundleInput(t *testing.T) {
	root := t.TempDir()
	stats := filepath.Join(root, "dist/apps/shop/stats.json")
	source := filepath.Join(root, "apps/shop/src/main.ts")
	for _, path := range []string{stats, source} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	artifactTime := time.Date(2026, 9, 20, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(stats, artifactTime, artifactTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, artifactTime.Add(time.Minute), artifactTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	got := CheckFreshness(root, "apps/shop", stats, &snapshot.BundleSnapshot{
		Inputs: []snapshot.Module{{Path: "apps/shop/src/main.ts"}},
	})
	if got.Status != "stale-suspected" || got.NewestInput != "apps/shop/src/main.ts" || got.ArtifactModifiedAt != "2026-09-20T10:30:00Z" || got.NewestInputModifiedAt != "2026-09-20T10:31:00Z" {
		t.Fatalf("freshness: %+v", got)
	}
}

func TestNxOutputsAndOwnership(t *testing.T) {
	root := t.TempDir()
	var metadata Metadata
	if err := json.Unmarshal([]byte(`{"graph":{"nodes":{"shop":{"type":"app","data":{"root":"apps/shop","projectType":"application","targets":{"build":{"executor":"@nx/angular:application","options":{"outputPath":"dist/apps/shop"},"configurations":{"production":{"outputPath":{"base":"out/shop","browser":"web"}}}}}}},"ui":{"type":"lib","data":{"root":"libs/ui"}},"nested":{"type":"lib","data":{"root":"libs/ui/nested"}}}}}`), &metadata); err != nil {
		t.Fatal(err)
	}
	project := metadata.Graph.Nodes["shop"]
	base, browser, err := OutputDirectories(root, project, "build", "production")
	if err != nil || base != filepath.Join(root, "out/shop") || browser != filepath.Join(base, "web") {
		t.Fatalf("directories: %s %s %v", base, browser, err)
	}
	if _, _, err := OutputDirectories(root, project, "build", "missing"); err == nil {
		t.Fatal("missing configuration accepted")
	}
	for _, dir := range []string{base, browser} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(base, "stats.json"))
	write(filepath.Join(browser, "index.html"))
	stats, dist, err := Artifacts(base, browser)
	if err != nil || stats != filepath.Join(base, "stats.json") || dist != browser {
		t.Fatalf("artifacts: %s %s %v", stats, dist, err)
	}
	write(filepath.Join(browser, "other.stats.json"))
	if _, _, err := Artifacts(base, browser); err == nil {
		t.Fatal("ambiguous stats accepted")
	}
	s := &snapshot.BundleSnapshot{Outputs: []snapshot.BundleOutput{
		{Path: "main.js", Initial: true, Inputs: []snapshot.Contribution{{Input: `libs\ui\nested\button.ts`, Bytes: 20}, {Input: "libs/ui/nested/button.ts", Bytes: 20}, {Input: "libs/ui/theme.ts", Bytes: 10}, {Input: "libs/ui-other/no.ts", Bytes: 80}, {Input: "node_modules/x/index.js", Bytes: 90}, {Input: "libs/ui/dead.ts", Bytes: 0}}},
		{Path: "lazy.js", Inputs: []snapshot.Contribution{{Input: filepath.Join(root, "libs/ui/theme.ts"), Bytes: 5}}},
	}}
	libraries, err := Libraries(root, metadata.Graph.Nodes, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 2 || libraries[0].Name != "nested" || libraries[0].InitialBytes != 20 || libraries[1].Name != "ui" || libraries[1].LazyBytes != 5 {
		t.Fatalf("libraries: %+v", libraries)
	}
}

func TestMatricesKeepMissingAppsUnavailable(t *testing.T) {
	r := NewResult("/workspace", "build", "production")
	r.Apps = []App{{Name: "a", Status: "analyzed", Analysis: &analysis.AnalysisResult{}}, {Name: "b", Status: "analyzed", Analysis: &analysis.AnalysisResult{}}, {Name: "c", Status: "missing-artifacts"}}
	a := []snapshot.Package{{Name: "x", InitialBytes: 10, TotalBytes: 10}, {Name: "lazy", LazyBytes: 2, TotalBytes: 2}}
	b := []snapshot.Package{{Name: "x", InitialBytes: 30, TotalBytes: 30}}
	matrix, err := Matrix(r.Apps, map[string][]snapshot.Package{"a": a, "b": b})
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix) != 2 || matrix[0].Name != "x" || matrix[0].InitialBytes != 40 || matrix[0].Apps["c"] != nil || matrix[1].Apps["b"] == nil || matrix[1].Apps["b"].TotalBytes != 0 {
		t.Fatalf("matrix: %+v", matrix)
	}
	r.Packages = matrix
	r.Findings = Findings(r.Packages, nil)
	if len(r.Findings) != 1 || len(r.Findings[0].Apps) != 2 {
		t.Fatalf("findings: %+v", r.Findings)
	}
}

func TestRootAndMetadataFailures(t *testing.T) {
	root := t.TempDir()
	if _, err := Root(root, true); err == nil {
		t.Fatal("missing nx.json accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "nx.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "apps/shop")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if got, err := Root(nested, false); err != nil || got != root {
		t.Fatalf("root %s %v", got, err)
	}
	if _, err := ReadMetadata(context.Background(), root); err == nil {
		t.Fatal("missing Nx accepted")
	}
}

func TestLibraryAppBoundaryAndOverflow(t *testing.T) {
	var metadata Metadata
	if err := json.Unmarshal([]byte(`{"graph":{"nodes":{"lib":{"type":"lib","data":{"root":"libs/shared"}},"app":{"type":"app","data":{"root":"libs/shared/demo"}}}}}`), &metadata); err != nil {
		t.Fatal(err)
	}
	s := &snapshot.BundleSnapshot{Outputs: []snapshot.BundleOutput{{Path: "main.js", Initial: true, Inputs: []snapshot.Contribution{{Input: "libs/shared/demo/app.ts", Bytes: 100}, {Input: "libs/shared/index.ts", Bytes: 20}}}}}
	values, err := Libraries("/workspace", metadata.Graph.Nodes, s)
	if err != nil || len(values) != 1 || values[0].InitialBytes != 20 {
		t.Fatalf("app boundary: %+v %v", values, err)
	}
	apps := []App{{Name: "a", Status: "analyzed"}, {Name: "b", Status: "analyzed"}}
	if _, err := Matrix(apps, map[string][]snapshot.Package{"a": {{Name: "x", InitialBytes: math.MaxInt64}}, "b": {{Name: "x", InitialBytes: 1}}}); err == nil {
		t.Fatal("overflow accepted")
	}
	s.Outputs[0].Inputs = []snapshot.Contribution{{Input: `C:\repo\libs\shared\index.ts`, Bytes: 20}}
	values, err = Libraries(`C:\repo`, metadata.Graph.Nodes, s)
	if err != nil || len(values) != 1 || values[0].InitialBytes != 20 {
		t.Fatalf("Windows absolute path: %+v %v", values, err)
	}
}

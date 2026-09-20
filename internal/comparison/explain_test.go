package comparison_test

import (
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/comparison"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestGenerateFindings_Reconciliation(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 20 * 1024, InitialBytes: 20 * 1024},
		},
	}
	after := &analysis.AnalysisResult{
		// TotalJS grew by 42 KB
		Summary: snapshot.Totals{InitialJS: 142 * 1024, TotalJS: 142 * 1024},
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 20 * 1024, InitialBytes: 20 * 1024},
			// chart.js added 31 KB
			{Name: "chart.js", TotalBytes: 31 * 1024, InitialBytes: 31 * 1024},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (chart.js and unattributed), got %d: %+v", len(findings), findings)
	}

	// 1. First finding: chart.js (31 KB)
	if findings[0].Name != "chart.js" || findings[0].DeltaBytes != 31*1024 {
		t.Errorf("expected chart.js with 31744 bytes, got %+v", findings[0])
	}
	if findings[0].Kind != "package" {
		t.Errorf("expected kind 'package', got %s", findings[0].Kind)
	}

	// 2. Second finding: (unattributed) (11 KB)
	if findings[1].Name != "(unattributed)" || findings[1].DeltaBytes != 11*1024 {
		t.Errorf("expected (unattributed) with 11264 bytes, got %+v", findings[1])
	}
	if findings[1].Kind != "unattributed" {
		t.Errorf("expected kind 'unattributed', got %s", findings[1].Kind)
	}

	// 3. Reconciliation check: sum(attributed) + sum(unattributed) == total delta
	var sum int64
	for _, f := range findings {
		sum += f.DeltaBytes
	}
	expectedTotal := int64(42 * 1024)
	if sum != expectedTotal {
		t.Errorf("reconciliation failed: sum %d != expected total %d", sum, expectedTotal)
	}
}

func TestGenerateFindings_WithSnapshot(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 2500, TotalJS: 2500},
		Packages: []snapshot.Package{
			{Name: "lodash", TotalBytes: 1500, InitialBytes: 1500},
		},
	}

	snap := &snapshot.BundleSnapshot{
		Inputs: []snapshot.Module{
			{
				Path: "src/main.ts",
				Imports: []snapshot.Import{
					{Path: "node_modules/lodash/index.js"},
				},
			},
			{
				Path: "node_modules/lodash/index.js",
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "dist/browser/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "node_modules/lodash/index.js", Bytes: 1500},
				},
			},
		},
	}

	res := comparison.CompareWithSnapshot(before, after, snap)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(res.Findings))
	}

	finding := res.Findings[0]
	if finding.Name != "lodash" {
		t.Errorf("expected finding lodash, got %s", finding.Name)
	}
	if len(finding.Chunks) != 1 || finding.Chunks[0] != "dist/browser/main.js" {
		t.Errorf("expected chunk dist/browser/main.js, got %v", finding.Chunks)
	}
	if len(finding.TracePath) != 2 {
		t.Errorf("expected 2 trace elements, got %v", finding.TracePath)
	} else if finding.TracePath[0] != "src/main.ts" || finding.TracePath[1] != "node_modules/lodash/index.js" {
		t.Errorf("unexpected trace path: %v", finding.TracePath)
	}
}

func TestGenerateFindings_EqualByteTies(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 2000, TotalJS: 2000},
		Packages: []snapshot.Package{
			{Name: "zebra", TotalBytes: 500, InitialBytes: 500},
			{Name: "alpha", TotalBytes: 500, InitialBytes: 500},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	// "alpha" should come before "zebra" deterministically due to alphabetical tie-breaking
	if findings[0].Name != "alpha" || findings[1].Name != "zebra" {
		t.Errorf("expected [alpha, zebra], got [%s, %s]", findings[0].Name, findings[1].Name)
	}
}

func TestGenerateFindings_NoRegression(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 2000, TotalJS: 2000},
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 2000, InitialBytes: 2000},
		},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 1500, TotalJS: 1500}, // Reduced!
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 1500, InitialBytes: 1500},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings on bundle reduction, got %d", len(findings))
	}
}

func TestGenerateFindings_ApplicationSourceGrowth(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 50 * 1024, TotalJS: 50 * 1024},
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 20 * 1024, InitialBytes: 20 * 1024},
		},
	}
	after := &analysis.AnalysisResult{
		// App code grew by 15 KB, rxjs unchanged
		Summary: snapshot.Totals{InitialJS: 65 * 1024, TotalJS: 65 * 1024},
		Packages: []snapshot.Package{
			{Name: "rxjs", TotalBytes: 20 * 1024, InitialBytes: 20 * 1024},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unattributed app growth, got %d", len(findings))
	}
	if findings[0].Name != "(unattributed)" {
		t.Errorf("expected (unattributed), got %s", findings[0].Name)
	}
	if findings[0].DeltaBytes != 15*1024 {
		t.Errorf("expected 15360 bytes, got %d", findings[0].DeltaBytes)
	}
	if findings[0].Kind != "unattributed" {
		t.Errorf("expected kind 'unattributed', got %s", findings[0].Kind)
	}
}

func TestGenerateFindings_RemovedPackages(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Packages: []snapshot.Package{
			{Name: "old-lib", TotalBytes: 20 * 1024, InitialBytes: 20 * 1024},
		},
	}
	after := &analysis.AnalysisResult{
		// old-lib was removed (-20 KB), new-lib added (+30 KB), net delta = +10 KB
		Summary: snapshot.Totals{InitialJS: 110 * 1024, TotalJS: 110 * 1024},
		Packages: []snapshot.Package{
			{Name: "new-lib", TotalBytes: 30 * 1024, InitialBytes: 30 * 1024},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (new-lib), got %d: %+v", len(findings), findings)
	}
	if findings[0].Name != "new-lib" || findings[0].DeltaBytes != 30*1024 {
		t.Errorf("expected new-lib with 30720 bytes, got %+v", findings[0])
	}
}

func TestGenerateFindings_SplitAndRenamedChunks(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 3000, TotalJS: 3000},
		Packages: []snapshot.Package{
			{Name: "shared-pkg", TotalBytes: 2000, InitialBytes: 2000},
		},
	}

	// Package appears in multiple split chunks: chunk-b.js and chunk-a.js
	snap := &snapshot.BundleSnapshot{
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "dist/chunk-b.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "node_modules/shared-pkg/b.js", Bytes: 1000},
				},
			},
			{
				Path:    "dist/chunk-a.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "node_modules/shared-pkg/a.js", Bytes: 1000},
				},
			},
		},
	}

	res := comparison.CompareWithSnapshot(before, after, snap)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(res.Findings))
	}
	chunks := res.Findings[0].Chunks
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %v", chunks)
	}
	// Verify deterministic sorting of chunks: chunk-a.js before chunk-b.js
	if chunks[0] != "dist/chunk-a.js" || chunks[1] != "dist/chunk-b.js" {
		t.Errorf("chunks not deterministically sorted: %v", chunks)
	}
}

func TestGenerateFindings_MissingGraphData(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
	}
	after := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 2000, TotalJS: 2000},
		Packages: []snapshot.Package{
			{Name: "pkg-no-graph", TotalBytes: 1000, InitialBytes: 1000},
		},
	}

	// Snapshot with no outputs and no inputs (missing graph data)
	snap := &snapshot.BundleSnapshot{}

	res := comparison.CompareWithSnapshot(before, after, snap)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(res.Findings))
	}
	if res.Findings[0].Name != "pkg-no-graph" {
		t.Errorf("expected pkg-no-graph, got %s", res.Findings[0].Name)
	}
	if len(res.Findings[0].Chunks) != 0 || len(res.Findings[0].TracePath) != 0 {
		t.Errorf("expected empty chunks and trace path, got chunks=%v, trace=%v", res.Findings[0].Chunks, res.Findings[0].TracePath)
	}
}

func TestGenerateFindings_AttributedSources(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 100000, TotalJS: 100000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "projects/movies/src/app/pages/movie-detail-page", InitialBytes: 0, LazyBytes: 20000, TotalBytes: 20000},
		},
	}
	after := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 120000, TotalJS: 100000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "projects/movies/src/app/pages/movie-detail-page", InitialBytes: 20000, LazyBytes: 0, TotalBytes: 20000},
		},
	}
	snap := &snapshot.BundleSnapshot{
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "main.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "projects/movies/src/app/pages/movie-detail-page/movie-detail-page.component.ts", Bytes: 20000},
				},
			},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, snap)

	if len(findings) == 0 {
		t.Fatalf("expected findings, got 0")
	}

	var sourceFinding *comparison.Finding
	for i := range findings {
		if findings[i].Kind == "source" {
			sourceFinding = &findings[i]
			break
		}
	}
	if sourceFinding == nil {
		t.Fatalf("expected finding with Kind='source', got: %+v", findings)
	}
	if !strings.Contains(sourceFinding.Name, "movie-detail-page") {
		t.Errorf("expected movie-detail-page in finding name, got %q", sourceFinding.Name)
	}
	if sourceFinding.DeltaBytes != 20000 {
		t.Errorf("expected 20000 delta bytes, got %d", sourceFinding.DeltaBytes)
	}
}

func TestGenerateFindings_AttributedSourcesWithGraph(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 10000, TotalJS: 10000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/pages/profile", InitialBytes: 0, LazyBytes: 5000, TotalBytes: 5000},
		},
	}
	after := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 15000, TotalJS: 15000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/pages/profile", InitialBytes: 5000, LazyBytes: 0, TotalBytes: 5000},
		},
	}
	snap := &snapshot.BundleSnapshot{
		Inputs: []snapshot.Module{
			{
				Path: "src/main.ts",
				Imports: []snapshot.Import{
					{Path: "src/app/pages/profile/profile.component.ts"},
				},
			},
			{
				Path: "src/app/pages/profile/profile.component.ts",
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "dist/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 10000},
					{Input: "src/app/pages/profile/profile.component.ts", Bytes: 5000},
				},
			},
		},
	}

	res := comparison.CompareWithSnapshot(before, after, snap)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0]
	if f.Kind != "source" {
		t.Errorf("expected kind 'source', got %q", f.Kind)
	}
	if f.Name != "src/app/pages/profile" {
		t.Errorf("expected name 'src/app/pages/profile', got %q", f.Name)
	}
	if f.DeltaBytes != 5000 {
		t.Errorf("expected 5000 delta bytes, got %d", f.DeltaBytes)
	}
	if len(f.Chunks) != 1 || f.Chunks[0] != "dist/main.js" {
		t.Errorf("expected chunks [dist/main.js], got %v", f.Chunks)
	}
	if len(f.TracePath) != 2 || f.TracePath[0] != "src/main.ts" || f.TracePath[1] != "src/app/pages/profile/profile.component.ts" {
		t.Errorf("expected trace path [src/main.ts, src/app/pages/profile/profile.component.ts], got %v", f.TracePath)
	}
	if f.Reason != "Application component moved into initial bundle" {
		t.Errorf("expected Reason 'Application component moved into initial bundle', got %q", f.Reason)
	}
}

func TestGenerateFindings_SourcesReconciliationMixed(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: snapshot.Totals{InitialJS: 100000, TotalJS: 100000},
		Packages: []snapshot.Package{
			{Name: "rxjs", InitialBytes: 20000, TotalBytes: 20000},
		},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/feature-a", InitialBytes: 10000, TotalBytes: 10000},
		},
	}
	after := &analysis.AnalysisResult{
		// Total grew by 50000
		Summary: snapshot.Totals{InitialJS: 150000, TotalJS: 150000},
		Packages: []snapshot.Package{
			{Name: "rxjs", InitialBytes: 20000, TotalBytes: 20000},
			// Package grew by 20000
			{Name: "chart.js", InitialBytes: 20000, TotalBytes: 20000},
		},
		Sources: []analysis.SourceContribution{
			// Source grew by 15000
			{Name: "src/app/feature-a", InitialBytes: 25000, TotalBytes: 25000},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (chart.js, feature-a, unattributed), got %d: %+v", len(findings), findings)
	}

	// 1. First finding: chart.js (20000)
	if findings[0].Name != "chart.js" || findings[0].DeltaBytes != 20000 || findings[0].Kind != "package" {
		t.Errorf("expected chart.js 20000 package, got %+v", findings[0])
	}
	// 2. Second finding: src/app/feature-a (15000)
	if findings[1].Name != "src/app/feature-a" || findings[1].DeltaBytes != 15000 || findings[1].Kind != "source" {
		t.Errorf("expected feature-a 15000 source, got %+v", findings[1])
	}
	// 3. Third finding: (unattributed) (15000)
	if findings[2].Name != "(unattributed)" || findings[2].DeltaBytes != 15000 || findings[2].Kind != "unattributed" {
		t.Errorf("expected (unattributed) 15000, got %+v", findings[2])
	}

	// Reconciliation invariant: sum(findings) == 50000
	var sum int64
	for _, f := range findings {
		sum += f.DeltaBytes
	}
	if sum != 50000 {
		t.Errorf("reconciliation failed: sum %d != 50000", sum)
	}
}

func TestGenerateFindings_SourceEqualByteTies(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
		Sources:  []analysis.SourceContribution{},
	}
	after := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 2000, TotalJS: 2000},
		Packages: []snapshot.Package{},
		Sources: []analysis.SourceContribution{
			{Name: "src/app/zebra", InitialBytes: 500, TotalBytes: 500},
			{Name: "src/app/alpha", InitialBytes: 500, TotalBytes: 500},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, nil)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
	if findings[0].Name != "src/app/alpha" || findings[1].Name != "src/app/zebra" {
		t.Errorf("expected [src/app/alpha, src/app/zebra], got [%s, %s]", findings[0].Name, findings[1].Name)
	}
}

func TestGenerateFindings_LegacyBaselineSourceFallback(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 1000, TotalJS: 1000},
		Packages: []snapshot.Package{},
		// Sources is nil (legacy baseline v0.4.1)
	}
	after := &analysis.AnalysisResult{
		Summary:  snapshot.Totals{InitialJS: 2500, TotalJS: 2500},
		Packages: []snapshot.Package{},
		// Sources is nil (e.g. not populated or legacy)
	}
	snap := &snapshot.BundleSnapshot{
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "dist/main.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "src/app/app.component.ts", Bytes: 1500},
				},
			},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from snap fallback, got %d: %+v", len(findings), findings)
	}
	if findings[0].Kind != "source" {
		t.Errorf("expected kind 'source', got %q", findings[0].Kind)
	}
	if findings[0].Name != "src/app" {
		t.Errorf("expected name 'src/app', got %q", findings[0].Name)
	}
	if findings[0].DeltaBytes != 1500 {
		t.Errorf("expected 1500 delta bytes, got %d", findings[0].DeltaBytes)
	}
}

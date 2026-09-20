package comparison_test

import (
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

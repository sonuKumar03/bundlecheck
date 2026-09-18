package advisor_test

import (
	"testing"

	"bundlecheck/internal/advisor"
	"bundlecheck/internal/snapshot"
)

func TestAdvisorHeavyUtility(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals: snapshot.Totals{
			InitialJS: 200 * 1024,
			LazyJS:    100 * 1024,
			TotalJS:   300 * 1024,
		},
		Inputs: []snapshot.Module{
			{Path: "src/main.ts", Bytes: 5000},
			{Path: "node_modules/pdfjs-dist/index.js", Bytes: 60 * 1024},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				Initial:    true,
				EntryPoint: "src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 5000},
					{Input: "node_modules/pdfjs-dist/index.js", Bytes: 60 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{
				Name:         "pdfjs-dist",
				InitialBytes: 60 * 1024,
				TotalBytes:   60 * 1024,
			},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})

	if len(res.Suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion for heavy package")
	}

	found := false
	for _, s := range res.Suggestions {
		if s.Target == "pdfjs-dist" && s.Severity == "HIGH" {
			found = true
			if s.Savings != 60*1024 {
				t.Errorf("expected savings 60KB, got %d", s.Savings)
			}
		}
	}
	if !found {
		t.Error("expected HIGH severity suggestion for pdfjs-dist")
	}
}

func TestAdvisorEagerComponent(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Outputs: []snapshot.BundleOutput{
			{
				Path:    "browser/main.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "src/app/dashboard-page.component.ts", Bytes: 30 * 1024},
				},
			},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})
	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestion for eager page component")
	}
	if res.Suggestions[0].Rule != "eager-feature-component" {
		t.Errorf("expected rule eager-feature-component, got %s", res.Suggestions[0].Rule)
	}
}

func TestDuplicateAdviceUsesEmittedInitialContributions(t *testing.T) {
	for _, tc := range []struct {
		name           string
		initial        bool
		contribution   int64
		wantDuplicates int
		wantSavings    int64
	}{
		{"tree shaken", true, 0, 0, 0},
		{"lazy only", false, 2000, 0, 0},
		{"bundled initial copies", true, 2000, 1, 2000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &snapshot.BundleSnapshot{Inputs: []snapshot.Module{
				{Path: "node_modules/unused/index.js", Bytes: 20000},
				{Path: "node_modules/host/node_modules/unused/index.js", Bytes: 20000},
			}, Outputs: []snapshot.BundleOutput{{Path: "chunk.js", Initial: tc.initial, Inputs: []snapshot.Contribution{
				{Input: "node_modules/unused/index.js", Bytes: tc.contribution},
				{Input: "node_modules/host/node_modules/unused/index.js", Bytes: tc.contribution},
			}}}}
			result := advisor.Analyze(s, advisor.AdvisorOptions{})
			count := 0
			for _, suggestion := range result.Suggestions {
				if suggestion.Rule == "duplicate-package" {
					count++
					if suggestion.Savings != tc.wantSavings {
						t.Errorf("duplicate savings = %d, want %d", suggestion.Savings, tc.wantSavings)
					}
				}
			}
			if count != tc.wantDuplicates {
				t.Errorf("duplicate suggestions = %d, want %d", count, tc.wantDuplicates)
			}
		})
	}
}

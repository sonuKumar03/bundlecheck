package advisor_test

import (
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
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

func BenchmarkAdvisorScaling(b *testing.B) {
	const numPackages = 100
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 5000000, TotalJS: 5000000},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: make([]snapshot.Import, 0, numPackages),
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
				},
			},
		},
	}

	for i := 0; i < numPackages; i++ {
		pkgName := "testpkg" + string(rune('a'+(i%26))) + string(rune('0'+(i/26)))
		snap.Packages = append(snap.Packages, snapshot.Package{
			Name:         pkgName,
			InitialBytes: 20000,
			TotalBytes:   20000,
		})

		entryPath := "node_modules/" + pkgName + "/index.js"
		snap.Inputs[0].Imports = append(snap.Inputs[0].Imports, snapshot.Import{Path: entryPath})

		var lastPath string
		for j := 0; j < 10; j++ {
			modPath := "node_modules/" + pkgName + "/mod" + string(rune('0'+j)) + ".js"
			if j == 0 {
				modPath = entryPath
			}
			snap.Inputs = append(snap.Inputs, snapshot.Module{
				Path:  modPath,
				Bytes: 2000,
			})
			if lastPath != "" {
				snap.Inputs[len(snap.Inputs)-2].Imports = append(snap.Inputs[len(snap.Inputs)-2].Imports, snapshot.Import{Path: modPath})
			}
			lastPath = modPath
			snap.Outputs[0].Inputs = append(snap.Outputs[0].Inputs, snapshot.Contribution{
				Input: modPath,
				Bytes: 2000,
			})
		}
	}

	opts := advisor.AdvisorOptions{MinSavings: 1024}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = advisor.Analyze(snap, opts)
	}
}

func TestAdvisorRootBootstrapSuggestion(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/app.config.ts"}},
			},
			{
				Path:    "src/app/app.config.ts",
				Bytes:   2000,
				Imports: []snapshot.Import{{Path: "node_modules/@angular/animations/fesm2022/animations.mjs"}},
			},
			{
				Path:  "node_modules/@angular/animations/fesm2022/animations.mjs",
				Bytes: 25 * 1024,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				Initial:    true,
				EntryPoint: "src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "src/app/app.config.ts", Bytes: 2000},
					{Input: "node_modules/@angular/animations/fesm2022/animations.mjs", Bytes: 25 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{Name: "@angular/animations", InitialBytes: 25 * 1024, TotalBytes: 25 * 1024},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})
	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestion for root-imported package")
	}

	s := res.Suggestions[0]
	if s.Target != "@angular/animations" {
		t.Errorf("expected target @angular/animations, got %s", s.Target)
	}
	if s.File != "src/app/app.config.ts" {
		t.Errorf("expected file src/app/app.config.ts, got %s", s.File)
	}
	// Verify action is disabled and title calls out root bootstrap
	if s.Action != "" {
		t.Errorf("expected action to be disabled/empty, got: %s", s.Action)
	}
	if !strings.Contains(s.Title, "root bootstrap") {
		t.Errorf("expected title to mention root bootstrap, got: %s", s.Title)
	}
	if !strings.Contains(s.Description, "src/app/app.config.ts") {
		t.Errorf("expected description to mention importer file, got: %s", s.Description)
	}
}

func TestAdvisorRouteComponentSuggestion(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/app.routes.ts"}},
			},
			{
				Path:    "src/app/app.routes.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/pages/movie-detail/movie-detail.component.ts"}},
			},
			{
				Path:    "src/app/pages/movie-detail/movie-detail.component.ts",
				Bytes:   5000,
				Imports: []snapshot.Import{{Path: "node_modules/@push-based/ngx-fast-svg/index.js"}},
			},
			{
				Path:  "node_modules/@push-based/ngx-fast-svg/index.js",
				Bytes: 30 * 1024,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				Initial:    true,
				EntryPoint: "src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "src/app/app.routes.ts", Bytes: 1000},
					{Input: "src/app/pages/movie-detail/movie-detail.component.ts", Bytes: 5000},
					{Input: "node_modules/@push-based/ngx-fast-svg/index.js", Bytes: 30 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{Name: "@push-based/ngx-fast-svg", InitialBytes: 30 * 1024, TotalBytes: 30 * 1024},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})
	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestion for package in route component")
	}

	s := res.Suggestions[0]
	if s.Target != "@push-based/ngx-fast-svg" {
		t.Errorf("expected target @push-based/ngx-fast-svg, got %s", s.Target)
	}
	if s.Action != "" {
		t.Errorf("expected action to be disabled/empty, got: %s", s.Action)
	}
	if !strings.Contains(s.Title, "eager component") {
		t.Errorf("expected title to identify eager component, got: %s", s.Title)
	}
	if !strings.Contains(s.Description, "app.routes.ts") {
		t.Errorf("expected description to mention app.routes.ts, got: %s", s.Description)
	}
}

func TestAdvisorGeneralUtilitySuggestion(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/services/pdf-export.service.ts"}},
			},
			{
				Path:    "src/app/services/pdf-export.service.ts",
				Bytes:   2000,
				Imports: []snapshot.Import{{Path: "node_modules/pdfjs-dist/index.js"}},
			},
			{
				Path:  "node_modules/pdfjs-dist/index.js",
				Bytes: 50 * 1024,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				Initial:    true,
				EntryPoint: "src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "src/app/services/pdf-export.service.ts", Bytes: 2000},
					{Input: "node_modules/pdfjs-dist/index.js", Bytes: 50 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{Name: "pdfjs-dist", InitialBytes: 50 * 1024, TotalBytes: 50 * 1024},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})
	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestion for service-imported package")
	}

	s := res.Suggestions[0]
	if s.Target != "pdfjs-dist" {
		t.Errorf("expected target pdfjs-dist, got %s", s.Target)
	}
	if s.Action != "" {
		t.Errorf("expected action to be disabled/empty, got: %s", s.Action)
	}
	if !strings.Contains(s.Title, "pdf-export.service.ts") {
		t.Errorf("expected title to reference importer file, got: %s", s.Title)
	}
}

func TestAdvisorTransitiveNodeModulesImporter(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals:        snapshot.Totals{InitialJS: 100 * 1024, TotalJS: 100 * 1024},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/app.config.ts"}},
			},
			{
				Path:    "src/app/app.config.ts",
				Bytes:   2000,
				Imports: []snapshot.Import{{Path: "node_modules/intermediate-pkg/index.js"}},
			},
			{
				Path:    "node_modules/intermediate-pkg/index.js",
				Bytes:   5000,
				Imports: []snapshot.Import{{Path: "node_modules/target-pkg/sub.js"}},
			},
			{
				Path:    "node_modules/target-pkg/sub.js",
				Bytes:   10000,
				Imports: []snapshot.Import{{Path: "node_modules/target-pkg/index.js"}},
			},
			{
				Path:  "node_modules/target-pkg/index.js",
				Bytes: 15 * 1024,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				Initial:    true,
				EntryPoint: "src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "src/app/app.config.ts", Bytes: 2000},
					{Input: "node_modules/intermediate-pkg/index.js", Bytes: 5000},
					{Input: "node_modules/target-pkg/sub.js", Bytes: 10000},
					{Input: "node_modules/target-pkg/index.js", Bytes: 15 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{Name: "target-pkg", InitialBytes: 25 * 1024, TotalBytes: 25 * 1024},
		},
	}

	res := advisor.Analyze(snap, advisor.AdvisorOptions{MinSavings: 1024})
	if len(res.Suggestions) == 0 {
		t.Fatal("expected suggestion for target-pkg")
	}

	s := res.Suggestions[0]
	if s.Target != "target-pkg" {
		t.Errorf("expected target target-pkg, got %s", s.Target)
	}
	// The application source file responsible should be src/app/app.config.ts, NOT node_modules/target-pkg/sub.js
	if s.File != "src/app/app.config.ts" {
		t.Errorf("expected application caller src/app/app.config.ts, got %s", s.File)
	}
	if s.Action != "" {
		t.Errorf("expected action to be disabled/empty, got: %s", s.Action)
	}
	if !strings.Contains(s.Title, "root bootstrap") {
		t.Errorf("expected title to classify as root bootstrap, got: %s", s.Title)
	}
}

func TestAnalyzeWithEntry_DistinctImporters(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Totals: snapshot.Totals{
			InitialJS: 200 * 1024,
			LazyJS:    100 * 1024,
			TotalJS:   300 * 1024,
		},
		Inputs: []snapshot.Module{
			{
				Path:    "src/main.ts",
				Bytes:   1000,
				Imports: []snapshot.Import{{Path: "src/app/app.config.ts"}},
			},
			{
				Path:    "src/app/app.config.ts",
				Bytes:   2000,
				Imports: []snapshot.Import{{Path: "node_modules/heavy-pkg/index.js"}},
			},
			{
				Path:    "src/worker.ts",
				Bytes:   800,
				Imports: []snapshot.Import{{Path: "src/worker/compute.service.ts"}},
			},
			{
				Path:    "src/worker/compute.service.ts",
				Bytes:   1500,
				Imports: []snapshot.Import{{Path: "node_modules/heavy-pkg/index.js"}},
			},
			{
				Path:  "node_modules/heavy-pkg/index.js",
				Bytes: 50 * 1024,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 1000},
					{Input: "src/app/app.config.ts", Bytes: 2000},
					{Input: "node_modules/heavy-pkg/index.js", Bytes: 50 * 1024},
				},
			},
			{
				Path:       "browser/worker.js",
				EntryPoint: "src/worker.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/worker.ts", Bytes: 800},
					{Input: "src/worker/compute.service.ts", Bytes: 1500},
					{Input: "node_modules/heavy-pkg/index.js", Bytes: 50 * 1024},
				},
			},
		},
		Packages: []snapshot.Package{
			{
				Name:         "heavy-pkg",
				InitialBytes: 50 * 1024,
				TotalBytes:   50 * 1024,
			},
		},
	}

	opts := advisor.AdvisorOptions{MinSavings: 1024}

	// 1. Analyze with main entry: chain derives from src/main.ts -> src/app/app.config.ts (root bootstrap)
	resMain, err := advisor.AnalyzeWithEntry(snap, opts, "src/main.ts")
	if err != nil {
		t.Fatalf("AnalyzeWithEntry(main) failed: %v", err)
	}
	var suggMain *advisor.Suggestion
	for i := range resMain.Suggestions {
		if resMain.Suggestions[i].Target == "heavy-pkg" {
			suggMain = &resMain.Suggestions[i]
			break
		}
	}
	if suggMain == nil {
		t.Fatal("expected suggestion for heavy-pkg with main entry")
	}
	if suggMain.File != "src/app/app.config.ts" {
		t.Errorf("main entry: expected importer file src/app/app.config.ts, got %s", suggMain.File)
	}
	if !strings.Contains(suggMain.Title, "root bootstrap") {
		t.Errorf("main entry: expected title referencing root bootstrap, got %s", suggMain.Title)
	}

	// 2. Analyze with worker entry: chain derives from src/worker.ts -> src/worker/compute.service.ts
	resWorker, err := advisor.AnalyzeWithEntry(snap, opts, "src/worker.ts")
	if err != nil {
		t.Fatalf("AnalyzeWithEntry(worker) failed: %v", err)
	}
	var suggWorker *advisor.Suggestion
	for i := range resWorker.Suggestions {
		if resWorker.Suggestions[i].Target == "heavy-pkg" {
			suggWorker = &resWorker.Suggestions[i]
			break
		}
	}
	if suggWorker == nil {
		t.Fatal("expected suggestion for heavy-pkg with worker entry")
	}
	if suggWorker.File != "src/worker/compute.service.ts" {
		t.Errorf("worker entry: expected importer file src/worker/compute.service.ts, got %s", suggWorker.File)
	}
	if !strings.Contains(suggWorker.Title, "compute.service.ts") {
		t.Errorf("worker entry: expected title referencing compute.service.ts, got %s", suggWorker.Title)
	}

	// 3. AnalyzeWithEntry with invalid entry returns error
	if _, err := advisor.AnalyzeWithEntry(snap, opts, "missing.js"); err == nil {
		t.Fatal("expected error for unmatched entry selector in AnalyzeWithEntry, got nil")
	}

	// 4. Default Analyze (no entry) continues to work
	resDef := advisor.Analyze(snap, opts)
	if len(resDef.Suggestions) == 0 {
		t.Fatal("expected suggestions from default Analyze")
	}
}




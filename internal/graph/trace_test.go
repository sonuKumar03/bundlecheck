package graph_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/graph"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestTracePackage(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs: []snapshot.Module{
			{
				Path:  "src/main.ts",
				Bytes: 500,
				Imports: []snapshot.Import{
					{Path: "src/app/app.component.ts"},
				},
			},
			{
				Path:  "src/app/app.component.ts",
				Bytes: 1200,
				Imports: []snapshot.Import{
					{Path: "node_modules/lodash/index.js"},
				},
			},
			{
				Path:  "node_modules/lodash/index.js",
				Bytes: 5000,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "dist/browser/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 500},
					{Input: "src/app/app.component.ts", Bytes: 1200},
					{Input: "node_modules/lodash/index.js", Bytes: 5000},
				},
			},
		},
	}

	res, err := graph.TracePackage(snap, "lodash", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Found {
		t.Fatal("expected package to be found")
	}
	if res.PackageName != "lodash" {
		t.Errorf("expected package name lodash, got %s", res.PackageName)
	}
	if res.InitialBytes != 5000 {
		t.Errorf("expected initial bytes 5000, got %d", res.InitialBytes)
	}
	if len(res.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(res.Chains))
	}

	chain := res.Chains[0]
	if !chain.Initial {
		t.Errorf("expected chain to be initial")
	}
	expectedPath := []string{"src/main.ts", "src/app/app.component.ts", "node_modules/lodash/index.js"}
	if len(chain.Path) != 3 {
		t.Fatalf("expected 3 steps, got %d: %+v", len(chain.Path), chain.Path)
	}
	for i, step := range expectedPath {
		if chain.Path[i] != step {
			t.Errorf("step %d: expected %s, got %s", i, step, chain.Path[i])
		}
	}
}

func TestTracePackageNotFound(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs:        []snapshot.Module{{Path: "src/main.ts", Bytes: 100}},
		Outputs:       []snapshot.BundleOutput{},
	}

	res, err := graph.TracePackage(snap, "missing-pkg", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Found {
		t.Error("expected found to be false")
	}
}

func TestTraceDependencyPathPreservesPackageOrder(t *testing.T) {
	for _, s := range []*snapshot.BundleSnapshot{
		{Inputs: []snapshot.Module{
			{Path: "node_modules/abc/index.js"},
			{Path: "node_modules/abd/index.js"},
		}},
		{Outputs: []snapshot.BundleOutput{{Inputs: []snapshot.Contribution{
			{Input: "node_modules/abc/index.js", Bytes: 10},
			{Input: "node_modules/abd/index.js", Bytes: 20},
		}}}},
	} {
		g := graph.NewGraph(s)
		for i := 0; i < 100; i++ {
			result, err := g.TracePackage("node_modules/ab", false, 5)
			if err != nil {
				t.Fatal(err)
			}
			if result.PackageName != "abc" {
				t.Fatalf("package order changed: got %q, want abc", result.PackageName)
			}
		}
	}
}

func TestIndexedTraceHandlesCyclesAndMultipleEntries(t *testing.T) {
	s := &snapshot.BundleSnapshot{
		Inputs: []snapshot.Module{
			{Path: "src/a.ts", Imports: []snapshot.Import{{Path: "src/cycle.ts"}}},
			{Path: "src/cycle.ts", Imports: []snapshot.Import{{Path: "src/a.ts"}, {Path: "node_modules/pkg/index.js"}}},
			{Path: "src/z.ts", Imports: []snapshot.Import{{Path: "node_modules/pkg/index.js"}}},
			{Path: "node_modules/pkg/index.js", Imports: []snapshot.Import{{Path: "src/cycle.ts"}}},
		},
		Outputs: []snapshot.BundleOutput{
			{Path: "main.js", EntryPoint: "src/a.ts", Initial: true, Inputs: []snapshot.Contribution{{Input: "node_modules/pkg/index.js", Bytes: 10}}},
			{Path: "lazy.js", EntryPoint: "src/z.ts", Inputs: []snapshot.Contribution{{Input: "node_modules/pkg/index.js", Bytes: 20}}},
		},
	}
	g := graph.NewGraph(s)
	for _, initialOnly := range []bool{false, true} {
		result, err := g.TracePackage("pkg", initialOnly, 5)
		if err != nil {
			t.Fatal(err)
		}
		wantChains := 2
		if initialOnly {
			wantChains = 1
		}
		if result.InitialBytes != 10 || result.LazyBytes != 20 || len(result.Chains) != wantChains {
			t.Fatalf("unexpected trace totals or chains: %+v", result)
		}
		for _, chain := range result.Chains {
			if chain.Initial {
				wantPath := []string{"src/a.ts", "src/cycle.ts", "node_modules/pkg/index.js"}
				if !reflect.DeepEqual(chain.Path, wantPath) {
					t.Fatalf("expected initial chain from src/a.ts: got %+v, want %+v", chain.Path, wantPath)
				}
			} else {
				wantPath := []string{"src/z.ts", "node_modules/pkg/index.js"}
				if !reflect.DeepEqual(chain.Path, wantPath) {
					t.Fatalf("expected lazy chain from src/z.ts: got %+v, want %+v", chain.Path, wantPath)
				}
			}
		}
	}
}

func TestTraceExactDependencyFileAndPackageBoundaries(t *testing.T) {
	s := &snapshot.BundleSnapshot{
		Inputs: []snapshot.Module{
			{Path: "src/main.ts", Imports: []snapshot.Import{{Path: "node_modules/lodash/index.js"}, {Path: "node_modules/lodash-es/index.js"}}},
			{Path: "node_modules/lodash/index.js"},
			{Path: "node_modules/lodash-es/index.js"},
		},
		Outputs: []snapshot.BundleOutput{{Path: "main.js", EntryPoint: "src/main.ts", Initial: true, Inputs: []snapshot.Contribution{
			{Input: "node_modules/lodash/index.js", Bytes: 2000},
			{Input: "node_modules/lodash-es/index.js", Bytes: 3000},
		}}},
	}
	for _, target := range []string{"lodash", "node_modules/lodash/index.js"} {
		t.Run(target, func(t *testing.T) {
			result, err := graph.TracePackage(s, target, false, 5)
			if err != nil || !result.Found || result.InitialBytes != 2000 || len(result.Chains) != 1 {
				t.Fatalf("wrong target attribution: result=%+v err=%v", result, err)
			}
		})
	}
}

func TestTracePackageWithEntry_MainAndWorkerEntries(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs: []snapshot.Module{
			{
				Path:  "src/main.ts",
				Bytes: 500,
				Imports: []snapshot.Import{
					{Path: "src/app/app.component.ts"},
				},
			},
			{
				Path:  "src/app/app.component.ts",
				Bytes: 1200,
				Imports: []snapshot.Import{
					{Path: "node_modules/main-pkg/index.js"},
				},
			},
			{
				Path:  "node_modules/main-pkg/index.js",
				Bytes: 3000,
			},
			{
				Path:  "src/worker.ts",
				Bytes: 400,
				Imports: []snapshot.Import{
					{Path: "node_modules/worker-pkg/index.js"},
				},
			},
			{
				Path:  "node_modules/worker-pkg/index.js",
				Bytes: 4000,
			},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				EntryPoint: "src/main.ts",
				Initial:    true,
				Inputs: []snapshot.Contribution{
					{Input: "src/main.ts", Bytes: 500},
					{Input: "src/app/app.component.ts", Bytes: 1200},
					{Input: "node_modules/main-pkg/index.js", Bytes: 3000},
				},
			},
			{
				Path:       "browser/worker.js",
				EntryPoint: "src/worker.ts",
				Initial:    false,
				Inputs: []snapshot.Contribution{
					{Input: "src/worker.ts", Bytes: 400},
					{Input: "node_modules/worker-pkg/index.js", Bytes: 4000},
				},
			},
		},
		Packages: []snapshot.Package{
			{Name: "main-pkg", TotalBytes: 3000},
			{Name: "worker-pkg", TotalBytes: 4000},
		},
	}

	// 1. Selecting worker by source: chains root at src/worker.ts
	resWorkerSource, err := graph.TracePackageWithEntry(snap, "worker-pkg", "src/worker.ts", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resWorkerSource.Found {
		t.Fatalf("expected worker-pkg to be found")
	}
	if len(resWorkerSource.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(resWorkerSource.Chains))
	}
	if len(resWorkerSource.Chains[0].Path) == 0 || resWorkerSource.Chains[0].Path[0] != "src/worker.ts" {
		t.Fatalf("expected chain to root at src/worker.ts, got: %v", resWorkerSource.Chains[0].Path)
	}

	// 2. Selecting worker by emitted glob: chains root at src/worker.ts
	resWorkerGlob, err := graph.TracePackageWithEntry(snap, "worker-pkg", "*worker.js", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resWorkerGlob.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(resWorkerGlob.Chains))
	}
	if len(resWorkerGlob.Chains[0].Path) == 0 || resWorkerGlob.Chains[0].Path[0] != "src/worker.ts" {
		t.Fatalf("expected chain to root at src/worker.ts, got: %v", resWorkerGlob.Chains[0].Path)
	}

	// 3. Selecting main: cannot return the worker-only chain
	resMainTracingWorker, err := graph.TracePackageWithEntry(snap, "worker-pkg", "src/main.ts", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resMainTracingWorker.Chains) != 0 {
		t.Fatalf("selecting main must not return worker-only chain, got: %+v", resMainTracingWorker.Chains)
	}

	// 4. Selecting main: returns main-only chain rooted at src/main.ts
	resMainTracingMain, err := graph.TracePackageWithEntry(snap, "main-pkg", "src/main.ts", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resMainTracingMain.Chains) != 1 {
		t.Fatalf("expected 1 chain for main-pkg, got %d", len(resMainTracingMain.Chains))
	}
	if len(resMainTracingMain.Chains[0].Path) == 0 || resMainTracingMain.Chains[0].Path[0] != "src/main.ts" {
		t.Fatalf("expected chain to root at src/main.ts, got: %v", resMainTracingMain.Chains[0].Path)
	}

	// 5. Selecting worker: cannot return the main-only chain
	resWorkerTracingMain, err := graph.TracePackageWithEntry(snap, "main-pkg", "src/worker.ts", false, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resWorkerTracingMain.Chains) != 0 {
		t.Fatalf("selecting worker must not return main-only chain, got: %+v", resWorkerTracingMain.Chains)
	}
}

func TestNewGraphWithEntry_Cases(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs: []snapshot.Module{
			{Path: "src/main.ts", Bytes: 500},
			{Path: "src/worker.ts", Bytes: 400},
			{Path: "src/chunk.ts", Bytes: 300},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "browser/main.js",
				EntryPoint: "src/main.ts",
			},
			{
				Path:       "browser/worker.js",
				EntryPoint: "src/worker.ts",
			},
			{
				Path:       "browser/chunk.js",
				EntryPoint: "",
			},
		},
	}

	t.Run("multiple-match", func(t *testing.T) {
		multiSnap := &snapshot.BundleSnapshot{
			Outputs: []snapshot.BundleOutput{
				{Path: "browser/z-main.js", EntryPoint: "src/z-main.ts"},
				{Path: "browser/a-main.js", EntryPoint: "src/a-main.ts"},
			},
		}
		g, err := graph.NewGraphWithEntry(multiSnap, "*-main.js")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantRoots := []string{"src/a-main.ts", "src/z-main.ts"}
		if !reflect.DeepEqual(g.Roots, wantRoots) {
			t.Fatalf("got roots %v, want %v", g.Roots, wantRoots)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		_, err := graph.NewGraphWithEntry(snap, "[")
		if err == nil || !strings.Contains(err.Error(), "[") {
			t.Fatalf("expected error containing '[', got %v", err)
		}
	})

	t.Run("unmatched", func(t *testing.T) {
		_, err := graph.NewGraphWithEntry(snap, "missing.js")
		if err == nil || !strings.Contains(err.Error(), "missing.js") {
			t.Fatalf("expected error containing 'missing.js', got %v", err)
		}
	})

	t.Run("output-without-entrypoint", func(t *testing.T) {
		_, err := graph.NewGraphWithEntry(snap, "browser/chunk.js")
		if err == nil {
			t.Fatal("expected error for output without entrypoint, got nil")
		}
		wantMsg := `matched output "browser/chunk.js" has no source entryPoint; specify source entry path`
		if !strings.Contains(err.Error(), wantMsg) {
			t.Fatalf("expected error containing %q, got %v", wantMsg, err)
		}
	})

	t.Run("default-trace regression", func(t *testing.T) {
		g, err := graph.NewGraphWithEntry(snap, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantRoots := []string{"src/main.ts", "src/worker.ts"}
		if !reflect.DeepEqual(g.Roots, wantRoots) {
			t.Fatalf("got roots %v, want %v", g.Roots, wantRoots)
		}

		gDef := graph.NewGraph(snap)
		if !reflect.DeepEqual(gDef.Roots, wantRoots) {
			t.Fatalf("got roots %v, want %v", gDef.Roots, wantRoots)
		}
	})

	t.Run("nil snapshot", func(t *testing.T) {
		g, err := graph.NewGraphWithEntry(nil, "")
		if err != nil || g != nil {
			t.Fatalf("expected nil, nil for nil snapshot with empty entry, got %v, %v", g, err)
		}
		_, err = graph.NewGraphWithEntry(nil, "src/main.ts")
		if err == nil {
			t.Fatalf("expected error for nil snapshot with non-empty entry, got nil")
		}
		_, err = graph.TracePackageWithEntry(nil, "pkg", "src/main.ts", false, 5)
		if err == nil {
			t.Fatalf("expected error for nil snapshot in TracePackageWithEntry, got nil")
		}
	})
}

func TestGraph_NoNodeModulesRoots_AndApplicationIngress(t *testing.T) {
	snap := &snapshot.BundleSnapshot{
		SchemaVersion: "1",
		Inputs: []snapshot.Module{
			{Path: "apps/portal/src/main.ts", Bytes: 100, Imports: []snapshot.Import{
				{Path: "apps/portal/src/app/app.module.ts"},
			}},
			{Path: "apps/portal/src/app/app.module.ts", Bytes: 200, Imports: []snapshot.Import{
				{Path: "libs/timezone-scheduler/src/index.ts"},
			}},
			{Path: "libs/timezone-scheduler/src/index.ts", Bytes: 150, Imports: []snapshot.Import{
				{Path: "node_modules/moment-timezone/index.js"},
			}},
			{Path: "node_modules/moment-timezone/index.js", Bytes: 500, Imports: []snapshot.Import{
				{Path: "node_modules/moment-timezone/data/packed/latest.json"},
			}},
			{Path: "node_modules/moment-timezone/data/packed/latest.json", Bytes: 10000},
		},
		Outputs: []snapshot.BundleOutput{
			{
				Path:       "main.js",
				Initial:    true,
				EntryPoint: "apps/portal/src/main.ts",
				Inputs: []snapshot.Contribution{
					{Input: "apps/portal/src/main.ts", Bytes: 100},
				},
			},
			{
				Path:    "chunk-initial-vendor.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "node_modules/moment-timezone/index.js", Bytes: 500},
					{Input: "node_modules/moment-timezone/data/packed/latest.json", Bytes: 10000},
					{Input: "apps/portal/src/app/app.module.ts", Bytes: 200},
					{Input: "libs/timezone-scheduler/src/index.ts", Bytes: 150},
				},
			},
			{
				Path:       "chunk-lazy.js",
				Initial:    false,
				EntryPoint: "libs/timezone-scheduler/src/index.ts",
				Inputs: []snapshot.Contribution{
					{Input: "libs/timezone-scheduler/src/index.ts", Bytes: 50},
				},
			},
		},
	}

	g := graph.NewGraph(snap)
	if g == nil {
		t.Fatalf("expected non-nil graph")
	}

	// Verify roots do not contain any node_modules
	for _, r := range g.Roots {
		if strings.Contains(r, "node_modules") {
			t.Errorf("expected no node_modules in roots, got: %s", r)
		}
	}

	// Trace moment-timezone
	res, err := g.TracePackage("moment-timezone", true, 1)
	if err != nil {
		t.Fatalf("TracePackage failed: %v", err)
	}
	if len(res.Chains) == 0 {
		t.Fatalf("expected at least 1 chain, got 0")
	}

	chain := res.Chains[0].Path
	if len(chain) == 0 || chain[0] != "apps/portal/src/main.ts" {
		t.Errorf("expected chain to start with apps/portal/src/main.ts, got: %v", chain)
	}
}


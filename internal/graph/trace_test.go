package graph_test

import (
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
			if len(chain.Path) != 2 || chain.Path[0] != "src/z.ts" || chain.Path[1] != "node_modules/pkg/index.js" {
				t.Fatalf("expected shortest path from src/z.ts: %+v", chain)
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

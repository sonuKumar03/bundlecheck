package graph_test

import (
	"testing"

	"bundlecheck/internal/graph"
	"bundlecheck/internal/snapshot"
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

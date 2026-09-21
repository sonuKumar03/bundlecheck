# Application Ingress Regression Attribution & Entrypoint Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix dependency graph root detection and tracing so regression explanations trace back to the application entrypoint (`apps/.../main.ts` / `app.module.ts`) rather than stopping at internal `node_modules` files or lazy route entrypoints.

**Architecture:** 
1. Disallow `node_modules` from ever qualifying as graph roots in `internal/graph/trace.go`.
2. Differentiate initial application entrypoints from lazy dynamic import entrypoints in `NewGraph` so initial chunk regressions trace from bootstrap roots (`main.ts`).
3. In `internal/comparison/explain.go`, scope initial package regression tracing to initial outputs using `initialOnly: true` and leverage configured entrypoints from `project.json` / `angular.json`.
4. In `internal/report/github_pr.go` and `markdown.go`, format the ingress path to emphasize the application-to-package boundary.

**Tech Stack:** Go 1.22+, esbuild / Angular stats metadata, Cobra CLI, GitHub Actions composite action.

**Spec:** `docs/agents.md` & `SKILL.md` contracts for bundle comparison, regression explanations, and dependency tracing.

## Global Constraints

- Never treat any file containing `node_modules` as an application root.
- Never break backward compatibility with existing baseline JSON files (schema version `1`).
- Keep total reconciliation intact: sum(attributed packages) + sum(attributed sources) + sum(unattributed) == positive regression delta.
- All Go tests (`go test ./...`) must pass with zero race conditions.
- No third-party dependencies; use standard library and existing internal packages.
- Always prefix CLI commands with `rtk` per project rules.

---

### Task 1: Fix Root Detection and Initial vs. Lazy Segregation in `internal/graph/trace.go`

**Files:**
- Modify: `internal/graph/trace.go:133-211`
- Test: `internal/graph/trace_test.go`

**Interfaces:**
- Consumes: `snapshot.BundleSnapshot`, `snapshot.BundleOutput`, `snapshot.Module`
- Produces: `Graph.Roots []string`, `Graph.ShortestPath(target string) []string`, `Graph.TracePackage(target string, initialOnly bool, maxChains int)`

- [ ] **Step 1: Write failing unit test verifying node_modules is never a root and initial roots are prioritized**

In `internal/graph/trace_test.go`:
```go
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

	g := NewGraph(snap)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/graph -run TestGraph_NoNodeModulesRoots_AndApplicationIngress`
Expected: FAIL (chain starts with `node_modules/moment-timezone/index.js` or `libs/timezone-scheduler/src/index.ts`)

- [ ] **Step 3: Update `NewGraphWithEntry` in `internal/graph/trace.go`**

1. Ensure roots gathered from `o.EntryPoint` filter out `node_modules/`:
```go
if o.EntryPoint != "" {
    ep := snapshot.CleanPath(o.EntryPoint)
    if !strings.Contains(ep, "node_modules") {
        roots = append(roots, ep)
    }
}
```
2. When scanning fallback inputs (`o.Inputs`), strictly exclude `node_modules`:
```go
for _, c := range o.Inputs {
    cPath := snapshot.CleanPath(c.Input)
    if strings.Contains(cPath, "node_modules") {
        continue
    }
    if strings.Contains(cPath, "main.") || strings.Contains(cPath, "polyfills.") || strings.HasPrefix(cPath, "src/index.") || strings.HasPrefix(cPath, "apps/") {
        roots = append(roots, cPath)
    }
}
```
3. Separate initial roots (`o.Initial && o.EntryPoint != ""`) so `InitialRoots` can be used when tracing initial output chunks or when `initialOnly == true`. Store `InitialRoots []string` on `Graph`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/graph -run TestGraph_NoNodeModulesRoots_AndApplicationIngress`
Expected: PASS

- [ ] **Step 5: Run all package tests**

Run: `rtk go test ./internal/graph/...`
Expected: PASS

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/graph/trace.go internal/graph/trace_test.go
git commit -m "fix(graph): eliminate node_modules roots and prioritize initial entrypoints"
```

---

### Task 2: Pass Application Entrypoint and Initial Filter into `internal/comparison/explain.go`

**Files:**
- Modify: `internal/comparison/explain.go:101-137`
- Test: `internal/comparison/explain_test.go`

**Interfaces:**
- Consumes: `analysis.AnalysisResult`, `snapshot.BundleSnapshot`, `graph.Graph`
- Produces: `Finding.TracePath []string` containing full application ingress path

- [ ] **Step 1: Write failing unit test in `internal/comparison/explain_test.go`**

Add test checking that an initial package regression has a `TracePath` starting at application source root (`apps/.../main.ts`):
```go
func TestGenerateFindings_InitialPackageRegression_ApplicationTrace(t *testing.T) {
	before := &analysis.AnalysisResult{
		Summary: analysis.SummaryMetrics{InitialJS: 1000, TotalJS: 1000},
		Packages: []analysis.PackageContribution{},
	}
	after := &analysis.AnalysisResult{
		Summary: analysis.SummaryMetrics{InitialJS: 11000, TotalJS: 11000, Delta: analysis.DeltaMetrics{InitialJS: 10000, TotalJS: 10000}},
		Packages: []analysis.PackageContribution{
			{
				Name: "moment-timezone",
				InitialBytes: 10000,
				TotalBytes: 10000,
				Delta: analysis.PackageDelta{InitialBytes: 10000, TotalBytes: 10000},
			},
		},
	}
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
			{Path: "node_modules/moment-timezone/index.js", Bytes: 500},
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
				Path:    "chunk-vendor.js",
				Initial: true,
				Inputs: []snapshot.Contribution{
					{Input: "node_modules/moment-timezone/index.js", Bytes: 500},
					{Input: "apps/portal/src/app/app.module.ts", Bytes: 200},
					{Input: "libs/timezone-scheduler/src/index.ts", Bytes: 150},
				},
			},
		},
	}

	res := comparison.Compare(before, after)
	findings := comparison.GenerateFindings(res, snap)
	if len(findings) == 0 {
		t.Fatalf("expected at least 1 finding, got 0")
	}

	f := findings[0]
	if f.Name != "moment-timezone" {
		t.Fatalf("expected finding name 'moment-timezone', got %s", f.Name)
	}
	if len(f.TracePath) == 0 || f.TracePath[0] != "apps/portal/src/main.ts" {
		t.Errorf("expected TracePath to start with apps/portal/src/main.ts, got: %v", f.TracePath)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/comparison -run TestGenerateFindings_InitialPackageRegression_ApplicationTrace`
Expected: FAIL

- [ ] **Step 3: Update `traceImportPath` in `internal/comparison/explain.go`**

In `internal/comparison/explain.go`:
```go
traceImportPath := func(target string, initialOnly bool) []string {
    if g == nil {
        return nil
    }
    whyRes, err := g.TracePackage(target, initialOnly, 1)
    if err == nil && whyRes != nil && len(whyRes.Chains) > 0 {
        return whyRes.Chains[0].Path
    }
    // Fallback without initialOnly filter if no chain found
    if initialOnly {
        whyRes, err = g.TracePackage(target, false, 1)
        if err == nil && whyRes != nil && len(whyRes.Chains) > 0 {
            return whyRes.Chains[0].Path
        }
    }
    return nil
}
```
When creating package finding:
```go
isInitialRegression := p.Delta.InitialBytes > 0
finding := Finding{
    Name:         p.Name,
    DeltaBytes:   effectiveDelta,
    InitialDelta: p.Delta.InitialBytes,
    LazyDelta:    p.Delta.LazyBytes,
    Kind:         "package",
    Chunks:       findEmittedChunks(p.Name),
    TracePath:    traceImportPath(p.Name, isInitialRegression),
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./internal/comparison -run TestGenerateFindings_InitialPackageRegression_ApplicationTrace`
Expected: PASS

- [ ] **Step 5: Run all comparison package tests**

Run: `rtk go test ./internal/comparison/...`
Expected: PASS

- [ ] **Step 6: Commit Task 2**

```bash
git add internal/comparison/explain.go internal/comparison/explain_test.go
git commit -m "feat(comparison): trace initial package regressions from application roots"
```

---

### Task 3: Format Application-to-Package Boundary in PR Reports

**Files:**
- Modify: `internal/report/github_pr.go:100-110`
- Modify: `internal/report/markdown.go:140-150`
- Test: `internal/report/github_pr_test.go`
- Test: `internal/report/markdown_test.go`

**Interfaces:**
- Consumes: `Finding.TracePath []string`
- Produces: Formatted `Import path: ...` markdown string showing meaningful application boundary

- [ ] **Step 1: Write test for compact/informative import path rendering**

In `internal/report/github_pr_test.go`:
Verify that when `TracePath` is `["apps/portal/src/main.ts", "apps/portal/src/app/app.config.ts", "apps/portal/src/app/app.module.ts", "libs/timezone-scheduler/src/index.ts", "node_modules/moment-timezone/index.js"]`:
The markdown output cleanly renders the entrypoint, the importing application file, the library boundary, and the target package:
`apps/portal/src/main.ts → apps/portal/src/app/app.module.ts → libs/timezone-scheduler/src/index.ts → node_modules/moment-timezone/index.js`

- [ ] **Step 2: Run test to verify failure/expectation**

Run: `rtk go test ./internal/report -run TestGitHubPR_ImportPath`

- [ ] **Step 3: Implement helper `formatTracePath(path []string) string` in `internal/report/github_pr.go` and `markdown.go`**

Implement `formatTracePath` to eliminate redundant intermediate hops if a path is longer than 4 elements, keeping:
1. Root entrypoint (`main.ts`)
2. Immediate application importer (`app.module.ts`)
3. Library boundary (`libs/.../index.ts` if present)
4. Emitted dependency (`moment-timezone`)
Or render the concise `A → ... → B → C` path without intermediate framework boilerplate.

- [ ] **Step 4: Run all report package tests**

Run: `rtk go test ./internal/report/...`
Expected: PASS

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/report/github_pr.go internal/report/markdown.go internal/report/github_pr_test.go internal/report/markdown_test.go
git commit -m "feat(report): format application ingress path cleanly in PR comments"
```

---

### Task 4: End-to-End Verification on `bundlecheck-nx-test`

**Files:**
- Build: `bundlecheck` binary in `/Users/sonukumar/project/bundlecheck/`
- Test against: `/Users/sonukumar/project/bundlecheck-nx-test/dist/apps/portal`

- [ ] **Step 1: Build local bundlecheck binary**

Run in `/Users/sonukumar/project/bundlecheck`:
`rtk go build -o /Users/sonukumar/go/bin/bundlecheck .`

- [ ] **Step 2: Run `bundlecheck why moment-timezone` in `bundlecheck-nx-test`**

Run in `/Users/sonukumar/project/bundlecheck-nx-test`:
`rtk bundlecheck why moment-timezone -p portal`
Expected output:
Chain starts with `apps/portal/src/main.ts` and traverses `app.module.ts` to `libs/timezone-scheduler/src/index.ts` to `moment-timezone`.

- [ ] **Step 3: Run `bundlecheck measure` against baseline**

Run in `/Users/sonukumar/project/bundlecheck-nx-test`:
`rtk bundlecheck measure -p portal -b .bundlecheck/baseline-origin-master.json -o test-report.md`
Verify `test-report.md` shows the application ingress path for `moment-timezone` in the Regression Explanation section!

- [ ] **Step 4: Clean up temporary test file**

Run: `rm -f test-report.md`

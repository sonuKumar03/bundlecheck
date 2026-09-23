# BundleRadar v2: Modular Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform BundleRadar into a universal, framework-agnostic, zero-dependency bundle analysis, diffing, and budget gate engine with a clean Hexagonal (Ports & Adapters) architecture.

**Architecture:** Pure Go domain core (`internal/core/`) representing a universal bundle AST; pluggable ingress adapters (`internal/adapters/parsers` and `internal/adapters/workspaces`) with auto-detection; pure diff and policy evaluator; pluggable egress reporters (`internal/adapters/reporters`); and a streamlined 4-verb CLI (`cmd/`) alongside a public Go SDK (`pkg/bundleradar`).

**Tech Stack:** Go (Standard Library, Cobra, pflag). Zero third-party dependencies in core domain.

**Spec:** `docs/superpowers/specs/2026-09-23-bundleradar-v2-modular-architecture-design.md`

## Global Constraints
- All shell commands must be prefixed with `rtk ` (e.g. `rtk go test ./...`, `rtk git commit ...`).
- Zero external runtime dependencies: no Node.js, npm, or Python required by the binary.
- Pure domain models in `internal/core` must not import any adapter, CLI, or third-party package.
- All tasks must follow strict Test-Driven Development (TDD): write failing test first, verify failure, implement code, verify green, commit.

---

### Task 1: Core Domain AST & Ports (`internal/core`)

**Files:**
- Create: `internal/core/bundle.go`
- Create: `internal/core/ports.go`
- Test: `internal/core/bundle_test.go`

**Interfaces:**
- Produces:
  - `core.LoadType`: Enum (`"initial"`, `"async"`, `"worker"`).
  - `core.Bundle`: Root aggregate with `Metadata`, `Entrypoints`, `Chunks`, `Modules`, `Assets`.
  - `core.Entrypoint`: `Name`, `InitialBytes`, `InitialGzipBytes`, `AsyncBytes`, `ChunkIDs`.
  - `core.Chunk`: `ID`, `Name`, `Path`, `SizeBytes`, `GzipBytes`, `Type`, `ModuleIDs`.
  - `core.Module`: `ID`, `Package`, `Version`, `SizeBytes`, `GzipBytes`, `IsAppCode`, `ChunkIDs`, `IngressPaths`.
  - `core.Asset`: `Path`, `SizeBytes`, `GzipBytes`, `MimeType`.
  - `core.Target`: `Name`, `StatsPath`, `DistPath`, `Bundler`.
  - `core.Parser`: Interface (`Name()`, `Detect()`, `Parse()`).
  - `core.WorkspaceResolver`: Interface (`Name()`, `Detect()`, `Resolve()`).
  - `core.BaselineProvider`: Interface (`Name()`, `Fetch()`, `Save()`).
  - `core.Reporter`: Interface (`Format()`, `Render()`).

- [ ] **Step 1: Write failing unit tests for core domain model validation**

In `internal/core/bundle_test.go`:
```go
package core_test

import (
	"testing"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

func TestBundleAggregation(t *testing.T) {
	b := core.NewBundle(core.Metadata{Bundler: "test"})
	b.AddEntrypoint("main", core.Entrypoint{
		Name:         "main",
		InitialBytes: 150000,
	})
	if b.TotalInitialBytes() != 150000 {
		t.Fatalf("expected 150000 initial bytes, got %d", b.TotalInitialBytes())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/core -run TestBundleAggregation`
Expected: FAIL (types / methods not declared yet)

- [ ] **Step 3: Implement core AST and ports**

Implement `internal/core/bundle.go` and `internal/core/ports.go` with helper methods (`TotalInitialBytes`, `TotalAsyncBytes`, `FindModule`, `FindChunk`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/core -run TestBundle`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/core/
rtk git commit -m "feat(core): implement universal bundle AST and ports interfaces"
```

---

### Task 2: Pluggable Parser Adapters & Sniff Registry (`internal/adapters/parsers`)

**Files:**
- Create: `internal/adapters/parsers/registry.go`
- Create: `internal/adapters/parsers/esbuild.go`
- Create: `internal/adapters/parsers/angular.go`
- Create: `internal/adapters/parsers/vite.go`
- Create: `internal/adapters/parsers/webpack.go`
- Test: `internal/adapters/parsers/parsers_test.go`

**Interfaces:**
- Consumes: `core.Parser`, `core.Bundle`, `core.Target`.
- Produces: `parsers.DefaultRegistry()`, `parsers.Registry.Resolve(target core.Target) (core.Parser, error)`.

- [ ] **Step 1: Write failing tests for parser auto-detection & parsing**

In `internal/adapters/parsers/parsers_test.go`:
- Test auto-detection for esbuild metafile fixture (`testdata/minimal/stats.json`).
- Test Angular CLI auto-detection and chunk classification (`testdata/lazy-import/stats.json`).
- Test Vite manifest mock JSON detection.
- Test Webpack stats mock JSON detection.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/parsers`
Expected: FAIL

- [ ] **Step 3: Implement Parser Registry and 4 built-in adapters**

- `registry.go`: Sniffs first 8KB of stats file and checks `Detect(sniff, distDir)`.
- `esbuild.go`: Parses standard esbuild metafile JSON (`inputs`, `outputs`).
- `angular.go`: Extends esbuild parsing with Angular entrypoint conventions (`main.js`, `polyfills.js`, `styles.css`).
- `vite.go`: Parses Vite `manifest.json`.
- `webpack.go`: Parses Webpack `stats.json`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/adapters/parsers`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/adapters/parsers/
rtk git commit -m "feat(parsers): implement pluggable parser registry and adapters (esbuild, angular, vite, webpack)"
```

---

### Task 3: Pluggable Workspace Resolvers (`internal/adapters/workspaces`)

**Files:**
- Create: `internal/adapters/workspaces/registry.go`
- Create: `internal/adapters/workspaces/explicit.go`
- Create: `internal/adapters/workspaces/monorepo.go`
- Create: `internal/adapters/workspaces/nx.go`
- Test: `internal/adapters/workspaces/workspaces_test.go`

**Interfaces:**
- Consumes: `core.WorkspaceResolver`, `core.Target`.
- Produces: `workspaces.DefaultRegistry()`, `workspaces.Registry.Resolve(root string, explicit []core.Target) ([]core.Target, error)`.

- [ ] **Step 1: Write failing tests for workspace resolvers**

In `internal/adapters/workspaces/workspaces_test.go`:
- Test `ExplicitResolver` with `--target` arguments.
- Test `MonorepoResolver` discovering packages in a pnpm/npm workspace fixture.
- Test `NxResolver` discovering projects in `testdata/nx-workspace/nx.json`.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/workspaces`
Expected: FAIL

- [ ] **Step 3: Implement Workspace Registry and resolvers**

- `explicit.go`: Parses direct user targets.
- `monorepo.go`: Discovers build artifacts from package workspaces.
- `nx.go`: Reads `nx.json` if present without requiring the Nx CLI.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/adapters/workspaces`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/adapters/workspaces/
rtk git commit -m "feat(workspaces): implement pluggable workspace resolvers (explicit, monorepo, nx)"
```

---

### Task 4: Universal Diff & Source Attribution Engine (`internal/core/diff`)

**Files:**
- Create: `internal/core/diff/diff.go`
- Test: `internal/core/diff/diff_test.go`

**Interfaces:**
- Consumes: `core.Bundle`.
- Produces: `diff.Diff(base, current *core.Bundle, opts diff.Options) *core.BundleDiff`.

- [ ] **Step 1: Write failing tests for diffing two bundles**

In `internal/core/diff/diff_test.go`:
- Test entrypoint initial and async deltas.
- Test chunk addition, removal, and size change.
- Test package size regression attribution.
- Test micro-drift collapsing (`<1KB`).

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/core/diff`
Expected: FAIL

- [ ] **Step 3: Implement Diff engine**

Calculate deltas, detect added/removed chunks, map package increases to contributing source modules, and bucket changes below threshold into micro-drift.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/core/diff`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/core/diff/
rtk git commit -m "feat(diff): implement universal bundle diff and source attribution engine"
```

---

### Task 5: Policy & Budget Gate Engine (`internal/core/policy`)

**Files:**
- Create: `internal/core/policy/policy.go`
- Test: `internal/core/policy/policy_test.go`

**Interfaces:**
- Consumes: `core.Bundle`, `diff.BundleDiff`.
- Produces: `policy.Evaluate(bundle *core.Bundle, d *diff.BundleDiff, p policy.Policy) policy.EvaluationResult`.

- [ ] **Step 1: Write failing tests for policy checks**

In `internal/core/policy/policy_test.go`:
- Test `MaxInitial` limit violation.
- Test `MaxInitialDelta` regression violation.
- Test forbidden package detection (`forbid: ["moment"]`).
- Test duplicate package version detection.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/core/policy`
Expected: FAIL

- [ ] **Step 3: Implement Policy evaluation engine**

Evaluate absolute sizes, regressions, package bans, and version duplication with zero external dependencies.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/core/policy`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/core/policy/
rtk git commit -m "feat(policy): implement budget and architecture policy evaluator"
```

---

### Task 6: Egress Reporters & Public Go SDK (`internal/adapters/reporters` & `pkg/bundleradar`)

**Files:**
- Create: `internal/adapters/reporters/terminal.go`
- Create: `internal/adapters/reporters/markdown.go`
- Create: `internal/adapters/reporters/github_pr.go`
- Create: `internal/adapters/reporters/json.go`
- Create: `pkg/bundleradar/client.go`
- Test: `internal/adapters/reporters/reporters_test.go`
- Test: `pkg/bundleradar/client_test.go`

**Interfaces:**
- Consumes: `core.Reporter`, `core.Bundle`, `diff.BundleDiff`, `policy.EvaluationResult`.
- Produces: `reporters.New(format string) (core.Reporter, error)`, `bundleradar.New()`.

- [ ] **Step 1: Write failing tests for reporters & SDK**

- Test terminal rendering with ANSI formatting.
- Test markdown rendering with GitHub Flavored Markdown and tables.
- Test GitHub PR reporter with sticky comments (`<!-- bundleradar-report -->`).
- Test JSON v2 output schema.
- Test Go client programmatic scan & gate.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/reporters ./pkg/bundleradar`
Expected: FAIL

- [ ] **Step 3: Implement Reporters and SDK Client**

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./internal/adapters/reporters ./pkg/bundleradar`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/adapters/reporters/ pkg/bundleradar/
rtk git commit -m "feat(reporters): implement terminal, markdown, github-pr, json reporters and public Go SDK"
```

---

### Task 7: Streamlined CLI Verbs (`cmd/`)

**Files:**
- Create: `cmd/scan.go`
- Create: `cmd/diff.go`
- Create: `cmd/gate.go`
- Modify: `cmd/root.go`
- Modify: `cmd/workspace.go`
- Test: `cmd/v2_cli_test.go`

**Interfaces:**
- CLI verbs: `bundleradar scan`, `bundleradar diff`, `bundleradar gate`, `bundleradar workspace`.

- [ ] **Step 1: Write failing end-to-end CLI tests**

In `cmd/v2_cli_test.go`:
- Test `scan` on `testdata/minimal/stats.json`.
- Test `scan --why lodash` on `testdata/minimal`.
- Test `diff` comparing before and after fixtures.
- Test `gate` passing on compliant budget and failing with exit code 1 on exceeded threshold.

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./cmd -run TestV2CLI`
Expected: FAIL

- [ ] **Step 3: Implement new CLI commands**

Wire Cobra commands to call `pkg/bundleradar` client methods directly. Keep commands thin.

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test ./cmd -run TestV2CLI`
Expected: PASS

- [ ] **Step 5: Run full test suite across the repo**

Run: `rtk go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git add cmd/
rtk git commit -m "feat(cmd): implement streamlined v2 CLI verbs (scan, diff, gate, workspace)"
```

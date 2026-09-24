# Angular Adapter & Parser Correctness Verification Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Formally verify and harden the correctness of the Angular (>= 17) parser and workspace adapters from first principles using the real build artifacts of `testdata/nx-workspace/dist/apps/portal` (and `admin-dashboard`), establishing a verifiable baseline without assuming Angular build internals.

**Architecture:** Iterative verification pipeline that derives ground truth directly from physical build files (`index.html`, chunks, stylesheets, source maps, `stats.json`). We compare the parser output against verifiable on-disk reality across four dimensions: chunk classification (initial vs async), byte size precision (raw and gzip), npm package attribution, and monorepo workspace resolution.

**Tech Stack:** Go 1.22+, `github.com/sonuKumar03/bundleradar` core engine, Angular 17+ Application Builder (esbuild metafile format), Nx workspace resolver.

**Spec/Fixture:** `testdata/nx-workspace/dist/apps/portal` & `testdata/nx-workspace/dist/apps/admin-dashboard`

---

## Global Constraints

- Exclusively target **Angular >= 17** applications built with the esbuild-powered application builder (`@angular-devkit/build-angular:application` / `@angular/build:application`).
- Do not commit changes; keep all work staged (`git add`) as instructed by user.
- Every test must compare against physical ground truth on disk (e.g. `os.Stat`, real gzip via `compress/gzip`, exact HTML tags in `index.html`).
- Zero external dependencies introduced into `internal/core`.

---

## File Structure

- **Tests to Create/Modify**:
  - `internal/adapters/parsers/angular_verification_test.go`: Deep verification test suite comparing parser output against physical `portal` and `admin-dashboard` dist directories.
  - `internal/adapters/parsers/angular.go`: Angular >= 17 parser implementation.
  - `internal/adapters/workspaces/nx_test.go`: Unit and integration tests for Nx workspace resolver targeting real Angular projects.
  - `internal/adapters/reporters/terminal_test.go`: Reporter integration tests verifying label and chunk count consistency.

---

## Tasks

### Task 1: Ground Truth Extraction & On-Disk Baseline

**Files:**
- Create: `internal/adapters/parsers/angular_verification_test.go`
- Test: `testdata/nx-workspace/dist/apps/portal/browser/*`

**Interfaces:**
- Consumes: Physical files in `testdata/nx-workspace/dist/apps/portal`
- Produces: `PortalGroundTruth` struct capturing real initial chunks, lazy chunks, raw byte sizes, and disk-measured gzip bytes.

- [x] **Step 1: Write ground truth inspector helper in `angular_verification_test.go`**
  Write a test helper that inspects `testdata/nx-workspace/dist/apps/portal/browser/index.html`:
  - Parses `<script src="...">`, `<link rel="stylesheet" href="...">`, and `<link rel="modulepreload" href="...">`.
  - Extracts the exact set of initial chunks the browser loads.
  - Measures physical file size with `os.Stat` for each file.
  - Calculates real `gzip` compressed size using `compress/gzip`.

- [x] **Step 2: Run test to print baseline metrics**
  Run `rtk go test -v ./internal/adapters/parsers/ -run TestPortalGroundTruthBaseline` to assert:
  - Exact initial files: `main-*.js`, `polyfills-*.js`, `styles-*.css`, plus the 6 `chunk-*.js` modulepreload files.
  - Exact lazy files: all remaining `chunk-*.js` not in `index.html`.

- [x] **Step 3: Stage changes**
  Stage test file with `rtk git add internal/adapters/parsers/angular_verification_test.go`.

---

### Task 2: Initial vs. Async Chunk Boundary Verification

**Files:**
- Modify: `internal/adapters/parsers/angular_verification_test.go`
- Target: `internal/adapters/parsers/angular.go`

**Interfaces:**
- Consumes: `core.Target{StatsPath: ..., DistPath: ...}`
- Produces: `*core.Bundle` with exact `Chunk.Type` classification matching `index.html`.

- [x] **Step 1: Write failing test verifying 100% chunk classification equivalence**
  Assert that every chunk marked `core.LoadTypeInitial` by `AngularParser`:
  - Appears in `index.html` (either as `<script>`, `<link rel="stylesheet">`, or `<link rel="modulepreload">`).
  - And every chunk marked `core.LoadTypeAsync` does NOT appear in `index.html`.
  - Assert that total initial bytes in `bundle.Entrypoints["main"].InitialBytes` matches `sum(os.Stat(initial_file).Size())`.

- [x] **Step 2: Run test and observe output**
  Execute `rtk go test -v ./internal/adapters/parsers/ -run TestAngularParser_InitialVsAsyncBoundary`.

- [x] **Step 3: Fix any boundary edge cases in `angular.go`**
  Ensure graph traversal handles edge cases:
  - Entrypoint prefixes: `main`, `polyfills`, `styles`.
  - Transitive static imports: `out.Imports` where `Kind == "import-statement"`.
  - Dynamic imports: `out.Imports` where `Kind == "dynamic-import"`.
  - Component CSS: excluded from JS code chunks and tracked under `bundle.Assets`.

- [x] **Step 4: Verify test passes**
  Run `rtk go test -v ./internal/adapters/parsers/ -run TestAngularParser_InitialVsAsyncBoundary`.

- [x] **Step 5: Stage changes**
  Run `rtk git add -u`.

---

### Task 3: Package Attribution & First-Party vs Third-Party Attribution

**Files:**
- Modify: `internal/adapters/parsers/angular_verification_test.go`
- Target: `internal/adapters/parsers/angular.go`

**Interfaces:**
- Consumes: `bundle.TopPackages(limit)`, `bundle.TotalAppCodeBytes()`
- Produces: Verified attribution matching disk files and `stats.json.inputs`.

- [x] **Step 1: Write test for package attribution and module deduplication**
  In `angular_verification_test.go`:
  - Assert that `TopPackages` ranks known heavy packages: `exceljs`, `pdfjs-dist`, `chart.js`, `@angular/material`, `@angular/core`.
  - Verify that no module in `bundle.Modules` is duplicated (all `m.ID` are distinct).
  - Verify that modules shared across chunks have multiple entries in `m.ChunkIDs`.
  - Verify that `isAppCode: true` applies strictly to `apps/portal/...` and `libs/...`, while `node_modules/...` is marked `isAppCode: false`.

- [x] **Step 2: Run test and confirm assertions**
  Execute `rtk go test -v ./internal/adapters/parsers/ -run TestAngularParser_PackageAttribution`.

- [x] **Step 3: Stage changes**
  Run `rtk git add -u`.

---

### Task 4: Real Disk Gzip Transfer vs Estimate Precision Check

**Files:**
- Modify: `internal/adapters/parsers/angular_verification_test.go`
- Target: `internal/adapters/parsers/angular.go`

**Interfaces:**
- Consumes: Physical file gzip measurements vs `bundle.Entrypoints["main"].InitialGzipBytes`.
- Produces: Documented comparison between real gzip (disk) and estimate heuristic (metafile).

- [x] **Step 1: Write test measuring delta between real gzip and parser estimate**
  - Read all physical initial chunk bytes and compress with standard gzip (level 6).
  - Compare the real wire size against `bundle.Entrypoints["main"].InitialGzipBytes`.
  - Ensure estimate is within an acceptable bound (e.g. ±15% of true wire transfer), or enable real gzip reading when `target.DistPath` contains matching files.

- [x] **Step 2: Run test and record findings**
  Execute `rtk go test -v ./internal/adapters/parsers/ -run TestAngularParser_GzipPrecision`.

- [x] **Step 3: Stage changes**
  Run `rtk git add -u`.

---

### Task 5: Multi-App Nx Workspace Adapter End-to-End Verification

**Files:**
- Modify: `internal/adapters/workspaces/workspaces_test.go`
- Target: `internal/adapters/workspaces/nx.go`

**Interfaces:**
- Consumes: `NxResolver.Resolve(ctx, "testdata/nx-workspace")`
- Produces: Validated `core.Target` list with correct `StatsPath` and `DistPath`.

- [x] **Step 1: Write end-to-end workspace scan test for real Angular apps**
  Test resolving and scanning `portal` and `admin-dashboard`:
  - Assert `portal` target points to `dist/apps/portal/stats.json` and `dist/apps/portal/browser`.
  - Assert `admin-dashboard` target points to `dist/apps/admin-dashboard/stats.json` and `dist/apps/admin-dashboard/browser`.
  - Assert both targets parse successfully with `AngularParser`.
  - Assert `portal` initial bytes ≈ 2.19 MB, `admin-dashboard` initial bytes ≈ 1.15 MB.

- [x] **Step 2: Run test and verify clean execution**
  Execute `rtk go test -v ./internal/adapters/workspaces/...`.

- [x] **Step 3: Stage changes**
  Run `rtk git add -u`.

---

### Task 6: CLI & Reporter Verification

**Files:**
- Test via `cmd/scan.go` and `cmd/workspace.go`
- Target: `internal/adapters/reporters/terminal.go`

**Interfaces:**
- Consumes: Terminal reporter output for `portal` and `admin-dashboard`.
- Produces: Accurate terminal reports without label or count mismatch.

- [x] **Step 1: Verify `bundleradar scan` CLI output on `portal`**
  Run:
  `rtk go run . scan testdata/nx-workspace/dist/apps/portal/stats.json`
  Verify:
  - Initial JavaScript size matches Angular CLI output (2.08 MB JS + 6.85 KB CSS).
  - Async chunk count matches actual lazy chunks (5 lazy chunks).
  - Top 5 NPM packages accurately list real dependencies (`exceljs`, `pdfjs-dist`, `chart.js`, `@angular/material`, `@angular/core`).

- [x] **Step 2: Verify `bundleradar workspace scan` CLI output on `testdata/nx-workspace`**
  Run:
  `rtk go run . workspace scan --root testdata/nx-workspace`
  Verify:
  - Discovers both `portal` and `admin-dashboard`.
  - Displays correct initial and async byte totals for both projects.

- [x] **Step 3: Run full verification suite**
  Run `rtk go test ./... && rtk go vet ./...`.

- [x] **Step 4: Keep all changes staged**
  Run `rtk git status` and verify clean staged status.

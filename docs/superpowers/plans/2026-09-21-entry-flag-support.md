# Optional `--entry` Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` task-by-task. Do not commit this plan unless the user explicitly asks.

**Goal:** Add optional `--entry` / `-e` selection to entry-sensitive CLI flows and the GitHub Action so classification, budgets, baselines, traces, and suggestions use the selected application or worker entry while no-flag behavior and schema-v1 files remain compatible.

**Architecture:** Resolve an entry from normalized browser outputs with one shared stdlib matcher. `build.LoadWithEntry` uses matched emitted paths as `graph.Classify` roots; an empty selector keeps today's `index.html` path. Graph tracing and advisor chains reuse the matcher to derive source-module roots. Existing functions remain no-entry compatibility wrappers.

**Tech Stack:** Go 1.27.1, Cobra, Go standard library (`path`, `slices`), Angular/esbuild stats, composite GitHub Actions, YAML documentation-contract tests.

**Spec:** This plan is the implementation spec; no separate entry-flag spec exists.

- Support `--entry` / `-e` in `summary`, `suggest`, `why`, `measure`, `check`, and `baseline save` / `baseline create`.
- `baseline rebuild` reuses the entry stored by save/create. Older baselines without the optional field still load and rebuild.
- Add optional Action input `entry` and pass it through the shared `ARGS` array to baseline capture, measure, summary, JSON summary, and check.
- Match normalized `BundleOutput.EntryPoint`, output `Path`, or output basename. Support exact values and `path.Match` globs such as `src/main.ts`, `src/crypto.worker.ts`, `main.js`, and `main-*.js`.
- Multiple matches are valid roots. Deduplicate and sort them by normalized output path.
- Malformed globs and unmatched selectors return clear errors containing the selector.
- Entry mode validates emitted browser files but does not require `index.html`. Empty-entry mode preserves current index discovery and golden output.
- Keep every browser JavaScript output in the snapshot. Only `Initial`/lazy reachability and entry-rooted chains are scoped; `TotalJS` remains whole-browser.
- Add no dependency and no public analysis-result field.

## Global Constraints

- Prefix shell commands with `rtk`.
- Use TDD: write the smallest failing test, observe the expected failure, implement, rerun the focused package.
- Preserve `BrowserOutputs`, `build.Load`, `graph.NewGraph`, `graph.TracePackage`, `advisor.Analyze`, and `runAnalysisWithOptions` as no-entry paths.
- Keep baseline `schemaVersion: "1"`; add only optional metadata field `Entry` tagged `json:"entry,omitempty"`.
- Leave `compare`, `inspect`, `workspace`, and MCP behavior unchanged.
- Do not add a custom glob engine, entry registry, or configuration abstraction.

## Review Focus

- A worker entry works when dist has no `index.html`.
- The selector scopes both byte classification and the first node of `why`/advisor chains.
- No-entry JSON and contract/golden outputs do not drift.
- Action comparisons apply the same selector to baseline and current builds.
- Rebuild preserves a saved selector; pre-feature schema-v1 files remain valid.

## File Map and Interfaces

- `internal/artifact/initial.go`
  - Add `MatchEntryOutputs(outputs []snapshot.BundleOutput, pattern string) ([]snapshot.BundleOutput, error)`.
  - Add `BrowserOutputsWithEntry(outputs []snapshot.BundleOutput, dist, entry string) ([]snapshot.BundleOutput, []string, error)`.
- `internal/build/load.go`
  - Add `LoadWithEntry(stats, dist, entry string) (*snapshot.BundleSnapshot, error)`.
  - Keep `Load` delegating with an empty entry.
- `internal/graph/trace.go`
  - Add `NewGraphWithEntry(s *snapshot.BundleSnapshot, entry string) (*Graph, error)`.
  - Add `TracePackageWithEntry(s *snapshot.BundleSnapshot, target, entry string, initialOnly bool, maxChains int) (*WhyResult, error)`.
- `internal/advisor/advisor.go`
  - Add `AnalyzeWithEntry(s *snapshot.BundleSnapshot, opts AdvisorOptions, entry string) (*AdvisorResult, error)`.
  - Keep `Analyze` with its current return type and no-entry behavior.
- `internal/baseline/baseline.go`: add optional metadata field `Entry` with JSON name `entry`.
- Tests: corresponding `*_test.go` files in artifact, build, graph, advisor, baseline, and cmd.
- CLI: `cmd/summary.go`, `suggest.go`, `why.go`, `measure.go`, `check.go`, `baseline.go`.
- Action/docs: `action.yml`, `.github/workflows/e2e.yml`, `docs_contract_test.go`, `README.md`, `npm/bundleradar/README.md`, `docs/index.html`.

---

### Task 1: Resolve entry outputs and classify the selected graph

**Files:** `internal/artifact/initial.go`, `internal/artifact/initial_test.go`, `internal/build/load.go`, `internal/build/load_test.go`

- [x] Add matcher table tests for source path, full output path, basename, `main-*.js`, backslash normalization, two sorted matches, malformed `[`, and no match.
- [x] Add an artifact test with matching `.js` files and no `index.html`. Entry mode must return all browser outputs and only the selected output path as a root.
- [x] Add a regression test proving `BrowserOutputs` still uses `index.html` and current roots.
- [x] Run `rtk go test ./internal/artifact -run 'Test(MatchEntryOutputs|BrowserOutputs)'`; confirm failure from the missing behavior.
- [x] Implement matching with `snapshot.CleanPath`, `path.Base`, and `path.Match`. Validate once, match three candidate forms, deduplicate by normalized path, sort with `slices.SortFunc`, and include the selector in errors.
- [x] Factor only browser-output filtering/file validation shared by the two paths. Entry mode skips index discovery; empty-entry mode retains existing index parsing.
- [x] Add a build test with two entries and an imported chunk. The chosen root and its non-dynamic emitted imports are initial, the other entry is lazy, and neither output is dropped.
- [x] Implement `LoadWithEntry` and make `Load` pass an empty entry.
- [x] Run `rtk go test ./internal/artifact ./internal/build`.

### Task 2: Scope traces and suggestions

**Files:** `internal/graph/trace.go`, `internal/graph/trace_test.go`, `internal/advisor/advisor.go`, `internal/advisor/advisor_test.go`

- [x] Add graph tests where main and worker entries import distinct packages. Selecting the worker by source and emitted glob roots chains at `src/worker.ts`; selecting main cannot return the worker-only chain.
- [x] Add multiple-match, malformed, unmatched, output-without-entrypoint, and default-trace regression cases.
- [x] Run `rtk go test ./internal/graph -run 'Test.*Entry'` and confirm failure.
- [x] Implement `NewGraphWithEntry` with `artifact.MatchEntryOutputs`, converting matched non-empty `EntryPoint` values into sorted graph roots. Return an error for a matched output without a source entrypoint.
- [x] Implement `TracePackageWithEntry` on that graph. Keep `TracePackage` unchanged for no-entry callers.
- [x] Add an advisor test where the same heavy package is reachable through distinct importers. The selected entry must determine the chain-derived file/action.
- [x] Implement `AnalyzeWithEntry` by building the scoped graph once and reusing the current suggestion loop. Keep `Analyze` as the no-entry wrapper.
- [x] Run `rtk go test ./internal/graph ./internal/advisor`.

### Task 3: Plumb CLI commands and baseline persistence

**Files:** `internal/baseline/baseline.go`, `internal/baseline/baseline_test.go`, six command files above, and their command tests

- [x] Add baseline tests proving old metadata loads, non-empty `entry` round-trips, and empty `entry` is omitted.
- [x] Add the optional `Entry` metadata field without changing schema version.
- [x] Add a shared command-test helper that writes a temporary two-entry stats file and emitted files. Do not commit generated Angular builds or duplicate fixtures per test.
- [x] Assert `summary`, `suggest`, `why`, `measure`, `check`, `baseline save`, and the `baseline create` alias expose `--entry` with shorthand `-e`.
- [x] Add behavior tests: summary scopes initial bytes, why scopes its root, suggest scopes its importer, check scopes its budget, measure scopes current bytes, save/create persists entry, and rebuild reuses it.
- [x] Run `rtk go test ./cmd ./internal/baseline -run 'Test.*Entry'` and confirm failure.
- [x] Add `runAnalysisWithEntry(stats, dist, entry string, withCompression bool)` using `build.LoadWithEntry`. Keep `runAnalysisWithOptions` as its empty-entry wrapper so `compare` is untouched.
- [x] Summary and suggest call `advisor.AnalyzeWithEntry` only for non-empty entry and propagate errors.
- [x] Why uses entry-aware load and trace only for non-empty entry.
- [x] Measure and check pass entry to current-build analysis. Do not invent baseline mismatch enforcement.
- [x] Baseline save/create stores the entry; rebuild reads and reuses it.
- [x] Run `rtk go test ./cmd ./internal/baseline` and `rtk go test ./cmd -run TestContracts`.

### Task 4: Wire Action and public documentation

**Files:** `action.yml`, `.github/workflows/e2e.yml`, `docs_contract_test.go`, `README.md`, `npm/bundleradar/README.md`, `docs/index.html`

- [x] Extend documentation-contract expectations for `--entry` / `-e` and the Action `entry` input across all three public documentation surfaces.
- [x] Run `rtk go test . -run TestDocumentationContract` and confirm failure.
- [x] Add optional Action input `entry`, export `INPUT_ENTRY`, and append `--entry "$INPUT_ENTRY"` once to shared `ARGS` when non-empty.
- [x] Confirm `ARGS` scopes base-ref capture, measure, summary, JSON extraction, and check identically.
- [x] Add an action-consumer smoke invocation with `entry: src/main.ts` and retain output assertions.
- [x] Document source and emitted-glob examples plus the whole-browser total invariant in all three docs.
- [x] Run `rtk go test . -run TestDocumentationContract`.

### Task 5: Production-readiness verification

- [x] `rtk go test ./internal/artifact ./internal/build ./internal/graph ./internal/advisor ./internal/baseline ./cmd`
- [x] `rtk go test ./...`
- [x] `rtk go test -race ./...`
- [x] `rtk go vet ./...`
- [x] `rtk go build ./...`
- [x] Compare no-entry JSON with existing contract/golden output; no field or ordering drift is allowed.
- [x] Smoke-test a source selector and emitted glob against the same two-entry fixture, including dist without `index.html`.
- [x] `rtk git diff --check` and `rtk git status --short`; keep this plan uncommitted unless separately authorized.

## Out of Scope

- MCP parameters, workspace-wide entry maps, compare/inspect/init flags, regex or negated selectors, and baseline-entry mismatch enforcement.
- Changing `TotalJS` semantics or removing non-selected outputs.
- New dependencies or schema-version bumps.

## Self-Review Checklist

- [x] Every goal has a failing test and implementation step.
- [x] Signatures agree across the file map and tasks.
- [x] Default wrappers preserve existing callers and output contracts.
- [x] No placeholder code or unresolved design choice remains.
- [x] Review-focus risks have focused tests and final verification.

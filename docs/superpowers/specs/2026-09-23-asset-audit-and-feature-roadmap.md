# Asset Audit — Design Spec (v2, audited)

**Date:** 2026-09-23
**Status:** Audited — ready for implementation planning
**Approach:** Test-Driven Development (TDD)

---

## 0. TDD Contract

Every implementation step in this spec follows Red → Green → Refactor:

1. **Write a failing test** that defines the exact behaviour (unit, table-driven, or golden).
2. **Write the minimum code** to make that test pass.
3. **Refactor** without breaking the test.

No production code is written before its test exists. The implementation order below (§7) reflects this — test files are listed before the production files they exercise.

---

## 1. Overview

`bundleradar` today audits only JavaScript outputs (initial + lazy chunks). The Angular/esbuild metafile contains full byte sizes for every emitted output — CSS bundles, images, fonts, and other file-loader assets — but the tool silently discards them at normalization time.

**Current data flow — assets silently dropped:**

```
esbuild metafile
  outputs[*].cssBundle           -> normalize.go ignores it
  outputs[*].bytes (non-JS ext)  -> analysis/summary.go:64: !IsJavaScript -> continue
  inputs[*].imports[file-loader] -> Asset: true set, but bytes never accumulated
```

This spec covers only the **Asset Auditing** feature (CSS + file-loader). The roadmap items (trend, diff, latency, dupes, overlap) are separate specs.

---

## 2. What Data Is Already Available

From `internal/angular/metafile.go`:

| Field | Type | Meaning |
|-------|------|---------|
| `Output.Bytes` | `int64` | Emitted file size — valid for CSS, image, font outputs |
| `Output.CSSBundle` | `string` | Path of the companion `.css` file for a JS chunk |
| `Output.EntryPoint` | `string` | Set on initial-entry JS chunks |
| `Import.Kind == "file-loader"` | `bool` | Import is a copied asset (image, font, binary) |
| `Import.Path` | `string` | Emitted asset output path |

From `internal/snapshot/model.go`:

| Field | Meaning |
|-------|---------|
| `Import.Asset` | Already set to `true` for `file-loader` in `normalize.go:44,81` |
| `BundleOutput.Initial` | Set by `graph.Classify` after normalization |

**Key constraint:** `graph.Classify` runs *after* `Normalize`. When `Normalize` runs, it does not yet know which JS chunks are initial. Therefore the CSS `Initial` flag cannot be set in `normalize.go` alone. It must be set in a second pass in `build.Load`, after `graph.Classify` marks the JS outputs.

---

## 3. Scope — v0.7

**In scope:**
- `internal/snapshot/model.go` — four new `omitempty` fields on `Totals`; `CSSPath`/`AssetPaths` on `BundleOutput`; `IsCSS()` helper
- `internal/angular/normalize.go` — record `CSSPath` and `AssetPaths`; emit CSS `BundleOutput` entries (Initial flag deferred to post-classify)
- `internal/build/load.go` — new `attachAssetInitialFlags()` pass after `graph.Classify`
- `internal/analysis/summary.go` — two new accumulation passes for CSS and assets
- `internal/budget/budget.go` — four new budget fields and comparison cases
- `internal/config/config.go` — four new YAML budget keys
- `internal/report/text.go` — asset rows in `TextWithOptions`
- `internal/report/markdown.go` — asset rows in comparison and summary tables
- `internal/report/github_pr.go` — asset delta rows in PR comment table
- `cmd/check.go`, `cmd/measure.go` — four new `--max-*` flags
- `internal/mcp/server.go` — `include_assets` param; expose asset totals in JSON

**Out of scope for v0.7:**
- Per-file-type breakdown (images vs fonts vs media) — asset paths are content-hashed and extension-opaque after esbuild
- Brotli compression estimates for assets
- Source-level CSS attribution via CSS source maps
- Gzip for assets (Gzip is JS-only today; assets use `Content-Encoding` differently)

---

## 4. Data Model Changes

### 4.1 internal/snapshot/model.go

**New fields on `Totals`** (all `omitempty` — backwards compatible, schema version stays `"1"`):

```go
type Totals struct {
    InitialJS     int64 `json:"initialJs"`
    InitialGzipJS int64 `json:"initialGzipJs,omitempty"`
    LazyJS        int64 `json:"lazyJs"`
    LazyGzipJS    int64 `json:"lazyGzipJs,omitempty"`
    TotalJS       int64 `json:"totalJs"`
    TotalGzipJS   int64 `json:"totalGzipJs,omitempty"`
    // Added in v0.7 — zero-value when no assets present (omitempty)
    InitialCSS    int64 `json:"initialCss,omitempty"`
    TotalCSS      int64 `json:"totalCss,omitempty"`
    InitialAssets int64 `json:"initialAssets,omitempty"`
    TotalAssets   int64 `json:"totalAssets,omitempty"`
}
```

**New fields on `BundleOutput`**:

```go
type BundleOutput struct {
    Path       string         `json:"path"`
    DiskPath   string         `json:"-"`
    Bytes      int64          `json:"bytes"`
    GzipBytes  int64          `json:"gzipBytes,omitempty"`
    Initial    bool           `json:"initial"`
    EntryPoint string         `json:"entryPoint,omitempty"`
    Inputs     []Contribution `json:"inputs"`
    Imports    []Import       `json:"imports"`
    // Added in v0.7
    CSSPath    string         `json:"cssPath,omitempty"`    // companion CSS bundle output path
    AssetPaths []string       `json:"assetPaths,omitempty"` // file-loader emitted asset output paths
}
```

**New helper** (alongside `IsJavaScript`):

```go
func IsCSS(p string) bool {
    return strings.ToLower(path.Ext(p)) == ".css"
}
```

### 4.2 internal/angular/normalize.go

In the JS output normalization loop (after building `o.Imports`):

```go
// Record companion CSS bundle path
if raw.CSSBundle != "" {
    o.CSSPath = snapshot.CleanPath(raw.CSSBundle)
}

// Record file-loader asset paths
for _, imp := range raw.Imports {
    if imp.Kind == "file-loader" {
        o.AssetPaths = append(o.AssetPaths, snapshot.CleanPath(imp.Path))
    }
}
```

For each CSS output (path ends in `.css`): emit a `BundleOutput` with `Bytes` set. **Do not** set `Initial` here — that is set in `build.Load` after graph classification.

```go
// Emit CSS outputs as BundleOutput entries (Initial set later)
if snapshot.IsCSS(name) {
    s.Outputs = append(s.Outputs, snapshot.BundleOutput{
        Path:   name,
        Bytes:  raw.Bytes,
        Inputs: []snapshot.Contribution{},
        Imports: []snapshot.Import{},
    })
}
```

### 4.3 internal/build/load.go — attachAssetInitialFlags()

After `graph.Classify(outputs, roots)` sets `Initial` on JS outputs, run a second pass to propagate `Initial` to CSS outputs referenced by initial JS chunks:

```go
// attachAssetInitialFlags marks CSS outputs as initial when their CSSPath
// is referenced by an initial JS chunk, and deduplicated asset outputs as
// initial when referenced by any initial JS output.
func attachAssetInitialFlags(outputs []snapshot.BundleOutput) {
    // Build set of CSS paths referenced by initial JS chunks
    initialCSSPaths := make(map[string]bool)
    for _, o := range outputs {
        if o.Initial && o.CSSPath != "" {
            initialCSSPaths[o.CSSPath] = true
        }
    }
    // Mark CSS outputs
    for i, o := range outputs {
        if snapshot.IsCSS(o.Path) && initialCSSPaths[o.Path] {
            outputs[i].Initial = true
        }
    }
    // Assets: mark as initial if any referencing output is initial
    initialAssetPaths := make(map[string]bool)
    for _, o := range outputs {
        if o.Initial {
            for _, ap := range o.AssetPaths {
                initialAssetPaths[ap] = true
            }
        }
    }
    // (Asset outputs are not BundleOutputs; bytes are looked up via AssetPaths.
    //  The initial flag is used only in analysis, not in the outputs slice.)
    // Store on snapshot for analysis use via a helper (see §4.4).
    _ = initialAssetPaths // used in Analyze, passed separately
}
```

**Open question resolved:** Asset bytes are *not* emitted as separate `BundleOutput` entries (esbuild doesn't emit image/font outputs in the same format). Instead, `AssetPaths` on JS outputs are the canonical record. The analysis pass accumulates bytes from `BundleOutput.Bytes` for matching non-JS, non-CSS output entries if present, or 0 if not.

### 4.4 internal/analysis/summary.go — Analyze()

After the existing JS loop (which skips non-JS), add two new passes:

```go
// Pass 2: CSS bundle accumulation
// CSS outputs were emitted by normalize.go and marked Initial by attachAssetInitialFlags.
for _, o := range s.Outputs {
    id := snapshot.CleanPath(o.Path)
    if seenOutputs[id] { continue } // already counted
    seenOutputs[id] = true
    if !snapshot.IsCSS(id) { continue }
    if err := add(&r.Summary.TotalCSS, o.Bytes); err != nil {
        return nil, fmt.Errorf("css output %q: %w", id, err)
    }
    if o.Initial {
        if err := add(&r.Summary.InitialCSS, o.Bytes); err != nil {
            return nil, fmt.Errorf("css output %q: %w", id, err)
        }
    }
}

// Pass 3: file-loader asset accumulation (deduplicated across all JS outputs)
// Build index: asset path -> output entry (if present as a BundleOutput)
assetIndex := make(map[string]*snapshot.BundleOutput)
for i := range s.Outputs {
    o := &s.Outputs[i]
    p := snapshot.CleanPath(o.Path)
    if !snapshot.IsJavaScript(p) && !snapshot.IsCSS(p) {
        assetIndex[p] = o
    }
}
seenAssets := make(map[string]bool)
for _, o := range s.Outputs {
    if !snapshot.IsJavaScript(o.Path) { continue }
    for _, ap := range o.AssetPaths {
        if seenAssets[ap] { continue }
        seenAssets[ap] = true
        asset, ok := assetIndex[ap]
        if !ok { continue } // asset not emitted as a trackable output
        if err := add(&r.Summary.TotalAssets, asset.Bytes); err != nil {
            return nil, fmt.Errorf("asset %q: %w", ap, err)
        }
        if o.Initial {
            if err := add(&r.Summary.InitialAssets, asset.Bytes); err != nil {
                return nil, fmt.Errorf("asset %q: %w", ap, err)
            }
        }
    }
}
```

**Note:** `findOutput()` is NOT introduced as a standalone function. The asset index map is built inline as shown above.

---

## 5. Budget Changes

### 5.1 internal/budget/budget.go

The existing `Budgets` struct and `Evaluate` function. **Add four new fields:**

```go
type Budgets struct {
    // ...existing fields...
    InitialCSSMax    int64 `yaml:"initial_css_max"`
    TotalCSSMax      int64 `yaml:"total_css_max"`
    InitialAssetsMax int64 `yaml:"initial_assets_max"`
    TotalAssetsMax   int64 `yaml:"total_assets_max"`
}
```

In `Evaluate(totals snapshot.Totals, b Budgets) []Violation`, append four new comparisons following the exact same pattern as the existing `InitialJSMax` check:

```go
if b.InitialCSSMax > 0 && totals.InitialCSS > b.InitialCSSMax {
    violations = append(violations, Violation{
        Metric:  "initialCss",
        Actual:  totals.InitialCSS,
        Budget:  b.InitialCSSMax,
        Exceeds: totals.InitialCSS - b.InitialCSSMax,
    })
}
// ... repeat for TotalCSSMax, InitialAssetsMax, TotalAssetsMax
```

### 5.2 internal/config/config.go

The YAML budget block gains four new optional keys (zero value = unlimited):

```yaml
budgets:
  initial_css_max: 100KB
  total_css_max: 500KB
  initial_assets_max: 2MB
  total_assets_max: 5MB
```

These map directly to the new `Budgets` fields above. The existing `ParseBytes` utility handles `KB`/`MB`/`GB` parsing — reuse it.

### 5.3 cmd/check.go and cmd/measure.go

Add four new flags alongside existing `--max-initial`, `--max-lazy`, `--max-total`:

```go
c.Flags().Int64Var(&maxInitialCSS,    "max-initial-css",    0, "Max total initial CSS bundle size in bytes (0 = unlimited)")
c.Flags().Int64Var(&maxTotalCSS,      "max-total-css",      0, "Max total CSS bundle size in bytes (0 = unlimited)")
c.Flags().Int64Var(&maxInitialAssets, "max-initial-assets", 0, "Max total initial file-loader asset size in bytes (0 = unlimited)")
c.Flags().Int64Var(&maxTotalAssets,   "max-total-assets",   0, "Max total file-loader asset size in bytes (0 = unlimited)")
```

Wire each to the corresponding `Budgets` field before calling `budget.Evaluate`.

---

## 6. Report Changes

### 6.1 internal/report/text.go — TextWithOptions()

After the existing JS total lines, add asset rows **only when non-zero**:

```go
if result.Summary.TotalCSS > 0 {
    fmt.Fprintf(w, "  Initial CSS   %s
", formatBytes(result.Summary.InitialCSS))
    fmt.Fprintf(w, "  Total CSS     %s
", formatBytes(result.Summary.TotalCSS))
}
if result.Summary.TotalAssets > 0 {
    fmt.Fprintf(w, "  Initial Assets %s
", formatBytes(result.Summary.InitialAssets))
    fmt.Fprintf(w, "  Total Assets   %s
", formatBytes(result.Summary.TotalAssets))
}
```

### 6.2 internal/report/markdown.go and internal/report/github_pr.go

In the comparison table builder, add CSS and asset rows when either snapshot has non-zero asset totals:

```go
hasAssets := before.Summary.TotalCSS > 0 || after.Summary.TotalCSS > 0 ||
             before.Summary.TotalAssets > 0 || after.Summary.TotalAssets > 0

if hasAssets {
    // emit | Initial CSS | before | after | delta |
    // emit | Initial Assets | before | after | delta | with warning badge if regression
}
```

Budget violations on asset metrics (`"initialCss"`, `"totalCss"`, `"initialAssets"`, `"totalAssets"`) get their own row in the violations table using the same `formatViolation` helper already used for JS violations.

### 6.3 internal/mcp/server.go

- Add `include_assets bool` (default `true`) parameter to `bundle_summary`, `bundle_check`, `bundle_measure`, `bundle_compare` tool schemas.
- When `include_assets` is `false`, zero out asset totals before serialising the response.
- Expose `initial_css_bytes`, `total_css_bytes`, `initial_asset_bytes`, `total_asset_bytes` in all JSON response objects.

---

## 7. TDD Implementation Order

Work through each step in order. **The test file for each step must be written and failing before any production code is written.**

### Step 1 — Snapshot model (foundation)

| Order | File | What |
|-------|------|------|
| TEST | `internal/snapshot/model_test.go` | `TestIsCSS`: table test `".css" -> true`, `".js" -> false`, `".CSS" -> true` (case insensitive), `"" -> false` |
| PROD | `internal/snapshot/model.go` | Add `IsCSS()`, `CSSPath`/`AssetPaths` on `BundleOutput`, four new `Totals` fields |

### Step 2 — Normalizer captures CSS and asset paths

| Order | File | What |
|-------|------|------|
| TEST | `internal/angular/normalize_test.go` | New table cases: metafile with `cssBundle` → `o.CSSPath` set; metafile with `file-loader` imports → `o.AssetPaths` populated; CSS output entry emitted with correct `Bytes`; existing tests must still pass |
| PROD | `internal/angular/normalize.go` | Set `o.CSSPath`, append to `o.AssetPaths`, emit CSS `BundleOutput` entries |

### Step 3 — Build loader propagates Initial flag to CSS

| Order | File | What |
|-------|------|------|
| TEST | `internal/build/load_test.go` | `TestAttachAssetInitialFlags`: synthetic `[]BundleOutput` with initial JS chunk referencing a CSS path → CSS output `Initial` becomes `true`; non-initial JS → CSS stays `false` |
| PROD | `internal/build/load.go` | Add `attachAssetInitialFlags()` function; call it after `graph.Classify` in `Load` and `LoadWithEntry` |

### Step 4 — Analysis accumulates CSS and asset totals

| Order | File | What |
|-------|------|------|
| TEST | `internal/analysis/summary_test.go` | New table cases: snapshot with CSS outputs → `InitialCSS`/`TotalCSS` correct; snapshot with asset outputs referenced via `AssetPaths` → `InitialAssets`/`TotalAssets` correct; asset deduplication: same asset path from two JS outputs counted once; existing JS-only cases unchanged |
| PROD | `internal/analysis/summary.go` | Add Pass 2 (CSS) and Pass 3 (assets) as shown in §4.4 |

### Step 5 — Budget enforcement for asset metrics

| Order | File | What |
|-------|------|------|
| TEST | `internal/budget/budget_test.go` | Four new table cases: `InitialCSSMax` exceeded → violation with `metric: "initialCss"`; same for `totalCss`, `initialAssets`, `totalAssets`; not exceeded → no violation; zero budget (unlimited) → no violation |
| PROD | `internal/budget/budget.go` | Add four fields to `Budgets`; add four comparison cases to `Evaluate` |
| TEST | `internal/config/config_test.go` | Parse YAML with `initial_css_max: 100KB` → `Budgets.InitialCSSMax == 102400`; all four keys; missing keys → zero (unlimited) |
| PROD | `internal/config/config.go` | Add four YAML keys |

### Step 6 — CLI flags

| Order | File | What |
|-------|------|------|
| TEST | `cmd/contracts_test.go` | Confirm new flags registered on `check` and `measure` (existing contract test pattern) |
| PROD | `cmd/check.go`, `cmd/measure.go` | Add four `--max-*` flags; wire to `Budgets` fields |

### Step 7 — Text report renders asset rows

| Order | File | What |
|-------|------|------|
| TEST | `internal/report/text_test.go` | Golden test: `AnalysisResult` with `InitialCSS=68KB, TotalCSS=68KB, InitialAssets=1.4MB, TotalAssets=2.1MB` → expected text output includes asset rows; `AnalysisResult` with all-zero asset fields → asset rows absent |
| PROD | `internal/report/text.go` | Add asset rows to `TextWithOptions` as shown in §6.1 |

### Step 8 — Markdown and GitHub PR report asset rows

| Order | File | What |
|-------|------|------|
| TEST | `internal/report/markdown_test.go`, `internal/report/github_pr_test.go` | Golden tests: comparison with before CSS=68KB / after CSS=74KB → asset row in table with delta; all-zero asset fields → no asset row; budget violation on `initialCss` → violation row present |
| PROD | `internal/report/markdown.go`, `internal/report/github_pr.go` | Add `hasAssets` guard and asset rows as shown in §6.2 |

### Step 9 — Golden contract tests

| Order | File | What |
|-------|------|------|
| TEST | `testdata/contracts/v1/summary-assets/` | Add fixture `stats.json` with CSS bundle and file-loader assets; add expected golden output for `text`, `json`, `markdown` |
| TEST | `testdata/contracts/v1/compare-assets/` | Before/after fixtures with asset delta |
| PROD | `cmd/contracts_test.go` | Register new contract test cases |

### Step 10 — MCP server

| Order | File | What |
|-------|------|------|
| TEST | `internal/mcp/server_test.go` | `bundle_summary` response includes `initial_css_bytes` when assets present; `include_assets: false` zeros asset fields |
| PROD | `internal/mcp/server.go` | Add parameter and response fields as shown in §6.3 |

---

## 8. Backwards Compatibility Guarantees

| Concern | Guarantee |
|---------|-----------|
| Existing `stats.json` without CSS outputs | All new fields zero → `omitempty` → not serialised → no JSON diff |
| Existing `.bundleradar.yml` without asset budget keys | Zero value = unlimited → no violations |
| Existing reports on JS-only snapshots | Asset rows suppressed by `TotalCSS == 0 && TotalAssets == 0` guard |
| Schema version | Stays `"1"` — additive only |
| Golden contracts | New test data in new subdirectories; existing contracts untouched |

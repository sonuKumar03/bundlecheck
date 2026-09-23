# Route-Scoped Bundle Summary — Design Spec (v2, audited)

**Date:** 2026-09-23
**Status:** Audited — ready for implementation planning
**Command:** `bundleradar summary --route /login`
**Approach:** Test-Driven Development (TDD)

---

## 0. TDD Contract

Every implementation step in this spec follows Red → Green → Refactor:

1. **Write a failing test** that defines the exact behaviour (unit, table-driven, or golden).
2. **Write the minimum code** to make that test pass.
3. **Refactor** without breaking the test.

Implementation order in §8 reflects this — test files precede the production files they exercise. No production code ships without a test.

---

## 1. Problem Statement

`bundleradar summary` shows what is in the **entire bundle**. A developer asking *"what does my `/login` page actually load?"* gets no actionable answer: they see a combined view of every lazy chunk across every route.

Today `--entry src/app/login/login.routes.ts` can scope the analysis, but the developer must already know which source file maps to `/login`. That mapping lives in Angular router configuration, not in the esbuild stats file.

**Goal:** `bundleradar summary --route /login` resolves the URL path to its Angular entry file, then reports: which chunks load, which packages are in the route-specific chunk, and total bytes paid on navigation — split into route-specific vs always-loaded-initial.

---

## 2. The Core Design Problem: Route URL → Bundle Entry Mapping

The esbuild metafile records `entryPoint` as a **source file path** (e.g. `src/app/login/login.routes.ts`). It does not record URL route strings.

The mapping `"/login" → "src/app/login/login.routes.ts"` exists only in Angular router source code.

### 2.1 Three Resolution Strategies (priority order, first match wins)

**Strategy 1 — Config map** (explicit, authoritative):
```yaml
# .bundleradar.yml
routes:
  /login:     src/app/login/login.routes.ts
  /admin:     src/app/admin/admin.routes.ts
  /dashboard: src/app/dashboard/dashboard.component.ts
```

**Strategy 2 — Source heuristic** (zero config, best-effort):

Regex-scan `*.routes.ts` and `*-routing.module.ts` files found under `SourceRoot` (defaulting to CWD) for three Angular lazy-load patterns:

```
Pattern A (loadChildren + then):
  path:\s*['"]([\w/-]+)['"]\s*,\s*loadChildren:\s*\(\)\s*=>\s*import\(['"](\.\/[^'"]+)['"]\)

Pattern B (loadComponent):
  path:\s*['"]([\w/-]+)['"]\s*,\s*loadComponent:\s*\(\)\s*=>\s*import\(['"](\.\/[^'"]+)['"]\)

Pattern C (NgModule loadChildren):
  path:\s*['"]([\w/-]+)['"]\s*,\s*loadChildren:\s*\(\)\s*=>\s*import\(['"](\.\/[^'"]+)['"]\)\.then
```

For each match: resolve the import path relative to the `.routes.ts` file location, append `.ts` if no extension, apply `snapshot.CleanPath`. This gives the canonical source entry path.

**Limitations acknowledged:** Does not handle computed paths (`path: env.loginPath`), aliased imports, or multi-level nested children. These are rare in practice.

**Strategy 3 — Fuzzy entry match** (fallback):

Find all `BundleOutput.EntryPoint` values whose last path segment contains the route's last URL segment (case-insensitive). If exactly one match: use it with a warning. If multiple: return an `Ambiguous` error listing candidates.

### 2.2 SourceRoot Discovery

`Resolver.SourceRoot` is populated in this order:
1. The `project.source_root` key in `.bundleradar.yml` if set.
2. The directory of the resolved `stats.json` file (i.e. the workspace root where `ng build` ran).
3. CWD as final fallback.

Scanner walks `SourceRoot` recursively via `filepath.WalkDir`, collecting files matching `*.routes.ts` or `*-routing.module.ts`, skipping `node_modules/` and `dist/` directories.

### 2.3 Shorthand `-r` Availability

`-r` is currently unassigned on `summary`. Assign it to `--route`. Confirm: `cmd/summary.go` has no `-r` flag today ✓.

---

## 3. Feature Design

### 3.1 CLI Interface

```
bundleradar summary --route /login [stats.json] [dist]
bundleradar summary --route /admin/users -f markdown
bundleradar summary --route /login --gzip --top 20
```

**Flag definition:**

```go
c.Flags().StringVarP(&routePath, "route", "r", "", "URL path to scope analysis to (e.g. /login)")
```

**Mutual exclusion with `--entry`:**

```go
if routePath != "" && entry != "" {
    return &UsageError{Err: fmt.Errorf("--route and --entry are mutually exclusive")}
}
```

**Format support:** `text`, `json`, `markdown`. The `github-pr` format is **not supported** for `--route` (route summary is not a CI comparison report). Attempting it returns an error: `"--format github-pr is not supported with --route; use markdown instead"`.

### 3.2 Output

#### Text format

```
ROUTE SUMMARY — /login  (portal)
Resolved entry: src/app/login/login.routes.ts  [source-heuristic]

CHUNKS LOADED ON NAVIGATION TO /login
  chunk-A1B2C3D4.js    142 KB  [route-specific]
  main.DEADBEEF.js     342 KB  [initial — always loaded]
  polyfills.98765.js    18 KB  [initial — always loaded]

TOP PACKAGES IN ROUTE CHUNK (142 KB)
  @angular/forms        38 KB  (26%)
  @angular/animations   22 KB  (15%)
  src/app/login         31 KB  (21%)

NAVIGATION COST: /login
  Route-specific:   142 KB
  Initial (shared): 360 KB
  Total:            502 KB
```

#### JSON format

```json
{
  "schemaVersion": "1",
  "toolVersion": "0.7.0",
  "command": "route-summary",
  "route": "/login",
  "resolvedEntry": "src/app/login/login.routes.ts",
  "resolutionMethod": "source-heuristic",
  "routeChunks": [
    {
      "path": "browser/chunk-A1B2C3D4.js",
      "bytes": 145408,
      "gzipBytes": 47000,
      "initial": false,
      "entryPoint": "src/app/login/login.routes.ts"
    }
  ],
  "initialChunks": [
    { "path": "browser/main.DEADBEEF.js", "bytes": 350208, "initial": true, "entryPoint": "src/main.ts" }
  ],
  "packages": [
    { "name": "@angular/forms", "bytes": 38912 },
    { "name": "@angular/animations", "bytes": 22528 }
  ],
  "modules": [
    { "name": "src/app/login/login.component.ts", "bytes": 8192 }
  ],
  "totals": {
    "routeOnlyBytes": 145408,
    "initialBytes": 368640,
    "totalOnNavigation": 514048
  }
}
```

#### Markdown format

```markdown
## Route Bundle: `/login`

> **Resolved entry:** `src/app/login/login.routes.ts` *(source-heuristic)*

### Chunks Loaded on Navigation

| Chunk | Size | Type |
|-------|------|------|
| `chunk-A1B2C3D4.js` | 142 KB | Route-specific |
| `main.DEADBEEF.js` | 342 KB | Initial (always) |
| `polyfills.js` | 18 KB | Initial (always) |

### Top Packages in Route Chunk

| Package | Size | % of route chunk |
|---------|------|-----------------|
| `@angular/forms` | 38 KB | 26% |
| `@angular/animations` | 22 KB | 15% |
| `src/app/login` | 31 KB | 21% |

### Navigation Cost

| | Bytes |
|-|-------|
| Route-specific | 142 KB |
| Initial (shared) | 360 KB |
| **Total on navigation** | **502 KB** |
```

---

## 4. New Package: internal/routemap

### 4.1 Types and Interface

```go
package routemap

import (
    "path/filepath"
    "github.com/sonuKumar03/bundleradar/internal/snapshot"
)

// RouteEntry is the result of resolving a URL path to a source entry file.
type RouteEntry struct {
    Route      string   // "/login" — the original URL path
    EntryPath  string   // "src/app/login/login.routes.ts" — canonical source entry
    Method     string   // "config" | "source-heuristic" | "fuzzy"
    Ambiguous  bool     // true when fuzzy found multiple candidates
    Candidates []string // non-empty when Ambiguous
}

// Resolver resolves URL route paths to Angular source entry files.
type Resolver struct {
    // ConfigRoutes: explicit map from .bundleradar.yml routes:
    ConfigRoutes map[string]string
    // SourceRoot: root directory to scan for *.routes.ts files (heuristic)
    SourceRoot string
    // Outputs: the snapshot outputs for fuzzy matching
    Outputs []snapshot.BundleOutput
}

// Resolve maps a URL path to the best-matching source entry.
// Returns an error if the route cannot be resolved unambiguously.
func (r *Resolver) Resolve(route string) (*RouteEntry, error)
```

### 4.2 internal/routemap/parser.go — Source Heuristic Scanner

```go
// ScanRoutes walks root (skipping node_modules/ and dist/) and extracts
// route-path -> import-path pairs from Angular lazy route patterns.
// Returns a map from normalised URL path (e.g. "login") to resolved
// canonical source file path (e.g. "src/app/login/login.routes.ts").
func ScanRoutes(root string) (map[string]string, error)
```

**Implementation detail:**

1. Walk `root` via `filepath.WalkDir`. Skip `node_modules` and `dist` dirs.
2. Collect files matching `*.routes.ts` or `*-routing.module.ts`.
3. For each file, read its content and apply three compiled regexes (one per pattern).
4. For each match: URL segment = submatch[1], import fragment = submatch[2].
5. Resolve the import fragment relative to the routes file's directory:
   - Strip leading `./`
   - If no extension: append `.ts`
   - Join with the routes file's dir: `filepath.Join(dir, importFragment)`
   - Apply `snapshot.CleanPath`
6. Return `map[urlSegment]resolvedSourcePath`.

**Regex constants** (compiled once at package init):

```go
var (
    reLoadChildren = regexp.MustCompile(
        `path:\s*['"]([\w/.-]+)['"]\s*,\s*loadChildren:\s*\(\)\s*=>\s*import\(['"](\./[^'"]+)['"]\)`)
    reLoadComponent = regexp.MustCompile(
        `path:\s*['"]([\w/.-]+)['"]\s*,\s*loadComponent:\s*\(\)\s*=>\s*import\(['"](\./[^'"]+)['"]\)`)
)
```

Both patterns cover NgModule and standalone component forms — the `.then(m => m.X)` suffix is optional and not captured.

### 4.3 Resolve() algorithm

```
1. Normalise route: strip trailing slash, lowercase, ensure leading slash.
2. Config lookup: if ConfigRoutes[route] exists → return RouteEntry{Method: "config"}.
3. Source heuristic:
   a. Call ScanRoutes(SourceRoot) → routeMap.
   b. Strip leading "/" from route to get segment ("login").
   c. Lookup routeMap[segment] → if found → return RouteEntry{Method: "source-heuristic"}.
   d. Try nested path: for "/admin/users", try segment "admin/users" then "users".
4. Fuzzy:
   a. Collect all BundleOutput.EntryPoint values.
   b. lastSeg = last URL segment (after final "/").
   c. Filter EntryPoints where filepath.Base(entryPoint) contains lastSeg (case-insensitive).
   d. If exactly 1 match → return RouteEntry{Method: "fuzzy"} with warning.
   e. If 0 matches → error: route not found.
   f. If >1 matches → return RouteEntry{Ambiguous: true, Candidates: [...], Method: "fuzzy"}.
```

---

## 5. Analysis: internal/analysis/route.go

### 5.1 Types

```go
type RouteSummaryResult struct {
    SchemaVersion    string         `json:"schemaVersion"`
    ToolVersion      string         `json:"toolVersion"`
    Command          string         `json:"command"`          // "route-summary"
    Route            string         `json:"route"`
    ResolvedEntry    string         `json:"resolvedEntry"`
    ResolutionMethod string         `json:"resolutionMethod"`
    RouteChunks      []ChunkSummary `json:"routeChunks"`
    InitialChunks    []ChunkSummary `json:"initialChunks"`
    Packages         []Contributor  `json:"packages"`  // from route chunks only
    Modules          []Contributor  `json:"modules"`   // from route chunks only
    Totals           RouteTotals    `json:"totals"`
}

type ChunkSummary struct {
    Path       string `json:"path"`
    Bytes      int64  `json:"bytes"`
    GzipBytes  int64  `json:"gzipBytes,omitempty"`
    Initial    bool   `json:"initial"`
    EntryPoint string `json:"entryPoint,omitempty"`
}

type RouteTotals struct {
    RouteOnlyBytes    int64 `json:"routeOnlyBytes"`
    InitialBytes      int64 `json:"initialBytes"`
    TotalOnNavigation int64 `json:"totalOnNavigation"`
}
```

### 5.2 RouteAnalyze() algorithm

```go
func RouteAnalyze(s *snapshot.BundleSnapshot, resolvedEntry string, route string, method string) (*RouteSummaryResult, error)
```

Steps:
1. **Find the root output** for `resolvedEntry`: iterate `s.Outputs`, match `o.EntryPoint` against `resolvedEntry` using `artifact.MatchEntryOutputs` logic (exact, path.Match, basename). If no match → error.
2. **Collect route chunks** via BFS on the output-level dynamic import graph:
   - Build `outputByPath map[string]*BundleOutput` index.
   - Build `dynamicEdges map[string][]string` from `o.Imports` where `imp.Dynamic && !imp.External && IsJavaScript(imp.Path)`.
   - BFS from the root output's path; collect all reachable outputs (including root). These are `RouteChunks`.
3. **Collect initial chunks**: all `o.Initial == true` outputs (may overlap with route chunks if route resolves to an initial entry).
4. **Accumulate packages and modules** from `RouteChunks` only, using the same dedup logic as `InspectChunk`.
5. **Compute totals**:
   - `RouteOnlyBytes` = sum of bytes in `RouteChunks` that are NOT initial.
   - `InitialBytes` = sum of bytes in `InitialChunks`.
   - `TotalOnNavigation` = `RouteOnlyBytes + InitialBytes`.
6. Sort `RouteChunks` by bytes descending, `InitialChunks` by bytes descending.
7. Sort `Packages` and `Modules` by bytes descending (reuse `sortContributors`).

---

## 6. Report Renderers

### 6.1 internal/report/route_text.go

```go
// RouteText writes a text-format route summary to w.
func RouteText(w io.Writer, r *analysis.RouteSummaryResult, opts TextOptions) error
```

Writes the text output shown in §3.2. `opts.Top` controls the number of packages shown (default 10). `opts.Gzip` shows gzip bytes if present.

### 6.2 internal/report/route_markdown.go

```go
// RouteMarkdown writes a markdown-format route summary to w.
func RouteMarkdown(w io.Writer, r *analysis.RouteSummaryResult, opts TextOptions) error
```

Writes the markdown output shown in §3.2. `github-pr` is not a valid format for route summary — rejected at the CLI layer before this function is called.

---

## 7. Modified Files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Routes map[string]string` field to `Config`; parse `routes:` YAML map |
| `internal/config/config_test.go` | Test `routes:` key parsing; missing key → empty map (no error) |
| `cmd/summary.go` | Add `--route` / `-r` flag; mutual exclusion check with `--entry`; wire to `routemap.Resolver` + `RouteAnalyze` + `RouteText`/`RouteMarkdown`; reject `github-pr` format with `--route` |
| `cmd/contracts_test.go` | Add golden contract for `summary --route` |
| `.agents/skills/bundleradar/SKILL.md` | Document `--route` flag and usage |
| `docs/index.html` | Add route summary example |
| `README.md` | Add `--route` to summary section |

---

## 8. TDD Implementation Order

**Write test first, then minimum production code. Each step must be independently compilable before moving to the next.**

### Step 1 — routemap: config resolution

| Order | File | What |
|-------|------|------|
| TEST | `internal/routemap/routemap_test.go` | `TestResolveConfig`: table tests — `/login` in config → `RouteEntry{Method:"config", EntryPath:"src/app/login/login.routes.ts"}`; route not in config → falls through (heuristic/fuzzy called with nil SourceRoot → returns not-found error) |
| PROD | `internal/routemap/routemap.go` | `Resolver` struct; `Resolve()` with Strategy 1 (config lookup) only; stub heuristic and fuzzy to return not-found |

### Step 2 — routemap: source heuristic parser

| Order | File | What |
|-------|------|------|
| TEST | `internal/routemap/parser_test.go` | `TestScanRoutes`: synthetic `.routes.ts` file content with all three patterns; verify `routeMap["login"] == "src/app/login/login.routes.ts"` (path normalisation tested); `node_modules` dir skipped; empty file → empty map |
| PROD | `internal/routemap/parser.go` | `ScanRoutes(root string)` with WalkDir, regex extraction, path normalisation |

### Step 3 — routemap: heuristic + fuzzy integration

| Order | File | What |
|-------|------|------|
| TEST | `internal/routemap/routemap_test.go` | `TestResolveHeuristic`: SourceRoot with fixture routes.ts → resolves `/login`; `TestResolveFuzzy`: outputs with EntryPoint containing "login" → resolves with warning; `TestResolveFuzzyAmbiguous`: two outputs contain "login" → `Ambiguous: true`; `TestResolveNotFound` → descriptive error |
| PROD | `internal/routemap/routemap.go` | Complete `Resolve()` with all three strategies |

### Step 4 — Config schema: routes key

| Order | File | What |
|-------|------|------|
| TEST | `internal/config/config_test.go` | Parse YAML with `routes: {/login: src/app/login/login.routes.ts}` → `Config.Routes["/login"] == "src/app/login/login.routes.ts"`; missing `routes:` key → `Config.Routes == nil` (no error) |
| PROD | `internal/config/config.go` | Add `Routes map[string]string` field |

### Step 5 — Analysis: RouteAnalyze

| Order | File | What |
|-------|------|------|
| TEST | `internal/analysis/route_test.go` | `TestRouteAnalyze_SingleLazyChunk`: snapshot with one initial chunk + one lazy chunk (entry=login.routes.ts) → `RouteChunks` has 1 entry, `InitialChunks` has 1 entry, `RouteOnlyBytes` = lazy chunk bytes, `TotalOnNavigation` = lazy + initial; `TestRouteAnalyze_InitialRoute`: route resolves to an initial chunk → `RouteOnlyBytes == 0`; `TestRouteAnalyze_EntryNotFound` → error; `TestRouteAnalyze_PackageAccumulation`: route chunk has known npm inputs → packages populated |
| PROD | `internal/analysis/route.go` | `RouteAnalyze()`, `RouteSummaryResult`, `ChunkSummary`, `RouteTotals` |

### Step 6 — Text renderer

| Order | File | What |
|-------|------|------|
| TEST | `internal/report/route_text_test.go` | Golden test: fixed `RouteSummaryResult` → expected text; zero route-specific bytes → note printed; `opts.Top` limits package rows |
| PROD | `internal/report/route_text.go` | `RouteText()` |

### Step 7 — Markdown renderer

| Order | File | What |
|-------|------|------|
| TEST | `internal/report/route_markdown_test.go` | Golden test: same `RouteSummaryResult` → expected markdown tables |
| PROD | `internal/report/route_markdown.go` | `RouteMarkdown()` |

### Step 8 — CLI wiring

| Order | File | What |
|-------|------|------|
| TEST | `cmd/contracts_test.go` | Confirm `--route` flag registered on `summary`; confirm `-r` shorthand; confirm `--route` + `--entry` together returns exit code 2 |
| PROD | `cmd/summary.go` | Add `--route` / `-r`; mutual exclusion; wire resolver + `RouteAnalyze` + renderers; reject `github-pr` with `--route` |

### Step 9 — Golden contract

| Order | File | What |
|-------|------|------|
| TEST | `testdata/contracts/v1/summary-route/` | Fixture `stats.json` + fixture `.routes.ts`; expected golden for `text`, `json`, `markdown` |
| PROD | `cmd/contracts_test.go` | Register new contract case |

---

## 9. Edge Cases & Error Handling

| Case | Behaviour |
|------|-----------|
| Route not resolved by any strategy | `UsageError`: `route "/login" could not be resolved; add it to .bundleradar.yml routes: or use --entry directly` |
| Fuzzy: multiple candidates | `UsageError` listing all candidates; exit code 2 |
| Route resolves to initial chunk | Works; `RouteOnlyBytes == 0`; text note: `"(this entry is in the initial bundle — loaded on every page)"` |
| Nested lazy children under `/admin` | Only direct route chunk BFS; nested dynamic children listed as `"+ N further lazy chunks (not included in route-specific total)"` |
| `--route` + `--entry` together | `UsageError`: `--route and --entry are mutually exclusive`; exit code 2 |
| `--format github-pr` + `--route` | `UsageError`: `--format github-pr is not supported with --route`; exit code 2 |
| `stats.json` entryPoint has no matching output | `ExecutionError`: `entry "..." matched no bundle outputs; the route may not have been emitted as a separate chunk` |
| ScanRoutes: permission denied on a subdirectory | Log warning; continue scanning; do not abort |

---

## 10. Backwards Compatibility

- `--route` is a new flag with no short conflict; no existing behaviour changes.
- Config `routes:` key is optional; existing configs parse without error (`nil` map).
- New JSON fields only appear under `"command": "route-summary"`; no existing command's output changes.
- No schema version bump.

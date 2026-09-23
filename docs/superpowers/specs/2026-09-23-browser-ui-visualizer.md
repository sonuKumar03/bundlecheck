# Browser UI Visualizer — Design Spec (v2: Live Server Edition)

**Date:** 2026-09-23  
**Status:** Draft — awaiting user review  
**Command:** `bundleradar view [stats.json] [dist]`  
**Approach:** Test-Driven Development (TDD)

---

## 0. TDD Contract

Every implementation step follows Red → Green → Refactor:

1. **Write a failing test** that defines the exact behaviour.
2. **Write the minimum code** to make that test pass.
3. **Refactor** without breaking the test.

The Go backend (data serializer, HTTP server, SSE, file watcher, embed) is fully TDD-covered. The frontend is tested via golden snapshot tests and integration tests. No production code ships before its test.

---

## 1. Problem Statement

`bundleradar` produces accurate, machine-readable data about Angular bundles — but only at a point in time. Developers running `ng build --watch` iterate fast and want a **live**, spatial view that updates automatically as each build completes — without switching terminal windows or refreshing a page.

Prior art gaps:
- `webpack-bundle-analyzer` — snapshot only, webpack-specific
- `esbuild-visualizer` / `rollup-visualizer` — no Angular layer, no live mode
- `source-map-explorer` — requires source maps, no live updates

`bundleradar view` fills this gap: an Angular-aware live visualizer that watches the build output, pushes updates over SSE, accumulates a build history timeline, and shows live budget violations — all in a zero-dependency, embedded, self-contained binary.

---

## 2. Scope

### v0.8 — In scope

**Core visualizer (static and live):**
- Three linked panels: **Treemap**, **Chunk List**, **Package Table**
- Drill-down: Bundle → Chunk → Package → Module
- Initial / Lazy / All toggle; Raw / Gzip size toggle
- Global search/filter by name (dims non-matches, preserves spatial context)
- Import chain panel (`why`-style trace) on package/module click
- Budget violation overlay (red badge + modal)
- Dark mode default; light mode toggle
- URL hash state — deep link to any view/filter/selection

**Live server (`bundleradar view --watch`):**
- File watcher: polls `stats.json` mtime every 500 ms (no `fsnotify` dependency)
- When stats.json changes: rebuild `UIPayload`, push SSE event `data: <payload-json>`
- Browser receives event, hot-swaps data without page reload
- "Building…" spinner while file is being written (detected via partial/locked file)
- Build history timeline: accumulates last 20 snapshots in memory; sparkline in header

**Advanced features:**
- **History timeline**: sparkline of Initial JS / Total JS over last N builds in current session
- **Live diff**: clicking any history point shows what changed from previous build
- **Multi-project dashboard**: `bundleradar view --workspace` shows all Nx projects in a single page with per-project panels
- **Shareable state**: all filters, selections, and view modes encoded in URL hash — copy/paste to share
- **Keyboard navigation**: full keyboard shortcuts for all interactions
- **Treemap export**: `Save as PNG` button (uses Canvas `toDataURL`)
- **CSV export**: Package Table exportable as CSV

**HTML export (static):**
- `bundleradar view --output report.html` — single self-contained file
- All JS/CSS inlined, payload embedded as `window.__BUNDLECHECK_PAYLOAD__`
- History snapshot included in export (last build only)

### v0.9 — Deferred
- Route-scoped overlay (`--route /login` in the UI) — depends on route spec
- Source map integration
- Comparison view between two named baselines (separate spec)
- Light/dark auto-detect from OS preference

---

## 3. Three Modes

```
┌──────────────────────────────────────────────────────────────────┐
│  Mode A: HTML Export     Mode B: Snapshot Server  Mode C: Live   │
│  ─────────────────────   ─────────────────────    ─────────────  │
│  bundleradar view        bundleradar view          bundleradar   │
│    --output out.html       (default)                 view --watch │
│                                                                   │
│  Writes single HTML      Serves current payload   Serves payload │
│  file. No server.        once. No file watching.  + SSE stream.  │
│  All data inlined.       Opens browser.           Live updates.  │
│                                                   Builds history. │
└──────────────────────────────────────────────────────────────────┘
               │                    │                   │
               └────────────────────┼───────────────────┘
                                    │
                    ┌───────────────▼────────────────┐
                    │  internal/ui/payload.go         │
                    │  BuildPayload(snapshot) →       │
                    │  UIPayload (pure Go, no HTTP)   │
                    └─────────────────────────────────┘
```

The frontend JS detects its mode at startup:
```js
const LIVE = !!window.__SSE_ENDPOINT__;  // set by server, absent in HTML export
const data = window.__BUNDLECHECK_PAYLOAD__ || await fetch('/api/payload').then(r=>r.json());
if (LIVE) subscribeSSE('/api/events');
```

---

## 4. UIPayload Data Contract

```go
// internal/ui/payload.go

type UIPayload struct {
    SchemaVersion string       `json:"schemaVersion"`  // "1"
    ToolVersion   string       `json:"toolVersion"`
    GeneratedAt   string       `json:"generatedAt"`    // RFC3339
    Project       string       `json:"project,omitempty"`
    BuildIndex    int          `json:"buildIndex"`     // 0-based; increments on each live reload
    Totals        UITotals     `json:"totals"`
    Chunks        []UIChunk    `json:"chunks"`
    Packages      []UIPackage  `json:"packages"`
    ImportGraph   UIGraph      `json:"importGraph"`
    Budgets       *UIBudgets   `json:"budgets,omitempty"`
    Violations    []UIViolation `json:"violations,omitempty"`
}

type UITotals struct {
    InitialJS     int64 `json:"initialJs"`
    LazyJS        int64 `json:"lazyJs"`
    TotalJS       int64 `json:"totalJs"`
    InitialGzipJS int64 `json:"initialGzipJs,omitempty"`
    TotalGzipJS   int64 `json:"totalGzipJs,omitempty"`
}

type UIChunk struct {
    ID          string          `json:"id"`           // stable: CleanPath basename without hash
    Path        string          `json:"path"`
    Bytes       int64           `json:"bytes"`
    GzipBytes   int64           `json:"gzipBytes,omitempty"`
    Initial     bool            `json:"initial"`
    EntryPoint  string          `json:"entryPoint,omitempty"`
    Modules     []UIModule      `json:"modules"`
    Imports     []UIChunkImport `json:"imports"`      // edges to other chunks
}

type UIModule struct {
    Path    string `json:"path"`
    Bytes   int64  `json:"bytes"`
    Package string `json:"package,omitempty"`
}

type UIChunkImport struct {
    ChunkID string `json:"chunkId"`
    Dynamic bool   `json:"dynamic"`
}

type UIPackage struct {
    Name         string   `json:"name"`
    InitialBytes int64    `json:"initialBytes"`
    LazyBytes    int64    `json:"lazyBytes"`
    TotalBytes   int64    `json:"totalBytes"`
    GzipBytes    int64    `json:"gzipBytes,omitempty"`
    ChunkIDs     []string `json:"chunkIds"`
}

type UIGraph struct {
    Nodes []UIGraphNode `json:"nodes"`
    Edges []UIGraphEdge `json:"edges"`
}

type UIGraphNode struct {
    ID    string `json:"id"`    // module path
    Bytes int64  `json:"bytes"`
    Kind  string `json:"kind"`  // "source" | "package" | "chunk"
}

type UIGraphEdge struct {
    From    string `json:"from"`
    To      string `json:"to"`
    Dynamic bool   `json:"dynamic"`
}

type UIBudgets struct {
    InitialJSMax int64 `json:"initialJsMax,omitempty"`
    TotalJSMax   int64 `json:"totalJsMax,omitempty"`
}

type UIViolation struct {
    Metric  string `json:"metric"`
    Actual  int64  `json:"actual"`
    Budget  int64  `json:"budget"`
    Exceeds int64  `json:"exceeds"`
}

// UIHistoryPoint is a compact record of one past build for the timeline sparkline.
type UIHistoryPoint struct {
    BuildIndex int    `json:"buildIndex"`
    GeneratedAt string `json:"generatedAt"`
    InitialJS  int64  `json:"initialJs"`
    TotalJS    int64  `json:"totalJs"`
    HasViolation bool `json:"hasViolation"`
}
```

`BuildPayload(s *snapshot.BundleSnapshot, result *analysis.AnalysisResult, opts PayloadOptions) (*UIPayload, error)` — pure function; no HTTP, no file I/O.

---

## 5. Live Server: File Watcher + SSE

### 5.1 File Watcher — polling, no external deps

```go
// internal/ui/watcher.go

// Watcher polls a file path at interval and calls onChange when mtime changes.
// Uses time.Ticker — zero new dependencies.
type Watcher struct {
    Path     string
    Interval time.Duration // default 500ms
    onChange func(path string)
}

func (w *Watcher) Start(ctx context.Context)
func (w *Watcher) Stop()
```

**Why polling, not `fsnotify`:**  
- `fsnotify` is a heavy dependency (kqueue/inotify/ReadDirectoryChangesW per-platform).
- Angular's `ng build --watch` rewrites `stats.json` atomically (temp file → rename). Polling at 500ms catches every build within half a second — imperceptible for a dev workflow.
- Polling also works correctly inside Docker volumes where inotify is unreliable.

**Detection of in-progress writes:**  
When `ng build --watch` is writing, `stats.json` may be partially written. After mtime change is detected, the watcher:
1. Attempts to parse the file. If parse fails → emit a `{"type":"building"}` SSE event (spinner shown in UI).
2. Retries after 200ms up to 5 times.
3. On successful parse → emit `{"type":"update","payload":{...}}` SSE event.

### 5.2 Server-Sent Events (SSE) — no WebSocket

SSE is simpler than WebSocket for this use case: unidirectional server → browser push, native browser `EventSource` API, automatic reconnection.

#### SSE endpoint: `GET /api/events`

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: {"type":"connected","buildIndex":0}

data: {"type":"building"}

data: {"type":"update","payload":{...UIPayload...},"history":[...UIHistoryPoint...]}

data: {"type":"violation","violations":[...UIViolation...]}
```

**SSE event types:**

| Event `type` | When | Payload |
|---|---|---|
| `connected` | On first connection | `buildIndex`, current payload |
| `building` | Stats file changed but parse failed | — |
| `update` | Successful rebuild | Full `UIPayload` + `history []UIHistoryPoint` |
| `violation` | Budget violated on rebuild | `violations []UIViolation` |
| `ping` | Every 30s (keepalive) | — |

**Multiple browser tabs:** The server holds a `sync.Map` of SSE response writers. When a new payload is built, it broadcasts to all connected clients.

### 5.3 Build History in Memory

```go
// internal/ui/history.go

type History struct {
    mu     sync.Mutex
    points []UIHistoryPoint
    max    int // default 50
}

func (h *History) Add(p *UIPayload)
func (h *History) Points() []UIHistoryPoint
func (h *History) Diff(indexA, indexB int) (*UIDiff, error)
```

The server holds one `History` instance. Every successful rebuild appends a `UIHistoryPoint`. The last 50 points are kept (ring buffer). `SSE "update"` events include the current history array for the sparkline.

---

## 6. Live Diff Between Builds

When the user clicks a history sparkline point, the UI requests a diff:

```
GET /api/diff?from=2&to=5
```

Response: `UIDiff` — what changed between build index 2 and 5:

```go
type UIDiff struct {
    From       int           `json:"from"`
    To         int           `json:"to"`
    DeltaInitialJS int64     `json:"deltaInitialJs"`
    DeltaTotalJS   int64     `json:"deltaTotalJs"`
    GrowedChunks   []UIDiffChunk `json:"growedChunks"`
    ShrunkenChunks []UIDiffChunk `json:"shrunkenChunks"`
    NewChunks      []UIDiffChunk `json:"newChunks"`
    RemovedChunks  []UIDiffChunk `json:"removedChunks"`
    GrowedPackages []UIDiffPkg   `json:"growedPackages"`
    NewPackages    []UIDiffPkg   `json:"newPackages"`
}

type UIDiffChunk struct {
    ID    string `json:"id"`
    From  int64  `json:"from"`
    To    int64  `json:"to"`
    Delta int64  `json:"delta"`
}

type UIDiffPkg struct {
    Name  string `json:"name"`
    From  int64  `json:"from"`
    To    int64  `json:"to"`
    Delta int64  `json:"delta"`
}
```

The server stores full payloads (not just history points) for diffing. It keeps the last 50 full payloads — memory cost is ~2–5 MB per payload × 50 = ~100–250 MB worst case. A `--history-limit` flag controls this (default 20 to be conservative).

---

## 7. Multi-Project Workspace Mode

```
bundleradar view --workspace [nx-root]
```

Discovers all Nx applications (reusing `internal/workspace`), loads each project's `stats.json`, and builds a `UIWorkspacePayload`:

```go
type UIWorkspacePayload struct {
    Projects []UIProjectSummary `json:"projects"`
}

type UIProjectSummary struct {
    Name       string   `json:"name"`
    InitialJS  int64    `json:"initialJs"`
    TotalJS    int64    `json:"totalJs"`
    Violations []UIViolation `json:"violations,omitempty"`
    PayloadURL string   `json:"payloadUrl"` // /api/project/<name>/payload
}
```

The UI renders a **workspace overview page**: a grid of project cards with mini-treemaps. Clicking a card switches to the full single-project view (fetches `/api/project/<name>/payload`).

In `--watch` mode, the workspace watcher polls all project stats files and pushes SSE events per-project.

---

## 8. Advanced Frontend Features

### 8.1 Full UI layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  ◉ bundleradar   portal  │ Initial 342KB │ Total 1.6MB │ ⚠ 1 budget │
│  [Treemap][Chunks][Pkgs] │ ○Raw ●Gzip   │ [🔍 filter] │ [⚙][↓CSV]  │
├────────────────────────────────────────────┬─────────────────────────┤
│                                            │  DETAIL PANEL           │
│  MAIN PANEL                                │                         │
│  (Treemap — canvas squarified layout)      │  ● @angular/forms       │
│                                            │    38 KB (initial)      │
│  ┌──────────────────────────────────────┐  │                         │
│  │ main.ABC.js  342KB  [initial]        │  │  Import chain:          │
│  │ ┌──────────┐ ┌────┐ ┌──────┐        │  │  main.ts                │
│  │ │@ang/core │ │rxjs│ │ src/ │        │  │  └─ app.config.ts       │
│  │ │  98 KB   │ │44KB│ │ ...  │        │  │     └─ @angular/forms   │
│  │ └──────────┘ └────┘ └──────┘        │  │                         │
│  └──────────────────────────────────────┘  │  [Highlight in treemap] │
│  chunk-A1B2.js   142KB  [lazy]             │  [Copy import path]     │
│  chunk-C3D4.js    28KB  [lazy]             │                         │
│                                            │  Appears in:            │
├────────────────────────────────────────────│  ● main.ABC.js  38 KB   │
│  TIMELINE (--watch mode)                   │  ○ chunk-A.js   2 KB    │
│  Build #12  ▂▃▃▄▄▄▅▅▆▆  342KB (+0%)       │                         │
│  [#9][#10][#11][●#12]   < click to diff >  │                         │
└────────────────────────────────────────────┴─────────────────────────┘
```

### 8.2 Keyboard shortcuts

| Key | Action |
|-----|--------|
| `T` | Switch to Treemap view |
| `C` | Switch to Chunk List view |
| `P` | Switch to Package Table view |
| `G` | Toggle Gzip / Raw |
| `I` | Toggle Initial / All / Lazy |
| `/` | Focus search input |
| `Esc` | Clear search; close detail panel |
| `←` `→` | Navigate history timeline |
| `S` | Save treemap as PNG |
| `X` | Export package table as CSV |
| `D` | Toggle dark / light mode |
| `?` | Show keyboard shortcut help |

### 8.3 URL hash state

All view state is encoded in the URL hash so any view is shareable:

```
#view=treemap&filter=angular&gzip=1&chunk=main.ABC.js&pkg=@angular/forms
```

On load, the app reads the hash and restores state. History `pushState` is called on every user interaction so the browser back button works.

### 8.4 Treemap: Canvas squarified layout

- Squarified treemap algorithm in ~120 lines of vanilla JS (Bruls et al. 2000)
- Rendered on `<canvas>` — 60fps smooth zoom animations via `requestAnimationFrame`
- Click chunk → zoom in; click background → zoom out
- Colour scheme:
  - Initial chunks: `hsl(220, 70%, 45%)` (blue)
  - Lazy chunks: `hsl(150, 55%, 38%)` (green)
  - Violated chunks: `hsl(0, 70%, 45%)` (red)
  - Hovered: +15% lightness
- Labels: chunk name + size; truncated with ellipsis when too small
- Tooltip on hover: full path, bytes, gzip bytes, % of total, package count
- "Save as PNG" calls `canvas.toDataURL('image/png')` → download link

### 8.5 Build status indicator (live mode only)

Shown in the header when `--watch` active:

```
● LIVE  Build #12  342 KB  (⬆ +6 KB from #11)  [updated 2s ago]
```

States:
- `● LIVE` (green pulsing dot) — connected, showing current build
- `◌ Building…` (spinner) — SSE `building` event received
- `✓ Up to date` — no changes since last build
- `⚠ Disconnected` (amber) — SSE connection dropped; auto-reconnects

### 8.6 Import Chain Trace Panel

When a package is selected:
1. Server receives `GET /api/why?pkg=@angular/forms&chunk=main.ABC.js`
2. Server calls `graph.TracePackage()` — reuses existing graph package
3. Returns `WhyResult` JSON (same type as `bundleradar why`)
4. Frontend renders the chain as a clickable vertical tree

This keeps the import graph query on-demand (not embedded in payload) to keep payload size manageable for apps with thousands of modules.

### 8.7 CSV / PNG Export

**Package Table → CSV:**
```
Name,Initial Bytes,Lazy Bytes,Total Bytes,Chunks
@angular/core,98304,0,98304,main.ABC.js
rxjs,45056,12288,57344,"main.ABC.js,chunk-A1B2.js"
```

Triggered by `X` key or Export button. Uses `Blob` + `URL.createObjectURL` for download — no server round-trip.

**Treemap → PNG:**
`canvas.toDataURL('image/png')` → download `bundleradar-<project>-<timestamp>.png`.

---

## 9. HTTP Server — Full API

```go
// internal/ui/server.go

type Server struct {
    Stats   string            // path to stats.json
    Dist    string            // path to dist dir
    Entry   string            // optional entry filter
    Gzip    bool
    Watch   bool              // enable file watcher + SSE
    Port    int               // 0 = auto-assign
    Open    bool              // open browser on start
    History *History          // live build history (non-nil when Watch=true)
    Budget  *config.Config    // for violation overlay; nil = no budgets
    Project string
}

func (s *Server) Start(ctx context.Context) (port int, err error)
```

### Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /` | GET | Serves `index.html` from embedded FS |
| `GET /static/*` | GET | Serves `app.js`, `style.css`, `icons.svg` |
| `GET /api/payload` | GET | Current `UIPayload` JSON |
| `GET /api/events` | GET | SSE stream (`--watch` only; 405 otherwise) |
| `GET /api/history` | GET | `[]UIHistoryPoint` (--watch only) |
| `GET /api/diff?from=N&to=M` | GET | `UIDiff` between two build indices |
| `GET /api/why?pkg=X&chunk=Y` | GET | `WhyResult` JSON (import chain trace) |
| `GET /api/workspace` | GET | `UIWorkspacePayload` (`--workspace` only) |
| `GET /api/project/:name/payload` | GET | Per-project payload (`--workspace` only) |

All API endpoints return `Content-Type: application/json`. All are bound to `127.0.0.1` only — not accessible from other machines.

---

## 10. CLI Command: `bundleradar view`

```
bundleradar view [stats.json] [dist]

Description:
  Launch an interactive browser UI to explore bundle composition.
  Defaults to snapshot mode (serves once). Use --watch to enable live
  reload as Angular rebuilds.

Flags:
  -s, --stats string        Path to stats.json (auto-detected if omitted)
  -d, --dist string         Path to browser dist (auto-detected if omitted)
  -p, --project string      Project name for multi-project workspaces
  -e, --entry string        Scope to specific entry point
  -g, --gzip                Show gzip sizes in the visualizer
  -o, --output string       Write self-contained HTML to file; do not serve
  -w, --watch               Enable live reload (polls stats.json every 500ms)
      --workspace           Show all Nx projects in workspace dashboard
      --port int            Dev server port (default: auto-assign)
      --no-open             Do not open browser automatically
      --history-limit int   Max builds to keep in history (default: 20)
      --poll-interval dur   Stats file poll interval (default: 500ms)
```

**Mode selection logic:**

```
if --output:           WriteHTML() → exit
if --workspace:        WorkspaceServer.Start()
if --watch:            Server{Watch: true}.Start()
else:                  Server{Watch: false}.Start()
```

**stderr output (not polluting stdout):**

```
bundleradar view: serving on http://127.0.0.1:54321
bundleradar view: watching /path/to/stats.json (every 500ms)
bundleradar view: build #12 detected — 342 KB initial (+6 KB)
bundleradar view: budget violation: initialJs 342 KB > 300 KB limit
bundleradar view: Ctrl-C to stop
```

---

## 11. Package Structure

### New files:

| File | Purpose |
|------|---------|
| `internal/ui/payload.go` | `UIPayload` types; `BuildPayload()` |
| `internal/ui/payload_test.go` | Unit + table tests |
| `internal/ui/server.go` | HTTP handlers; `WriteHTML()`; `Start()` |
| `internal/ui/server_test.go` | Handler tests; HTML export golden |
| `internal/ui/watcher.go` | Polling file watcher |
| `internal/ui/watcher_test.go` | Watcher: mtime detection; in-progress write retry |
| `internal/ui/history.go` | `History` ring buffer; `Diff()` |
| `internal/ui/history_test.go` | Add/Points/Diff correctness |
| `internal/ui/sse.go` | SSE broadcaster; client registry |
| `internal/ui/sse_test.go` | SSE: multi-client broadcast; ping; disconnect |
| `internal/ui/embed.go` | `//go:embed frontend/*` |
| `internal/ui/frontend/index.html` | App shell (`<title>`, meta, grid) |
| `internal/ui/frontend/app.js` | Full UI: treemap, panels, SSE, history, shortcuts (~700 lines) |
| `internal/ui/frontend/style.css` | Dark + light theme, layout (~350 lines) |
| `internal/ui/frontend/icons.svg` | SVG sprite |
| `cmd/view.go` | CLI command |
| `cmd/view_test.go` | CLI integration tests |

### Modified files:

| File | Change |
|------|--------|
| `cmd/root.go` | Register `viewCommand()` |
| `cmd/contracts_test.go` | Add `view` to expected command list |
| `docs_contract_test.go` | Add `view` to expected documented commands |
| `docs/index.html` | Document `view` command |
| `README.md` | Add `view` to command table |
| `.agents/skills/bundleradar/SKILL.md` | Add `view` usage |

---

## 12. TDD Implementation Order

### Step 1 — UIPayload: BuildPayload

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/payload_test.go` | `TestBuildPayload_Basic`: 2 chunks + 3 pkgs → correct `UIChunk.Modules`, `UIPackage.ChunkIDs`, `UITotals` |
| TEST | `internal/ui/payload_test.go` | `TestBuildPayload_ImportGraph`: static + dynamic imports → `UIGraph.Edges` with correct `Dynamic` flag |
| TEST | `internal/ui/payload_test.go` | `TestBuildPayload_Violations`: snapshot exceeding budget → `UIPayload.Violations` populated |
| TEST | `internal/ui/payload_test.go` | `TestBuildPayload_GzipAbsent`: no gzip data → all `GzipBytes` == 0 |
| TEST | `internal/ui/payload_test.go` | `TestBuildPayload_NilSnapshot` → error |
| PROD | `internal/ui/payload.go` | `UIPayload` types + `BuildPayload()` |

### Step 2 — File watcher (polling)

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/watcher_test.go` | `TestWatcher_DetectsMtimeChange`: write file → update mtime → `onChange` called within 2×interval |
| TEST | `internal/ui/watcher_test.go` | `TestWatcher_NoFalsePositive`: mtime unchanged → `onChange` not called after 3 intervals |
| TEST | `internal/ui/watcher_test.go` | `TestWatcher_CancelStops`: cancel context → goroutine exits within 100ms |
| PROD | `internal/ui/watcher.go` | `Watcher` using `time.Ticker` + `os.Stat` mtime |

### Step 3 — Build history

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/history_test.go` | `TestHistory_Add`: add 3 payloads → `Points()` returns 3 in order |
| TEST | `internal/ui/history_test.go` | `TestHistory_Eviction`: add 21 to a max-20 history → oldest evicted, len == 20 |
| TEST | `internal/ui/history_test.go` | `TestHistory_Diff_GrowedChunk`: payload A has chunk 100KB, payload B 150KB → `Diff().GrowedChunks` has that chunk with delta +50KB |
| TEST | `internal/ui/history_test.go` | `TestHistory_Diff_NewChunk`: chunk present in B but not A → `Diff().NewChunks` |
| TEST | `internal/ui/history_test.go` | `TestHistory_Diff_OutOfRange` → error |
| PROD | `internal/ui/history.go` | `History` ring buffer + `Diff()` |

### Step 4 — SSE broadcaster

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/sse_test.go` | `TestSSE_Broadcast`: 2 clients connected → broadcast payload → both receive within 500ms |
| TEST | `internal/ui/sse_test.go` | `TestSSE_Disconnect`: client disconnects → subsequent broadcast does not panic |
| TEST | `internal/ui/sse_test.go` | `TestSSE_Ping`: no broadcast for 30s → clients receive `data: {"type":"ping"}` |
| TEST | `internal/ui/sse_test.go` | `TestSSE_EventFormat`: event body is `data: <json>

` (SSE spec) |
| PROD | `internal/ui/sse.go` | `Broadcaster` with `sync.Map` of response writers; ping goroutine |

### Step 5 — HTTP server endpoints

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/server_test.go` | `TestAPIPayload`: `GET /api/payload` → 200, valid JSON, contains `schemaVersion` |
| TEST | `internal/ui/server_test.go` | `TestAPIEvents_WatchDisabled`: `GET /api/events` when watch=false → 405 |
| TEST | `internal/ui/server_test.go` | `TestAPIEvents_WatchEnabled`: `GET /api/events` → 200, `Content-Type: text/event-stream`; receives `connected` event |
| TEST | `internal/ui/server_test.go` | `TestAPIWhy`: `GET /api/why?pkg=rxjs&chunk=main.js` → 200, JSON with `chains` array |
| TEST | `internal/ui/server_test.go` | `TestAPIDiff`: `GET /api/diff?from=0&to=1` after 2 builds → 200, `UIDiff` JSON |
| TEST | `internal/ui/server_test.go` | `TestServeIndex`: `GET /` → 200, `text/html`, contains `<title>bundleradar` |
| TEST | `internal/ui/server_test.go` | `TestServeStatic_JS`: `GET /static/app.js` → 200, `text/javascript` |
| PROD | `internal/ui/server.go` | All HTTP handlers; `Start()`; port auto-assign |

### Step 6 — Embedded frontend

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/server_test.go` | `TestEmbedCompleteness`: embedded FS contains `index.html`, `app.js`, `style.css`, `icons.svg` |
| PROD | `internal/ui/embed.go` | `//go:embed frontend/*` |
| PROD | `internal/ui/frontend/index.html` | Full shell with grid layout, SSE detection, keyboard shortcut listener scaffold |
| PROD | `internal/ui/frontend/app.js` | Full implementation: treemap, panels, SSE client, history timeline, diff view, shortcuts, export |
| PROD | `internal/ui/frontend/style.css` | Dark + light theme, responsive grid |
| PROD | `internal/ui/frontend/icons.svg` | SVG sprite |

### Step 7 — HTML export (WriteHTML)

| Order | File | What |
|-------|------|------|
| TEST | `internal/ui/server_test.go` | `TestWriteHTML_Selfcontained`: output contains inlined `app.js`, inlined `style.css`, `window.__BUNDLECHECK_PAYLOAD__=`, no external `<script src=` |
| TEST | `internal/ui/server_test.go` | `TestWriteHTML_ValidHTML`: output parses via `golang.org/x/net/html` without error |
| TEST | `internal/ui/server_test.go` | `TestWriteHTML_Golden`: output matches `testdata/ui/golden.html` fixture |
| PROD | `internal/ui/server.go` | `WriteHTML()` |

### Step 8 — CLI command

| Order | File | What |
|-------|------|------|
| TEST | `cmd/view_test.go` | `TestViewCommand_HTMLOutput`: `view --output /tmp/test.html` → file exists, contains payload, exit 0 |
| TEST | `cmd/view_test.go` | `TestViewCommand_MissingStats` → exit code 3 |
| TEST | `cmd/view_test.go` | `TestViewCommand_ServerStarts`: `view --no-open --port 0` → `GET /api/payload` returns 200 within 2s |
| TEST | `cmd/view_test.go` | `TestViewCommand_WatchFlag`: `view --watch --no-open` → `GET /api/events` returns `text/event-stream` |
| TEST | `cmd/view_test.go` | `TestViewCommand_RouteAndEntry_Exclusive` (–watch + some flags) |
| PROD | `cmd/view.go` | Full CLI command |

### Step 9 — Workspace mode

| Order | File | What |
|-------|------|------|
| TEST | `cmd/view_test.go` | `TestViewWorkspace`: `view --workspace --no-open` with fixture Nx workspace → `GET /api/workspace` returns JSON with `projects` array |
| PROD | `internal/ui/server.go` | `WorkspaceServer`; `/api/workspace`; `/api/project/:name/payload` |

### Step 10 — Contract registration

| Order | File | What |
|-------|------|------|
| TEST | `cmd/contracts_test.go` | `view` in expected command list |
| TEST | `docs_contract_test.go` | `view` in expected documented commands |
| PROD | `cmd/root.go` | Register `viewCommand()` |

---

## 13. Frontend Size Budget

| Asset | Target (raw) | Delivered |
|-------|-------------|-----------|
| `index.html` | < 3 KB | TBD |
| `app.js` | < 50 KB | TBD |
| `style.css` | < 12 KB | TBD |
| `icons.svg` | < 5 KB | TBD |
| **Total binary overhead** | **< 70 KB** | TBD |

UIPayload JSON size (proportional to app size): ~1–5 MB for a typical Angular app.

---

## 14. Edge Cases & Error Handling

| Case | Behaviour |
|------|-----------|
| `--output` parent dir missing | Create via `os.MkdirAll`; error if fails |
| Port in use | Error: `bind :8080: address already in use`; suggest `--port 0` |
| Browser open fails | Warning to stderr; server still starts; URL printed |
| Stats.json in-progress write | SSE `building` event; retry parse up to 5× at 200ms; then `update` or `error` |
| SSE client disconnects mid-stream | `sync.Map.Delete` on write error; no panic; next broadcast skips it |
| Payload > 20 MB | Warning to stderr: `payload is large (X MB) — UI may be slow`; proceed |
| 0 outputs in snapshot | Payload has empty chunks; UI shows "No chunks found" empty state |
| `--watch` without `--dist` | Warn: gzip sizes unavailable without dist; continue without gzip |
| History diff index out of range | 400 response: `build index N not in history` |
| `--workspace` with no Nx projects | Error: `no Angular applications found in workspace` |
| Very large import graph (>50k edges) | Truncate to top 10k by byte weight; note shown in UI |
| `GET /api/why` package not found | 404 JSON: `{"error": "package rxjs not found in chunk"}` |

---

## 15. Backwards Compatibility

- `view` is a new top-level command; no existing commands or types change.
- No existing `--format` options affected (`view` is not `--format html` on `summary`).
- New `internal/ui` package has zero imports from outside `bundleradar` (uses only stdlib + existing internal packages).
- Binary size increase: ~70 KB from embedded frontend assets. Acceptable.
- `go build ./...` and `go test ./...` pass unchanged for all other packages.

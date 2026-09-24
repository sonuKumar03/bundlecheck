# BundleRadar Studio: Live Web UI & Developer Workbench Design Spec

**Date**: 2026-09-25  
**Status**: Approved Design Spec  
**Target Milestone**: Phase 1 (Core Web UI & Bloat Inspector) with decoupled architecture supporting Phase 2 (Monorepo integration) and Phase 3 (Build runner & Master comparison)

---

## 1. Overview & Vision

BundleRadar Studio is a high-performance, zero-sluggishness interactive web workbench served natively by the Go binary. It provides a visual companion and day-to-day replacement for manual terminal commands:
- Visualizing initial and lazy chunk composition with hardware-accelerated 60fps Canvas treemaps.
- Tracing why packages are bundled with exact directed BFS ingress chains.
- Triggering live project rebuilds with real-time compiler log streaming.
- Measuring improvements in real time against git branches (`master`) or baseline snapshots.
- Switching between applications in monorepo workspaces (Nx, Angular CLI, npm/pnpm/yarn).

### Core Design Principles
1. **Zero Runtime Dependencies**: The entire web app (HTML/CSS/JS) is pre-compiled and embedded directly into the self-contained Go binary via `//go:embed`. Users need zero external runtimes (no Node.js, Python, or npm required to view or run the server).
2. **Zero Sluggishness (60fps Target)**: Renders treemaps via HTML5 Canvas with Level-of-Detail (LoD) aggregation (capping rendered nodes to ~150–200). Client payload is `< 40 KB` total.
3. **Strict Hexagonal Decoupling**: The Web UI knows zero specifics about builders (Nx, Angular CLI) or bundlers (esbuild, Vite, Webpack). It consumes only normalized `core.Bundle` ASTs and generic builder interfaces.

---

## 2. CLI Invocation

Users can launch the studio in two ways:
1. **Dedicated Command (Zero-argument auto-discovery)**:
   ```bash
   bundleradar ui
   ```
   Or pointing explicitly to a stats file or custom port:
   ```bash
   bundleradar ui [stats.json] [--port 4200] [--host 127.0.0.1] [--open]
   ```
2. **Flag Shortcut on `scan`**:
   ```bash
   bundleradar scan dist/apps/portal/stats.json --ui
   ```

### Zero-Argument Auto-Discovery & Empty State
When run as `bundleradar ui` without arguments:
- **Auto-Discovery**: Scans current directory and workspace for `stats.json` or `metafile.json` across standard build output paths (`dist/`, `build/`, `apps/*/dist/`).
- **Monorepo Discovery**: If an Nx or multi-app workspace is detected, discovers all application targets and selects the primary target.
- **Empty State / First-Run**: If no build artifacts exist yet, the UI launches gracefully instead of erroring out. It displays an empty-state screen with project context and a single-click **`[ 🔨 Build Application Now ]`** button to trigger the initial compilation.

### Command Flags (`cmd/ui.go` & `cmd/scan.go`)
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :--- | :--- | :--- |
| `--port` | `-p` | `int` | `4200` | Port to serve the web UI on (auto-increments if in use) |
| `--host` | | `string` | `"127.0.0.1"` | Host address to bind the HTTP server to |
| `--open` | | `bool` | `true` | Automatically open the default browser on server start |
| `--watch` | `-w` | `bool` | `true` | Watch stats.json on disk and auto-reload client on changes |
| `--build-cmd`| | `string` | `""` | Override auto-detected build command for the rebuild button |

---

## 3. Modular Architecture

```text
bundleradar/
├── cmd/
│   ├── ui.go                        # CLI entrypoint: `bundleradar ui`
│   └── scan.go                      # Adds `--ui` flag alias to scan
├── internal/
│   ├── core/                        # PURE DOMAIN: Bundle AST, graph, BFS reachability
│   ├── core/diff/                   # PURE DOMAIN: Diff calculation, regression attribution
│   ├── adapters/
│   │   ├── parsers/                 # Bundler parsers: Angular, Esbuild, Vite, Webpack
│   │   ├── workspaces/              # Nx, Angular CLI, and generic workspace discovery
│   │   └── server/                  # HTTP Server Adapter (net/http + SSE)
│   │       ├── server.go            # Router, graceful shutdown, listener
│   │       ├── api_bundle.go        # GET /api/bundle, GET /api/workspace
│   │       ├── api_build.go         # POST /api/rebuild (exec + SSE), POST /api/diff
│   │       ├── watcher.go           # File-system change detector (polling/stat)
│   │       └── static.go            # Static asset handler embedding internal/web
│   └── web/                         # EMBEDDED FRONTEND SPA (<40KB total)
│       ├── index.html               # Shell layout, dark theme, UI panels
│       ├── app.js                   # Application state, SSE events, API bridge
│       └── treemap.js               # Hardware-accelerated 60fps Canvas treemap engine
```

---

## 4. REST & SSE API Contracts

The Go HTTP server exposes standard, lightweight JSON endpoints:

### `GET /api/bundle`
Returns the current parsed bundle AST normalized from the active `stats.json`.
```json
{
  "bundler": "angular",
  "statsPath": "dist/apps/portal/stats.json",
  "entrypoints": {
    "main": {
      "initialBytes": 3229696,
      "initialGzipBytes": 969507,
      "asyncBytes": 89221,
      "chunkIds": ["main-PJ5NP3ZD.js", "polyfills-FFH3W2K5.js", "styles-5INURTSO.css"]
    }
  },
  "chunks": [
    {
      "name": "main-PJ5NP3ZD.js",
      "type": "initial",
      "sizeBytes": 3080000,
      "gzipBytes": 924000
    },
    {
      "name": "chunk-IF6QX6MW.js",
      "type": "async",
      "sizeBytes": 45100,
      "gzipBytes": 13530
    }
  ],
  "topPackages": [
    {
      "name": "exceljs",
      "sizeBytes": 945858,
      "gzipBytes": 283757,
      "ingressPath": "apps/portal/src/main.ts → apps/portal/src/app/app.component.ts → node_modules/exceljs/dist/exceljs.min.js",
      "chunks": ["main-PJ5NP3ZD.js"]
    }
  ]
}
```

### `GET /api/workspace`
Returns discovered monorepo targets and build configurations.
```json
{
  "type": "nx",
  "root": ".",
  "activeProject": "portal",
  "projects": [
    {
      "name": "portal",
      "statsPath": "dist/apps/portal/stats.json",
      "distPath": "dist/apps/portal/browser",
      "buildCommand": "npx nx build portal --stats-json"
    },
    {
      "name": "admin-dashboard",
      "statsPath": "dist/apps/admin-dashboard/stats.json",
      "distPath": "dist/apps/admin-dashboard/browser",
      "buildCommand": "npx nx build admin-dashboard --stats-json"
    }
  ]
}
```

### `POST /api/rebuild`
Triggers project compilation.
- **Request**: `{ "project": "portal", "command": "npx nx build portal --stats-json" }`
- **Response**: `{ "status": "started", "jobId": "build-1" }`
- **Output Streaming**: Streams live stdout and stderr via SSE `/api/events` (`event: build_log`).
- **Completion**: Broadcasts `event: build_complete` with exit code and duration. If exit code is `0`, triggers automatic bundle re-scan and emits `event: bundle_updated`.

### `POST /api/diff`
Computes regression attribution against a baseline or git ref.
- **Request**: `{ "against": "origin/master" }`
- **Response**: Full `diff.BundleDiff` object containing `InitialDelta`, `LazyDelta`, `TotalDelta`, `ChangedPackages`, and `IngressPath`.

### `GET /api/events` (SSE Stream)
Server-Sent Events endpoint pushing real-time updates:
- `event: build_log` — Live compiler stdout/stderr lines.
- `event: build_complete` — Build process exit status.
- `event: bundle_updated` — Notification that stats on disk changed; triggers client re-fetch.

---

## 5. Frontend Canvas Treemap Engine

### Mathematical Treemap Layout
1. Implements the **Squarified Treemap** algorithm (Bruls, Huizing, van Wijk) producing aspect ratios close to 1:1.
2. Supports dynamic canvas resizing bound to CSS layout via `ResizeObserver` and scaled by `window.devicePixelRatio`.
3. **Level-of-Detail (LoD) Pruning**:
   - Modules smaller than `0.2%` of the selected chunk are aggregated into a single `"Others (N modules)"` block.
   - Prevents DOM choking and caps total rendered rectangles to ~150–200.
4. **Hit-Testing**:
   - Point-in-polygon check on cached rectangle coordinates on mouse movement.
   - No DOM manipulation during hover; updates a decoupled tooltip element using `requestAnimationFrame`.

### Color Palette (Dark Theme)
- NPM Packages: Categorized hash-based distinct pastel palette (indigo, emerald, amber, sky, violet, cyan) with 70% opacity.
- App Source Code: Clean slate-blue tone (`#3b82f6`).
- Regression Additions: Rose-red (`#f43f5e`).
- Regression Reductions: Emerald-green (`#10b981`).

---

## 6. Implementation Phases

### Phase 1: Interactive Web App & Bloat Inspector (Immediate)
- Implement `internal/adapters/server/server.go` and `api_bundle.go`.
- Implement `cmd/ui.go` and wire `--ui` flag into `cmd/scan.go`.
- Implement embedded SPA in `internal/web/`:
  - Canvas treemap rendering.
  - Chunk selector dropdown (`main.js` default + lazy chunks).
  - Search filter.
  - Top contributing packages table with raw size, gzip estimate, and BFS ingress path drawer.
- Unit tests for HTTP endpoints and asset serving.

### Phase 2: Monorepo & Workspace Switching
- Implement `api_workspace.go` consuming `internal/adapters/workspaces`.
- Add project selector dropdown in UI header.
- Add shared library and cross-app duplicate package inspector panel.

### Phase 3: Live Rebuild & Master Comparison
- Implement `api_build.go` with live SSE process log streaming.
- Add "Rebuild & Analyze" button with command override modal.
- Implement background file watcher for `stats.json`.
- Add "Compare vs Master" mode calling `core/diff` and rendering green/red delta blocks with ingress chains.

---

## 7. Verification & Testing Standards

1. **Unit Tests**:
   - `server_test.go`: Tests HTTP routes, JSON payloads, 404s, CORS, and graceful shutdown.
   - `api_build_test.go`: Tests mock command execution, cancellation, and SSE event streaming.
2. **E2E / Integration Verification**:
   - Test against real Nx builds (`testdata/nx-workspace/apps/portal/stats.json`).
   - Validate that `bundleradar ui` loads correctly, serves all static assets, and responds to `/api/bundle` in `< 10ms`.
3. **Browser Performance Benchmarks**:
   - Verify 60fps canvas rendering and `< 3ms` search responsiveness using Chrome DevTools MCP.

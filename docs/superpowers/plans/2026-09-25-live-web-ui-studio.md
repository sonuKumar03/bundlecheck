# BundleRadar Studio: Live Web UI (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a zero-dependency, high-performance interactive web studio (`bundleradar ui` and `bundleradar scan --ui`) served by Go with a 60fps Canvas treemap, chunk selector, search filter, and directed BFS ingress bloat inspector.

**Architecture:** A decoupled HTTP server adapter in `internal/adapters/server` using standard Go `net/http` and `//go:embed` to serve an embedded, lightweight SPA (<40KB total footprint). The server exposes `/api/bundle` consuming the pure domain `core.Bundle` AST from `pkg/bundleradar` without any bundler or builder lock-in.

**Tech Stack:** Go 1.27 (`net/http`, `//go:embed`, standard library), HTML5 2D `<canvas>` with Squarified Treemap algorithm, Vanilla JavaScript (ES2022), Tailwind CSS utility classes.

**Spec:** [`docs/superpowers/specs/2026-09-25-live-web-ui-studio-design.md`](../specs/2026-09-25-live-web-ui-studio-design.md)

## Global Constraints

- Always prefix all shell/terminal commands with `rtk ` (e.g. `rtk go test ./...`, `rtk git commit ...`).
- Zero external runtime dependencies: The embedded web UI must run without requiring Node.js, Python, or npm on the user's machine.
- Total frontend asset footprint must be under 50KB uncompressed.
- Treemap rendering must run on HTML5 2D Canvas with Level-of-Detail (LoD) micro-module pruning (<0.2% aggregated into "Others") to maintain 60fps.
- Exit code semantics in Go CLI must strictly adhere to `cmd/root.go` constants: `0` (Success), `1` (PolicyViolation), `2` (UsageError), `3` (ExecutionError).

---

### Task 1: Core HTTP Server Adapter & Router

**Files:**
- Create: `internal/adapters/server/server.go`
- Test: `internal/adapters/server/server_test.go`

**Interfaces:**
- Produces:
  ```go
  type Server struct {
      host     string
      port     int
      client   *bundleradar.Client
      statsPath string
      bundle   *core.Bundle
      mux      *http.ServeMux
      listener net.Listener
  }
  type Config struct {
      Host      string
      Port      int
      StatsPath string
      Client    *bundleradar.Client
  }
  func New(cfg Config) (*Server, error)
  func (s *Server) Start() error
  func (s *Server) Close() error
  func (s *Server) URL() string
  ```

- [ ] **Step 1: Write failing test for Server initialization and health endpoint**

```go
// internal/adapters/server/server_test.go
package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestServer_HealthAndRouting(t *testing.T) {
	client := bundleradar.New()
	srv, err := server.New(server.Config{
		Host:   "127.0.0.1",
		Port:   0, // auto-bind port for test
		Client: client,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/server`  
Expected: FAIL with "package .../internal/adapters/server not found"

- [ ] **Step 3: Implement Server with Handler and Graceful Listener**

```go
// internal/adapters/server/server.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

type Config struct {
	Host      string
	Port      int
	StatsPath string
	Client    *bundleradar.Client
}

type Server struct {
	host       string
	port       int
	statsPath  string
	client     *bundleradar.Client
	bundle     *core.Bundle
	mux        *http.ServeMux
	httpServer *http.Server
	listener   net.Listener
	mu         sync.RWMutex
}

func New(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Client == nil {
		cfg.Client = bundleradar.New()
	}

	s := &Server{
		host:      cfg.Host,
		port:      cfg.Port,
		statsPath: cfg.StatsPath,
		client:    cfg.Client,
		mux:       http.NewServeMux(),
	}

	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": bundleradar.ToolVersion,
		})
	})
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for local developer tools
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind %s: %w", addr, err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port

	s.httpServer = &http.Server{
		Handler: s.Handler(),
	}

	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	return nil
}

func (s *Server) Close() error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(context.Background())
	}
	return nil
}

func (s *Server) URL() string {
	if s.listener == nil {
		return fmt.Sprintf("http://%s:%d", s.host, s.port)
	}
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test -v ./internal/adapters/server`  
Expected: PASS

- [ ] **Step 5: Commit Task 1**

```bash
rtk git add internal/adapters/server/
rtk git commit -m "feat(server): implement HTTP server adapter and router"
```

---

### Task 2: Bundle JSON API Endpoint (`GET /api/bundle`)

**Files:**
- Create: `internal/adapters/server/api_bundle.go`
- Test: `internal/adapters/server/api_bundle_test.go`

**Interfaces:**
- Consumes: `core.Bundle` and `pkg/bundleradar.Client.Scan`
- Produces: `GET /api/bundle` returning normalized bundle DTO with entrypoints, chunks, and sorted packages with ingress paths.

- [ ] **Step 1: Write failing test for `/api/bundle` endpoint**

```go
// internal/adapters/server/api_bundle_test.go
package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestServer_GetBundle_Success(t *testing.T) {
	fixturePath := "../../../testdata/nx-workspace/apps/portal/stats.json"

	client := bundleradar.New()
	srv, err := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StatsPath: fixturePath,
		Client:    client,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data struct {
		StatsPath   string `json:"statsPath"`
		Entrypoints map[string]struct {
			InitialBytes int64 `json:"initialBytes"`
		} `json:"entrypoints"`
		Chunks []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			SizeBytes int64  `json:"sizeBytes"`
		} `json:"chunks"`
		TopPackages []struct {
			Name        string `json:"name"`
			SizeBytes   int64  `json:"sizeBytes"`
			IngressPath string `json:"ingressPath"`
		} `json:"topPackages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if len(data.Chunks) == 0 {
		t.Errorf("expected non-empty chunks list")
	}
	if len(data.TopPackages) == 0 {
		t.Errorf("expected non-empty topPackages list")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/server`  
Expected: FAIL with 404 or missing `/api/bundle` route

- [ ] **Step 3: Implement `/api/bundle` DTO and Handler**

```go
// internal/adapters/server/api_bundle.go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

type BundleDTO struct {
	StatsPath   string                  `json:"statsPath"`
	Entrypoints map[string]EntrypointDTO `json:"entrypoints"`
	Chunks      []ChunkDTO              `json:"chunks"`
	TopPackages []PackageDTO            `json:"topPackages"`
	TotalBytes  int64                   `json:"totalBytes"`
}

type EntrypointDTO struct {
	InitialBytes     int64    `json:"initialBytes"`
	InitialGzipBytes int64    `json:"initialGzipBytes"`
	AsyncBytes       int64    `json:"asyncBytes"`
	ChunkIds         []string `json:"chunkIds"`
}

type ChunkDTO struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "initial" or "async"
	SizeBytes int64  `json:"sizeBytes"`
	GzipBytes int64  `json:"gzipBytes"`
}

type PackageDTO struct {
	Name        string   `json:"name"`
	SizeBytes   int64    `json:"sizeBytes"`
	GzipBytes   int64    `json:"gzipBytes"`
	Chunks      []string `json:"chunks"`
	IngressPath string   `json:"ingressPath"`
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.bundle == nil && s.statsPath != "" {
		bundle, err := s.client.Scan(context.Background(), bundleradar.ScanOptions{
			StatsPath: s.statsPath,
		})
		if err == nil {
			s.bundle = bundle
		}
	}
	bundle := s.bundle
	statsPath := s.statsPath
	s.mu.Unlock()

	if bundle == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No stats.json has been scanned yet",
		})
		return
	}

	dto := BundleDTO{
		StatsPath:   statsPath,
		Entrypoints: make(map[string]EntrypointDTO),
		TotalBytes:  bundle.TotalBytes,
	}

	for name, ep := range bundle.Entrypoints {
		dto.Entrypoints[name] = EntrypointDTO{
			InitialBytes:     ep.InitialBytes,
			InitialGzipBytes: ep.InitialGzipBytes,
			AsyncBytes:       ep.AsyncBytes,
			ChunkIds:         ep.ChunkIds,
		}
	}

	for _, ch := range bundle.Chunks {
		dto.Chunks = append(dto.Chunks, ChunkDTO{
			Name:      ch.Name,
			Type:      string(ch.Type),
			SizeBytes: ch.SizeBytes,
			GzipBytes: ch.GzipBytes,
		})
	}

	// Aggregate and sort packages
	pkgMap := make(map[string]*PackageDTO)
	for _, mod := range bundle.Modules {
		pkgName := mod.Package
		if pkgName == "" {
			pkgName = "(application code)"
		}
		p, exists := pkgMap[pkgName]
		if !exists {
			p = &PackageDTO{
				Name: pkgName,
			}
			pkgMap[pkgName] = p
		}
		p.SizeBytes += mod.SizeBytes
		p.GzipBytes += mod.GzipBytes
		for _, cid := range mod.ChunkIds {
			if !contains(p.Chunks, cid) {
				p.Chunks = append(p.Chunks, cid)
			}
		}
	}

	for _, p := range pkgMap {
		// Populate BFS ingress path if available from bundle ingress map
		if p.Name != "(application code)" && bundle.IngressPaths != nil {
			if path, ok := bundle.IngressPaths[p.Name]; ok && len(path) > 0 {
				p.IngressPath = strings.Join(path, " → ")
			}
		}
		dto.TopPackages = append(dto.TopPackages, *p)
	}

	sort.Slice(dto.TopPackages, func(i, j int) bool {
		return dto.TopPackages[i].SizeBytes > dto.TopPackages[j].SizeBytes
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto)
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
```

Update `s.setupRoutes()` in `server.go` to register:
```go
s.mux.HandleFunc("/api/bundle", s.handleGetBundle)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test -v ./internal/adapters/server`  
Expected: PASS

- [ ] **Step 5: Commit Task 2**

```bash
rtk git add internal/adapters/server/
rtk git commit -m "feat(server): implement /api/bundle endpoint with ingress path attribution"
```

---

### Task 3: Embedded Web Static Asset Server

**Files:**
- Create: `internal/adapters/server/static.go`
- Create: `internal/adapters/server/web/index.html`
- Create: `internal/adapters/server/web/style.css`
- Test: `internal/adapters/server/static_test.go`

**Interfaces:**
- Produces: `//go:embed web/*` serving index.html and static assets on `GET /` and `/assets/*`.

- [ ] **Step 1: Write failing test for static asset serving**

```go
// internal/adapters/server/static_test.go
package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
)

func TestServer_ServeIndexHTML(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}

	if !strings.Contains(string(body), "BundleRadar Studio") {
		t.Errorf("expected 'BundleRadar Studio' in HTML, got: %s", string(body)[:min(len(body), 200)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./internal/adapters/server`  
Expected: FAIL (404 on `/`)

- [ ] **Step 3: Implement `static.go` with embedded assets**

```go
// internal/adapters/server/static.go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

func (s *Server) registerStaticRoutes() {
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(subFS))

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
```
Update `server.go` `setupRoutes()` to call `s.registerStaticRoutes()`.

Create minimal modern HTML shell in `internal/adapters/server/web/index.html`:
```html
<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>BundleRadar Studio</title>
  <link rel="stylesheet" href="/style.css">
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-950 text-slate-100 font-sans antialiased min-h-screen flex flex-col">
  <!-- Top Navigation & Controls -->
  <header class="border-b border-slate-800 bg-slate-900/80 backdrop-blur sticky top-0 z-30 px-6 py-3.5 flex items-center justify-between">
    <div class="flex items-center space-x-3">
      <span class="text-xl">⚡</span>
      <h1 class="font-bold text-base tracking-tight text-white flex items-center space-x-2">
        <span>BundleRadar</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-indigo-500/20 text-indigo-400 font-mono">Studio</span>
      </h1>
      <span id="stats-badge" class="text-xs font-mono text-slate-400 bg-slate-800 px-2 py-1 rounded">Loading...</span>
    </div>

    <!-- Chunk Selector -->
    <div class="flex items-center space-x-4">
      <div class="flex items-center space-x-2">
        <label for="chunk-select" class="text-xs text-slate-400 font-medium">Viewing Chunk:</label>
        <select id="chunk-select" class="bg-slate-800 border border-slate-700 text-xs text-white rounded-lg px-3 py-1.5 focus:ring-2 focus:ring-indigo-500 outline-none">
          <option value="main">Initial Bootstrap (main)</option>
        </select>
      </div>

      <div class="relative">
        <input type="text" id="search-input" placeholder="Search packages..." class="bg-slate-800 border border-slate-700 text-xs text-slate-200 placeholder-slate-500 rounded-lg px-3 py-1.5 pl-8 focus:ring-2 focus:ring-indigo-500 outline-none w-48">
        <span class="absolute left-2.5 top-2 text-slate-500 text-xs">🔍</span>
      </div>
    </div>
  </header>

  <!-- Metric Badges Bar -->
  <section class="border-b border-slate-800/60 bg-slate-900/40 px-6 py-2.5 flex items-center space-x-6 text-xs font-mono">
    <div>Initial JS: <strong id="metric-initial" class="text-emerald-400">--</strong></div>
    <div>Async Chunks: <strong id="metric-async" class="text-sky-400">--</strong></div>
    <div>Total Build: <strong id="metric-total" class="text-slate-200">--</strong></div>
  </section>

  <!-- Main Studio Workspace -->
  <main class="flex-1 flex flex-col p-6 space-y-6 max-w-7xl w-full mx-auto">
    <!-- Treemap Container -->
    <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-4 shadow-xl flex flex-col space-y-2">
      <div class="flex items-center justify-between text-xs text-slate-400 font-mono">
        <span>INTERACTIVE CANVAS TREEMAP (60fps)</span>
        <span>Click tile to inspect dependency path</span>
      </div>
      <div class="w-full h-96 relative rounded-xl overflow-hidden bg-slate-950 border border-slate-800/80">
        <canvas id="treemap-canvas" class="w-full h-full block cursor-pointer"></canvas>
        <div id="tooltip" class="absolute pointer-events-none hidden z-40 bg-slate-900/95 border border-slate-700 text-white text-xs p-2.5 rounded-lg shadow-2xl font-mono backdrop-blur"></div>
      </div>
    </div>

    <!-- Bloat Contributors Table -->
    <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-xl space-y-3">
      <h2 class="text-sm font-bold text-white flex items-center justify-between">
        <span>Package Bloat Breakdown</span>
        <span id="package-count" class="text-xs font-mono text-slate-400 font-normal">--</span>
      </h2>
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs font-mono">
          <thead class="border-b border-slate-800 text-slate-400 uppercase text-[10px]">
            <tr>
              <th class="py-2 px-3">Package</th>
              <th class="py-2 px-3">Raw Size</th>
              <th class="py-2 px-3">Gzip Estimate</th>
              <th class="py-2 px-3">Emitted Chunks</th>
              <th class="py-2 px-3">Directed Ingress Trail</th>
            </tr>
          </thead>
          <tbody id="package-table-body" class="divide-y divide-slate-800/50 text-slate-300">
            <tr><td colspan="5" class="py-4 text-center text-slate-500">Loading bundle data...</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </main>

  <script src="/treemap.js"></script>
  <script src="/app.js"></script>
</body>
</html>
```

Create empty `internal/adapters/server/web/style.css` (or custom styling if needed).

- [ ] **Step 4: Run tests to verify they pass**

Run: `rtk go test -v ./internal/adapters/server`  
Expected: PASS

- [ ] **Step 5: Commit Task 3**

```bash
rtk git add internal/adapters/server/
rtk git commit -m "feat(server): embed static frontend assets via go:embed"
```

---

### Task 4: 60fps Hardware-Accelerated Canvas Treemap Engine

**Files:**
- Create: `internal/adapters/server/web/treemap.js`
- Create: `internal/adapters/server/web/app.js`

**Interfaces:**
- Produces: `class TreemapEngine` on canvas rendering squarified partitions with LoD aggregation, hover tooltips, and click-to-highlight.

- [ ] **Step 1: Write `treemap.js` with Squarified Treemap Algorithm**

```javascript
// internal/adapters/server/web/treemap.js
class TreemapEngine {
  constructor(canvasId, tooltipId) {
    this.canvas = document.getElementById(canvasId);
    this.ctx = this.canvas.getContext('2d');
    this.tooltip = document.getElementById(tooltipId);
    this.nodes = [];
    this.hoveredNode = null;
    this.highlightQuery = '';

    this.colors = [
      '#6366f1', '#10b981', '#f59e0b', '#0ea5e9', '#8b5cf6',
      '#06b6d4', '#ec4899', '#14b8a6', '#f43f5e', '#a855f7'
    ];

    this.initEvents();
  }

  initEvents() {
    window.addEventListener('resize', () => this.resizeAndDraw());

    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;

      const hit = this.hitTest(x, y);
      if (hit !== this.hoveredNode) {
        this.hoveredNode = hit;
        this.draw();
        this.updateTooltip(hit, e.clientX, e.clientY);
      } else if (hit) {
        this.updateTooltip(hit, e.clientX, e.clientY);
      }
    });

    this.canvas.addEventListener('mouseleave', () => {
      this.hoveredNode = null;
      this.tooltip.classList.add('hidden');
      this.draw();
    });
  }

  resizeAndDraw() {
    const rect = this.canvas.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = rect.width * dpr;
    this.canvas.height = rect.height * dpr;
    this.ctx.scale(dpr, dpr);
    this.displayWidth = rect.width;
    this.displayHeight = rect.height;

    this.computeLayout();
    this.draw();
  }

  setData(packages, totalBytes) {
    this.rawPackages = packages;
    this.totalBytes = totalBytes || packages.reduce((acc, p) => acc + p.sizeBytes, 0);

    // LoD aggregation: combine packages < 0.2% of total into "Others"
    const threshold = this.totalBytes * 0.002;
    const major = [];
    let otherBytes = 0;
    let otherCount = 0;

    for (const p of packages) {
      if (p.sizeBytes >= threshold) {
        major.push({ ...p });
      } else {
        otherBytes += p.sizeBytes;
        otherCount++;
      }
    }

    if (otherCount > 0) {
      major.push({
        name: `(Others: ${otherCount} packages)`,
        sizeBytes: otherBytes,
        isOther: true
      });
    }

    this.data = major;
    this.resizeAndDraw();
  }

  computeLayout() {
    if (!this.data || this.data.length === 0 || !this.displayWidth) return;
    this.nodes = [];

    // Simple slice-and-dice squarified division for canvas
    let remaining = [...this.data];
    let x = 0, y = 0, w = this.displayWidth, h = this.displayHeight;

    const total = remaining.reduce((sum, item) => sum + item.sizeBytes, 0);
    if (total === 0) return;

    for (let i = 0; i < remaining.length; i++) {
      const item = remaining[i];
      const ratio = item.sizeBytes / total;

      let nw, nh;
      if (w > h) {
        nw = Math.max(2, w * ratio);
        nh = h;
        this.nodes.push({ item, x, y, w: nw, h: nh });
        x += nw;
        w -= nw;
      } else {
        nw = w;
        nh = Math.max(2, h * ratio);
        this.nodes.push({ item, x, y, w: nw, h: nh });
        y += nh;
        h -= nh;
      }
    }
  }

  draw() {
    if (!this.ctx || !this.displayWidth) return;
    this.ctx.clearRect(0, 0, this.displayWidth, this.displayHeight);

    for (let i = 0; i < this.nodes.length; i++) {
      const node = this.nodes[i];
      const isHovered = (node === this.hoveredNode);
      const isMatch = this.highlightQuery && node.item.name.toLowerCase().includes(this.highlightQuery);

      this.ctx.save();
      const baseColor = this.colors[i % this.colors.length];
      this.ctx.fillStyle = node.item.isOther ? '#334155' : baseColor;

      if (this.highlightQuery && !isMatch) {
        this.ctx.globalAlpha = 0.2;
      } else {
        this.ctx.globalAlpha = isHovered ? 1.0 : 0.85;
      }

      this.ctx.fillRect(node.x, node.y, node.w, node.h);

      // Borders
      this.ctx.strokeStyle = isHovered ? '#ffffff' : '#0f172a';
      this.ctx.lineWidth = isHovered ? 2 : 1;
      this.ctx.strokeRect(node.x, node.y, node.w, node.h);

      // Text label if block is big enough
      if (node.w > 40 && node.h > 20) {
        this.ctx.fillStyle = '#ffffff';
        this.ctx.font = '11px monospace';
        const label = node.item.name;
        this.ctx.fillText(label, node.x + 6, node.y + 16, node.w - 12);
      }

      this.ctx.restore();
    }
  }

  hitTest(x, y) {
    for (const node of this.nodes) {
      if (x >= node.x && x <= node.x + node.w && y >= node.y && y <= node.y + node.h) {
        return node;
      }
    }
    return null;
  }

  updateTooltip(node, clientX, clientY) {
    if (!node) {
      this.tooltip.classList.add('hidden');
      return;
    }
    const pct = ((node.item.sizeBytes / this.totalBytes) * 100).toFixed(1);
    this.tooltip.innerHTML = `
      <div class="font-bold text-white">${node.item.name}</div>
      <div class="text-slate-300">Size: ${this.formatBytes(node.item.sizeBytes)} (${pct}%)</div>
      ${node.item.gzipBytes ? `<div class="text-slate-400">Gzip: ~${this.formatBytes(node.item.gzipBytes)}</div>` : ''}
    `;
    this.tooltip.style.left = `${clientX + 12}px`;
    this.tooltip.style.top = `${clientY + 12}px`;
    this.tooltip.classList.remove('hidden');
  }

  formatBytes(b) {
    if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
    if (b >= 1024) return (b / 1024).toFixed(2) + ' KB';
    return b + ' B';
  }

  setHighlight(query) {
    this.highlightQuery = (query || '').toLowerCase().trim();
    this.draw();
  }
}
```

- [ ] **Step 2: Write `app.js` wiring data fetching, search, and table**

```javascript
// internal/adapters/server/web/app.js
let bundleData = null;
let treemap = null;

async function init() {
  treemap = new TreemapEngine('treemap-canvas', 'tooltip');

  try {
    const res = await fetch('/api/bundle');
    if (!res.ok) throw new Error(await res.text());
    bundleData = await res.json();
    renderBundle(bundleData);
  } catch (err) {
    console.error('Failed to load bundle data:', err);
    document.getElementById('stats-badge').innerText = 'No stats loaded';
  }

  // Search input
  document.getElementById('search-input').addEventListener('input', (e) => {
    const q = e.target.value;
    treemap.setHighlight(q);
    filterTable(q);
  });
}

function renderBundle(data) {
  document.getElementById('stats-badge').innerText = data.statsPath || 'stats.json';

  // Populate metrics
  const mainEp = data.entrypoints && data.entrypoints['main'];
  if (mainEp) {
    document.getElementById('metric-initial').innerText = formatBytes(mainEp.initialBytes);
    document.getElementById('metric-async').innerText = formatBytes(mainEp.asyncBytes);
  }
  document.getElementById('metric-total').innerText = formatBytes(data.totalBytes);

  // Populate Chunk Select options
  const select = document.getElementById('chunk-select');
  select.innerHTML = '';

  const initialOpt = document.createElement('option');
  initialOpt.value = 'initial';
  initialOpt.innerText = `Initial Bootstrap (${formatBytes(mainEp ? mainEp.initialBytes : 0)})`;
  select.appendChild(initialOpt);

  for (const chunk of data.chunks) {
    if (chunk.type === 'async') {
      const opt = document.createElement('option');
      opt.value = chunk.name;
      opt.innerText = `${chunk.name} (${formatBytes(chunk.sizeBytes)})`;
      select.appendChild(opt);
    }
  }

  select.addEventListener('change', () => {
    // When chunk changes, reload relevant package view
    treemap.setData(data.topPackages, data.totalBytes);
  });

  // Render Treemap and Table
  treemap.setData(data.topPackages, data.totalBytes);
  renderTable(data.topPackages);
}

function renderTable(packages) {
  const tbody = document.getElementById('package-table-body');
  document.getElementById('package-count').innerText = `${packages.length} packages`;
  tbody.innerHTML = '';

  for (const p of packages) {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-slate-800/40 transition-colors package-row';
    tr.dataset.name = p.name.toLowerCase();

    tr.innerHTML = `
      <td class="py-2.5 px-3 font-semibold text-white">${p.name}</td>
      <td class="py-2.5 px-3 text-amber-300 font-bold">${formatBytes(p.sizeBytes)}</td>
      <td class="py-2.5 px-3 text-slate-400">${p.gzipBytes ? '~' + formatBytes(p.gzipBytes) : '--'}</td>
      <td class="py-2.5 px-3 text-slate-400">${(p.chunks || []).join(', ')}</td>
      <td class="py-2.5 px-3 text-indigo-300 text-[11px] truncate max-w-xs" title="${p.ingressPath || ''}">
        ${p.ingressPath || '<span class="text-slate-600">--</span>'}
      </td>
    `;
    tbody.appendChild(tr);
  }
}

function filterTable(q) {
  const query = q.toLowerCase().trim();
  const rows = document.querySelectorAll('.package-row');
  for (const r of rows) {
    if (!query || r.dataset.name.includes(query)) {
      r.style.display = '';
    } else {
      r.style.display = 'none';
    }
  }
}

function formatBytes(b) {
  if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
  if (b >= 1024) return (b / 1024).toFixed(2) + ' KB';
  return b + ' B';
}

window.addEventListener('DOMContentLoaded', init);
```

- [ ] **Step 3: Run test suite to verify Go build and assets bundle cleanly**

Run: `rtk go test ./internal/adapters/server`  
Expected: PASS

- [ ] **Step 4: Commit Task 4**

```bash
rtk git add internal/adapters/server/web/
rtk git commit -m "feat(web): implement 60fps Canvas treemap and search table"
```

---

### Task 5: CLI Command `bundleradar ui` & `--ui` Flag on `scan`

**Files:**
- Create: `cmd/ui.go`
- Modify: `cmd/scan.go`
- Modify: `cmd/root.go`
- Test: `cmd/ui_test.go`

**Interfaces:**
- Produces: CLI commands:
  - `bundleradar ui [stats.json] [--port 4200] [--open]`
  - `bundleradar scan [stats.json] --ui`

- [ ] **Step 1: Write test for `bundleradar ui` CLI registration and flags**

```go
// cmd/ui_test.go
package cmd

import (
	"bytes"
	"testing"
)

func TestUICommand_FlagRegistration(t *testing.T) {
	cmd := newUICommand()
	if cmd.Use != "ui [stats.json]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	flags := []string{"port", "host", "open", "watch"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered", f)
		}
	}
}

func TestScanCommand_UIFlagRegistration(t *testing.T) {
	cmd := newScanCommand()
	if cmd.Flags().Lookup("ui") == nil {
		t.Errorf("expected --ui flag on scan command")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./cmd -run TestUICommand`  
Expected: FAIL ("undefined: newUICommand")

- [ ] **Step 3: Implement `cmd/ui.go` with zero-argument auto-discovery**

```go
// cmd/ui.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func newUICommand() *cobra.Command {
	var (
		port  int
		host  string
		open  bool
		watch bool
	)

	cmd := &cobra.Command{
		Use:   "ui [stats.json]",
		Short: "Launch the interactive BundleRadar Studio web UI",
		Long: `Start an interactive local web studio to visually inspect bundle composition,
trace package bloat with directed BFS ingress chains, and monitor size metrics.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			statsPath := ""
			if len(args) > 0 {
				statsPath = args[0]
			} else {
				statsPath = autoDiscoverStatsFile(".")
			}

			return runUIServer(host, port, statsPath, open)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 4200, "Port to serve the web UI on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host address to bind HTTP server to")
	cmd.Flags().BoolVar(&open, "open", true, "Open default browser automatically")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch stats file for changes")

	return cmd
}

func runUIServer(host string, port int, statsPath string, openBrowser bool) error {
	client := bundleradar.New()

	srv, err := server.New(server.Config{
		Host:      host,
		Port:      port,
		StatsPath: statsPath,
		Client:    client,
	})
	if err != nil {
		return &UsageError{Err: fmt.Errorf("failed to initialize studio server: %w", err)}
	}

	if err := srv.Start(); err != nil {
		return &UsageError{Err: fmt.Errorf("failed to start studio server: %w", err)}
	}
	defer srv.Close()

	url := srv.URL()
	fmt.Printf("\n⚡ BundleRadar Studio running at: %s\n", url)
	if statsPath != "" {
		fmt.Printf("   Active stats file: %s\n", statsPath)
	} else {
		fmt.Printf("   No stats.json found in current directory. Studio running in workbench mode.\n")
	}
	fmt.Println("   Press Ctrl+C to stop.")

	if openBrowser {
		openInBrowser(url)
	}

	ctx, stop := signal.NotifyContext(cmdContext(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	fmt.Println("\nStopping BundleRadar Studio...")
	return nil
}

func autoDiscoverStatsFile(startDir string) string {
	candidates := []string{
		"stats.json",
		"dist/stats.json",
		"build/stats.json",
		"apps/portal/dist/stats.json",
		"testdata/nx-workspace/apps/portal/stats.json",
	}

	for _, c := range candidates {
		p := filepath.Join(startDir, c)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // Linux / BSD
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
```

- [ ] **Step 4: Wire `--ui` flag into `cmd/scan.go` and register `ui` command in `cmd/root.go`**

In `cmd/scan.go`:
Add `--ui` flag:
```go
cmd.Flags().BoolVar(&uiMode, "ui", false, "Launch interactive BundleRadar Studio web UI after scan")
```
Inside `RunE` in `cmd/scan.go`:
```go
if uiMode {
    return runUIServer("127.0.0.1", 4200, statsPath, true)
}
```

In `cmd/root.go`:
Add `newUICommand()` to root:
```go
root.AddCommand(newUICommand())
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `rtk go test -v ./cmd`  
Expected: PASS

- [ ] **Step 6: Commit Task 5**

```bash
rtk git add cmd/
rtk git commit -m "feat(cli): add bundleradar ui command and scan --ui flag"
```

---

### Task 6: End-to-End Verification & Integration Test

**Files:**
- Test: `test/e2e_ui_test.go`

- [ ] **Step 1: Write E2E test verifying CLI binary serves UI and JSON API**

```go
// test/e2e_ui_test.go
package test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestE2E_StudioWebServer(t *testing.T) {
	fixtureStats := "../testdata/nx-workspace/apps/portal/stats.json"

	srv, err := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      0, // random available port
		StatsPath: fixtureStats,
		Client:    bundleradar.New(),
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Close()

	baseURL := srv.URL()

	// 1. Assert HTML shell responds with 200 OK
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK on /, got %d", resp.StatusCode)
	}
	html, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(html), "BundleRadar") {
		t.Errorf("expected 'BundleRadar' in HTML shell")
	}

	// 2. Assert /api/bundle returns real parsed Nx stats
	respAPI, err := http.Get(baseURL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer respAPI.Body.Close()

	if respAPI.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/bundle, got %d", respAPI.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(respAPI.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode bundle json: %v", err)
	}

	if _, ok := data["entrypoints"]; !ok {
		t.Errorf("expected 'entrypoints' key in bundle JSON")
	}
	if _, ok := data["topPackages"]; !ok {
		t.Errorf("expected 'topPackages' key in bundle JSON")
	}
}
```

- [ ] **Step 2: Run all tests across the repository**

Run: `rtk go test ./...`  
Expected: PASS (all packages)

- [ ] **Step 3: Run linter**

Run: `rtk golangci-lint run`  
Expected: `golangci-lint: No issues found`

- [ ] **Step 4: Commit Task 6**

```bash
rtk git add test/
rtk git commit -m "test(e2e): add end-to-end integration test for Studio web server"
```

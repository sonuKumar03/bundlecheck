# Performance & Architecture Roadmap: `bundlecheck`

**Last updated**: 2026-09-19  
**Status**: Active Architecture & Roadmap Specification

---

## 1. Executive Summary

`bundlecheck` is a high-speed, zero-dependency bundle analysis tool and AI agent skill engineered specifically for Angular esbuild outputs and Nx enterprise workspaces.

Following the comprehensive performance audit in `docs/cli-performance-audit.md`, this document outlines:
1. **Completed Performance Enhancements (Stage 1)**: Algorithmic optimizations eliminating quadratic bottlenecks.
2. **Upcoming Engine Optimizations (Stage 2)**: Parser, discovery, and fast-path metadata reader improvements.
3. **Product & Feature Roadmap (Stage 3)**: Interactive visualizer reports, CI action integrations, and multi-bundler expansion.

---

## 2. Performance Engineering & Status

### Stage 1: Algorithmic & Pipeline Optimizations *(Completed & Staged)*

| Optimization | Target Package | Before | After | Impact |
| :--- | :--- | :---: | :---: | :---: |
| **Single-Pass Gzip Attribution** | `internal/compression/gzip.go` | 10.49 ms / 4.8 MB | **0.113 ms / 53 KB** | **92x speedup**, 98.9% less memory churn ($O(P \times C) \rightarrow O(C)$) |
| **Pre-Indexed Graph & BFS Tree** | `internal/graph/trace.go` & `internal/advisor/advisor.go` | 49.64 ms / 64.1 MB | **0.856 ms / 1.19 MB** | **58x speedup**, 98.1% less memory churn ($O(V + E)$ single BFS pass) |
| **Conditional On-Demand Compression** | `cmd/` (`summary`, `check`, `suggest`, `workspace`) | ~20.29 ms | **~5.56 ms** | Core pipeline ~3.6x faster for raw checks & non-gzip text summaries |

---

### Stage 2: Engine & Parser Optimizations *(Next Up)*

```mermaid
flowchart TD
    A[Angular stats.json + browser dist] --> B[Single-Pass Parser & Validator]
    B --> C[O(1) Candidate Indexer]
    C --> D[Memory Snapshot Model]
    D --> E{Analysis Mode}
    E -->|Summary / Text| F[Fast In-Memory Summarizer]
    E -->|Suggest / Why| G[Pre-Indexed Graph & BFS Tree]
    E -->|Gzip Requested| H[Single-Pass Gzip Engine]
```

#### 1. Fast-Path $O(N)$ Candidate Matcher
- **Target**: `internal/discovery/discovery.go` (`FindCandidates`)
- **Current Issue**: Nested comparison loops between discovered `stats.json` files and candidate browser directories scale quadratically ($O(S \times D)$).
- **Target Architecture**: Map discovered parent paths to pre-index directory trees, resolving co-located and standard Angular/Nx build artifact layouts in a single $O(N)$ sweep.

#### 2. Single-Pass JSON Decoder & Structural Validator
- **Target**: `internal/angular/parser.go`
- **Current Issue**: Decoding `stats.json` into typed structs followed by a second unmarshaling into `map[string]any` to enforce schema constraints.
- **Target Architecture**: Stream or decode once with integrated validation checks for non-negative bytes, valid input references, and required structure.

#### 3. Fast-Path Nx Static Metadata Extraction *(Completed)*
- **Target**: `internal/workspace/nx.go` & `internal/workspace/static.go`
- **Status**: Completed — achieved sub-millisecond execution (~0.26ms, 46KB / 410 allocs), reduced from ~260ms via Node.js Nx CLI (~1000x speedup).
- **Previous Issue**: Executing `node node_modules/nx/bin/nx.js graph --print` incurred ~260ms of Node.js VM startup and graph compilation overhead.
- **Delivered Architecture**: Pure Go zero-dependency static parser scanning `project.json` manifests via bounded traversal (depth <= 4), with transparent automatic fallback to `node nx graph --print` if dynamic plugins, unsupported targets, or zero applications are encountered.

#### 4. Documentation Benchmark Qualification
- **Target**: `README.md` and `npm/bundlecheck/README.md`
- **Target Architecture**: Document realistic native execution ranges (sub-10ms standard bundle analysis, <1ms graph query & attribution, excluding external Node.js subprocess invocation).

---

## 3. Product & Feature Roadmap

### 1. Standalone Interactive HTML Treemap Visualizer
- **Command**: `bundlecheck summary --format html --output report.html`
- **Capabilities**:
  - Zero-dependency, self-contained SVG/Canvas interactive treemap and zoomable sunburst partition chart.
  - Interactive drill-down from initial vs lazy chunks down to individual NPM packages and source files.
  - Built-in Gzip vs Raw toggle, search filter, and inline optimization recommendations.

### 2. GitHub Actions PR Automation & Sticky Comment Mode *(Completed)*
- **Command**: `bundlecheck measure --format github-pr` (and `check --format github-pr`)
- **Capabilities**:
  - Embedded sticky comment marker (`<!-- bundlecheck-comment -->`) for seamless in-place PR comment updates.
  - Visual Unicode/ASCII diff progress bars (`[████████░░]`) and % changes.
  - Foldable `<details>` breakdowns for changed and unchanged package contributions.
  - Automatic status badges for reductions, regressions, and threshold/budget failures.

### 3. Named Baselines Worktree Synchronization
- **Command**: `bundlecheck baseline create <name> --from-git <ref>`
- **Capabilities**:
  - Automatically spin up a temporary git worktree at a reference branch/tag (e.g. `origin/main`), build or read existing artifacts, record the named baseline, and tear down the worktree cleanly.

### 4. Multi-Bundler & Ecosystem Expansion
- **Target**: Add first-class normalization adapters for:
  - Vite / Rollup `stats.json` (Rollup Visualizer plugin format).
  - Modern Webpack 5 production stats.

---

## 4. Benchmark Verification Standards

All future performance changes must be validated against the standard verification suite:
1. Microbenchmarks: `BenchmarkAttachCompressionScaling`, `BenchmarkAdvisorScaling`, and `BenchmarkDiscoveryScaling`.
2. Determinism checks: Repeat runs must produce identical byte-for-byte JSON representations.
3. Memory profiling: `go test -benchmem` must ensure zero unnecessary heap allocations in inner traversal loops.

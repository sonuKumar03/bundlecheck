<div align="center">

# ⚡ bundlecheck

**Lightning-fast bundle inspector, dependency tracer, optimization advisor, and CI budget gate for Angular esbuild.**

<br>

[![Release](https://img.shields.io/github/v/release/sonuKumar03/bundlecheck?color=indigo&label=release&logo=github)](https://github.com/sonuKumar03/bundlecheck/releases)
[![CI Status](https://img.shields.io/github/actions/workflow/status/sonuKumar03/bundlecheck/ci.yml?branch=master&label=CI&logo=githubactions)](https://github.com/sonuKumar03/bundlecheck/actions)
[![Go Report](https://img.shields.io/badge/Go-1.27.1+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-blue)](https://github.com/sonuKumar03/bundlecheck/releases)
[![Agent Skill](https://img.shields.io/badge/AI%20Skill-Ready-8A2BE2?style=flat&logo=anthropic)](.agents/skills/bundlecheck/SKILL.md)
[![License](https://img.shields.io/github/license/sonuKumar03/bundlecheck?color=emerald)](LICENSE)

<br>

[**Website & Live Docs**](https://sonukumar03.github.io/bundlecheck/) • [**Why bundlecheck?**](#-why-bundlecheck) • [**Installation**](#-installation) • [**Quick Start**](#-quick-start) • [**Command Reference**](#-command-reference) • [**CI & GitHub Actions**](#-ci--github-actions-integration) • [**AI Agent Skill**](#-ai-agent-skill-integration)

</div>

---

## ⚡ Overview

Modern Angular applications build with **esbuild** for incredible compilation speed. However, esbuild's raw `stats.json` files are massive, complex, and unreadable for quick human inspection or CI pull request reviews.

`bundlecheck` is a compiled Go binary (sub-10ms native execution, <1ms graph query & attribution) and zero-dependency npm tool that turns Angular `stats.json` files into **actionable dependency hierarchies, file-by-file root cause traces, automated optimization suggestions, and hard CI budget gates**.

```text
$ bundlecheck summary dist/my-app/stats.json --gzip --suggest

BUNDLE SUMMARY
-------------------------------------------------------------
Initial JavaScript:   248.50 KB (3 files: main, polyfills, styles) [~74.20 KB gzip]
Lazy Chunks:          812.20 KB (14 route-split chunks)
Total Assets:         1.06 MB

TOP CONTRIBUTING NPM PACKAGES
-------------------------------------------------------------
1. @angular/core       84.20 KB  (33.8% of initial)  [~25.10 KB gzip]
2. rxjs                 42.10 KB  (16.9% of initial)  [~12.40 KB gzip]
3. @angular/common     31.50 KB  (12.6% of initial)  [~9.20 KB gzip]
4. lodash-es            18.40 KB  (7.4% of initial)   [~5.80 KB gzip]
5. tslib                6.20 KB   (2.5% of initial)   [~1.90 KB gzip]

[!] 2 OPTIMIZATION OPPORTUNITIES DETECTED
-------------------------------------------------------------
• moment (72.4 KB): Found in initial JS. Move behind a dynamic import if not critical for first paint.
• Duplicate package 'tslib': Multiple installed copies contribute to initial JS. Run npm dedupe.
```

---

## 🥊 Why bundlecheck?

| Feature | `bundlecheck` | `webpack-bundle-analyzer` | `source-map-explorer` | Standard `angular.json` Budgets |
| :--- | :---: | :---: | :---: | :---: |
| **Execution Speed** | **`< 10ms` (Compiled Go)** | ~3–8 seconds (Node.js) | ~4–10 seconds (Node.js) | Integrated into build |
| **Runtime Dependencies** | **Zero** (Standalone Binary) | ~40+ npm packages | ~30+ npm packages | Node.js |
| **Import Chain Tracer (`why`)** | **Yes (ASCII Tree)** | ❌ No | ❌ No | ❌ No |
| **Optimization Advisor (`suggest`)** | **Yes (Automated Rules)** | ❌ No (Visual only) | ❌ No | ❌ No |
| **Gzip Wire Modeling** | **Yes (`--gzip`)** | Yes | Yes | ❌ Raw bytes only |
| **PR Delta Diffs (`compare`)** | **Yes (Signed +/- KB)** | ❌ No | ❌ No | ❌ No |
| **Headless CI Gating** | **Yes (Exit 0/1)** | ❌ GUI Required | ❌ GUI / HTML | Yes (Limited) |
| **AI Coding Agent Skill** | **Yes (`SKILL.md`)** | ❌ No | ❌ No | ❌ No |

---

## 📦 Installation

### Option 1: NPX / Zero-Install Runner (Recommended for Frontend Devs)

Run instantly without installing Go or compilers:

```bash
# Run on demand
npx bundlecheck summary dist/my-app/stats.json

# Or add to project devDependencies
npm install -D bundlecheck
```

### Option 2: 1-Line Standalone Shell Installer

Downloads the latest precompiled native binary to `$GOBIN` when set, otherwise `/usr/local/bin` (or `~/.local/bin`). No Go installation is needed for release binaries. Running the installer from a source checkout builds that checkout using Go:

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh
```

*To install the binary alongside the AI Agent skill:*
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

### Option 3: Precompiled Multi-Arch Binaries

Download standalone binaries directly from the [**GitHub Releases**](https://github.com/sonuKumar03/bundlecheck/releases/latest):
- 🍏 **macOS Apple Silicon (M1/M2/M3/M4)**: `bundlecheck_*_darwin_arm64.tar.gz`
- 🍏 **macOS Intel**: `bundlecheck_*_darwin_amd64.tar.gz`
- 🐧 **Linux x86_64**: `bundlecheck_*_linux_amd64.tar.gz`
- 🐧 **Linux ARM64**: `bundlecheck_*_linux_arm64.tar.gz`
- 🪟 **Windows x64**: `bundlecheck_*_windows_amd64.zip`

### Option 4: Go Install

```bash
go install github.com/sonuKumar03/bundlecheck@latest
```

---

## 🚀 Quick Start

### 1. Build your Angular application with stats

Add `--stats-json` to your build command (or configure `"statsJson": true` in `angular.json`):

```bash
ng build --configuration production --stats-json
```
*This produces `dist/<project-name>/stats.json`.*

### 2. Inspect with bundlecheck

```bash
# View initial JS vs lazy breakdown & top npm contributors
bundlecheck summary dist/my-app/stats.json

# Inspect the modules and packages inside one emitted chunk
bundlecheck inspect chunk-ABC123.js --stats dist/my-app/stats.json --dist dist/my-app/browser

# Trace why a package was pulled into initial JS
bundlecheck why dist/my-app/stats.json lodash-es

# Get automated optimization recommendations
bundlecheck suggest dist/my-app/stats.json

# Enforce CI size budget (fails with exit code 1 on violation)
bundlecheck check dist/my-app/stats.json --max-initial 250kb --max-total 1.2mb
```

---

## 📖 Command Reference

### 1. `bundlecheck summary`
Calculates accurate initial vs. lazy JavaScript byte totals and ranks all contributing npm packages.

```bash
# Basic summary
bundlecheck summary dist/my-app/stats.json

# Include estimated Gzip wire transfer sizes
bundlecheck summary dist/my-app/stats.json --gzip

# Show top 15 packages and filter by name
bundlecheck summary dist/my-app/stats.json --top 15 --filter @angular

# Export machine-readable JSON (ideal for scripts & agent loops)
bundlecheck summary dist/my-app/stats.json --format json -o summary.json
```

---

### 2. `bundlecheck inspect <chunk>`
Shows the exact module and npm package contributions inside one emitted JavaScript chunk. The target can be its full output path or a unique filename.

```bash
bundlecheck inspect chunk-ABC123.js --stats dist/my-app/stats.json --dist dist/my-app/browser
bundlecheck inspect browser/chunk-ABC123.js --format json
```

---

### 3. `bundlecheck why <package>`
Traces the exact import graph path from entrypoints (`src/main.ts`) down to any bundled file or package.

```bash
# Find why lodash-es is inside your bundle
bundlecheck why dist/my-app/stats.json lodash-es
```

**Example ASCII Tree Output:**
```text
IMPORT CHAIN TRACE: 'lodash-es' (18.40 KB)
-------------------------------------------------------------
src/main.ts
 └── src/app/app.config.ts
      └── src/app/services/analytics.service.ts
           └── node_modules/lodash-es/debounce.js [in chunk: main-C82D.js]

✓ Direct dependency. Package enters through the initial entry point.
```

---

### 4. `bundlecheck suggest`
Scans the bundle against optimization heuristics to suggest concrete refactoring opportunities.

```bash
bundlecheck suggest dist/my-app/stats.json
```

**Built-in Optimization Rules:**
- 🚫 **Initial Third-Party Packages**: Identifies packages in initial JS and suggests dynamic imports when they are not critical for first paint.
- 🧩 **Duplicate Package Copies**: Identifies distinct installed copies contributing to initial JS and suggests deduplication. Stats do not identify package version numbers.
- ⚡ **Eager Feature Routes**: Identifies routed components bundled directly into `main.js` that should use `loadComponent: () => import(...)`.

---

### 5. `bundlecheck baseline` & `measure` (Git Worktrees & Snapshots)
Capture, manage, switch, and compare baseline bundle metrics across git branches without manual branch switching or rebuilding.

```bash
# ─── 1. CAPTURE BASELINE FROM CURRENT BUILD OR GIT BRANCH ───
bundlecheck baseline save                                     # Save current build as active baseline
bundlecheck baseline save --ref release/v2.0 --name rel-v2   # Build branch in isolated worktree

# ─── 2. LIST & SWITCH SAVED BASELINES ───
bundlecheck baseline list                                     # List all saved baselines and active status
bundlecheck baseline use rel-v2                               # Switch active baseline for 'measure'

# ─── 3. REBUILD & UPDATE BASELINES ───
bundlecheck baseline rebuild rel-v2                           # Re-runs worktree build for latest branch commits

# ─── 4. CONTINUOUS LIVE DELTA MEASUREMENT ───
bundlecheck measure                                           # Measure current build against active baseline
bundlecheck measure -b rel-v2 --max-initial-delta 0B          # Fail in CI if initial bundle grows
```

**Example Output (`bundlecheck measure`):**
```text
MEASURING AGAINST ACTIVE BASELINE (rel-v2)
-------------------------------------------------------------
Initial JavaScript:  1.42 MB  -> 1.18 MB  (-240.00 KB / -16.9%) 📉
Lazy Chunks:         3.10 MB  -> 2.95 MB  (-150.00 KB / -4.8%)  📉
Total Bundle Size:   4.52 MB  -> 4.13 MB  (-390.00 KB / -8.6%)  🎉
```

---

### 6. `bundlecheck compare`
Compares saved JSON summary snapshots, or a baseline snapshot against Angular build stats:

```bash
bundlecheck compare .bundlecheck/baseline.json dist/my-app/stats.json
```

---

### 7. `bundlecheck check`
Strict CI budget gate with custom pass/fail exit codes.

```bash
# Enforce initial and total JS size limits
bundlecheck check dist/my-app/stats.json --max-initial 250kb --max-total 1.5mb

# Enforce regression limits against a baseline snapshot
bundlecheck check dist/my-app/stats.json --baseline baseline.json --max-initial-delta 0kb
```
*Exits with status `0` on success, or status `1` when any budget is exceeded.*

---

### Nx workspace intelligence (`bundlecheck workspace summary`)

Build the apps first using the workspace-installed Nx CLI, then compare their existing browser artifacts:

```bash
# Nx builds; bundlecheck never invokes build targets
nx run-many -t build --projects=shop,admin --configuration=production --stats-json
bundlecheck workspace summary --projects shop,admin
bundlecheck workspace summary --format json --output workspace-report.json
bundlecheck workspace summary --format markdown --all
```

The command finds the nearest `nx.json`, or accepts `--root <workspace>`. It uses local Node and the installed Nx package to read `nx graph --print`. Defaults are all supported Angular apps, `--target build`, and `--configuration production`; projects must define that configuration. No dependencies are downloaded by bundlecheck.

Supported builders are `@nx/angular:application`, `@nx/angular:browser-esbuild`, `@angular-devkit/build-angular:application`, `@angular-devkit/build-angular:browser-esbuild`, and `@angular/build:application`. Output paths and configuration overrides determine each app's stats and browser directory, including custom application-builder `{base, browser}` paths. Stats must be directly in the output base or browser directory; ambiguous files fail that app.

Reports include app initial/lazy/total sizes, npm and source-built Nx library contribution matrices, repeated initial contributions, artifact paths, and argument lists for `why`/`suggest` drill-down. Text and Markdown show ten contributors per category by default (`--top` or `--all` overrides); JSON includes every contributor. Library ownership uses emitted input paths and project-root boundaries. Generated files and compiled library paths without authoritative source ownership remain unattributed; contribution totals need not equal bundle totals.

Each analyzed app includes a `freshness` result. `stale-suspected` means a contributing source file, project/workspace configuration, or dependency lockfile is newer than `stats.json`; text and Markdown reports warn users to rebuild. `unknown` means no newer input was found, not that the requested build configuration or Git revision was verified.

Human reports lead with key findings and use readable binary sizes (KiB/MiB) plus each package's share of app startup JS. Framework/runtime contributions appear separately as context; library entries show their own emitted code, excluding imported npm costs. Markdown collapses exact artifact commands into a drill-down section. JSON retains exact byte counts and its existing schema.

Repeated contributions are summed **across app deployments**, not one user's transfer or deduplication savings. Build configuration and Git provenance are not verified.

Missing or failed apps remain in a partial report with `complete: false` and exit code **1**. Their matrix values are `null`, never zero; a successful app with no contribution has zero values. Unsupported applications are listed and skipped by default, but explicitly selecting one fails. Unknown projects, absent configurations, and duplicate artifact ownership also fail. Capture the report even on exit 1; diagnostics go to stderr. Single-app commands and baseline formats are unchanged; workspace baseline management is not included.

---

## 🛡️ CI & GitHub Actions Integration

### Official GitHub Action (`uses: sonuKumar03/bundlecheck@v0.1.2`)

Add automated bundle size budget validation and PR delta comments to `.github/workflows/bundle-size.yml`:

```yaml
name: Bundle Size Guard
on: [pull_request]

permissions:
  contents: read
  pull-requests: write

jobs:
  bundlecheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'npm'

      - run: npm ci
      - run: npx ng build --configuration production --stats-json

      - name: Run bundlecheck & Post PR Report
        uses: sonuKumar03/bundlecheck@v0.1.2
        with:
          stats: dist/my-app/stats.json
          max-initial: '250kb'
          max-total: '1.2mb'
          post-comment: true
```

---

## ⚙️ Configuration (`.bundlecheck.yml`)

Persist size budgets and disallowed-package rules at the root of your project. `check` loads the nearest configuration in the current directory or a parent directory; CLI budgets override corresponding configuration budgets. Invalid YAML and unsupported settings fail the check:

```yaml
# .bundlecheck.yml
budgets:
  initial_js_max: 250kb
  total_max: 1.5mb
  max_delta_increase: 25kb

rules:
  disallow_packages:
    - moment
    - lodash
```

---

## 🤖 AI Agent Skill Integration

`bundlecheck` includes an official agent skill definition ([`.agents/skills/bundlecheck/SKILL.md`](.agents/skills/bundlecheck/SKILL.md)) compatible with **Antigravity**, **Claude Code**, **OpenAI Codex**, and **Cursor**.

### Install Agent Skill:
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

Autonomous agents use `bundlecheck` in their inner coding loop to:
1. Capture a baseline before refactoring.
2. Read `bundlecheck suggest --format json` for ranked targets.
3. Trace exact files with `bundlecheck why <package> --format json`.
4. Validate byte reductions before committing code.

---

## 🏗️ Architecture

```text
Angular esbuild stats.json ──────────┐
                                     ▼
browser/ dist + index.html ──► internal/discovery
                                     │
                                     ▼
                             internal/angular (Metafile Parser)
                                     │
                                     ▼
                             internal/artifact (Script Matching)
                                     │
                                     ▼
                             internal/graph (BFS Traversal & 'why' Tracer)
                                     │
                                     ▼
                             internal/compression (Gzip Wire Estimation)
                                     │
                                     ▼
                             internal/advisor (Optimization Engine)
                                     │
                 ┌───────────────────┼───────────────────┐
                 ▼                   ▼                   ▼
           Text Reporter       JSON Reporter       Markdown Reporter
         (CLI Terminal)        (AI Agents)          (GitHub PR / CI)
```

---

## 🧪 Development & Testing

```bash
# Run test suite (171 tests across 14 packages)
go test -v ./...

# Run linter
go vet ./...

# Build binary
go build -o bundlecheck .
```

---

## 📄 License

Released under the **MIT License**. Built with ❤️ for the Angular & developer performance community.

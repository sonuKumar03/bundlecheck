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

`bundlecheck` is a self-contained Go binary with zero runtime dependencies that turns Angular `stats.json` files into **actionable dependency hierarchies, file-by-file root cause traces, automated optimization suggestions, and hard CI budget gates**. Fast native analysis; reported timings exclude Angular builds and external Nx subprocesses.

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
| **Execution** | **Fast native analysis** | Node.js process | Node.js process | Integrated into build |
| **Runtime Dependencies** | **Zero** (Self-contained binary) | ~40+ npm packages | ~30+ npm packages | Node.js |
| **Import Chain Tracer (`why`)** | **Yes (ASCII Tree)** | ❌ No | ❌ No | ❌ No |
| **Optimization Advisor (`suggest`)** | **Yes (Automated Rules)** | ❌ No (Visual only) | ❌ No | ❌ No |
| **Gzip Wire Modeling** | **Yes (`--gzip`)** | Yes | Yes | ❌ Raw bytes only |
| **PR Delta Diffs (`compare`)** | **Yes (Signed +/- KB)** | ❌ No | ❌ No | ❌ No |
| **Headless CI Gating** | **Yes (Exit 0/1)** | ❌ GUI Required | ❌ GUI / HTML | Yes (Limited) |
| **AI Coding Agent Skill** | **Yes (`SKILL.md`)** | ❌ No | ❌ No | ❌ No |

---

## 📦 Installation

### Option 1: 1-Line Standalone Shell Installer

Downloads the latest precompiled native binary to `$GOBIN` when set, otherwise `/usr/local/bin` (or `~/.local/bin`). No Go installation is needed for release binaries. Running the installer from a source checkout builds that checkout using Go:

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh
```

*To install the binary alongside the AI Agent skill:*
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

### Option 2: Precompiled Multi-Arch Binaries

Download standalone binaries directly from the [**GitHub Releases**](https://github.com/sonuKumar03/bundlecheck/releases/latest):
- 🍏 **macOS Apple Silicon (M1/M2/M3/M4)**: `bundlecheck_*_darwin_arm64.tar.gz`
- 🍏 **macOS Intel**: `bundlecheck_*_darwin_amd64.tar.gz`
- 🐧 **Linux x86_64**: `bundlecheck_*_linux_amd64.tar.gz`
- 🐧 **Linux ARM64**: `bundlecheck_*_linux_arm64.tar.gz`
- 🪟 **Windows x64**: `bundlecheck_*_windows_amd64.zip`

### Option 3: Go Install

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

# Scope analysis to a specific entrypoint by source file or chunk glob
bundlecheck summary dist/my-app/stats.json --entry src/main.ts
bundlecheck summary dist/my-app/stats.json -e "main-*.js"

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

# Scope summary to a specific entrypoint (source path or emitted chunk glob)
bundlecheck summary dist/my-app/stats.json --entry src/main.ts
bundlecheck summary dist/my-app/stats.json -e "main-*.js"

# Include estimated Gzip wire transfer sizes
bundlecheck summary dist/my-app/stats.json --gzip

# Show top 15 packages and filter by name
bundlecheck summary dist/my-app/stats.json --top 15 --filter @angular

# Export machine-readable JSON (ideal for scripts & agent loops)
bundlecheck summary dist/my-app/stats.json --format json -o summary.json
```

> **Entrypoint Scoping & TotalJS Invariant:** Using `--entry` / `-e` with either a source path (e.g. `src/main.ts`) or an emitted chunk glob (e.g. `main-*.js`, `worker.js`) scopes initial versus lazy reachability, package attribution, and root traces strictly to the selected entrypoint. The overall `TotalJS` metric consistently reflects the whole browser build across all chunks.

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

# Trace import path starting from a specific entrypoint
bundlecheck why dist/my-app/stats.json lodash-es --entry src/main.ts
bundlecheck why dist/my-app/stats.json lodash-es -e "main-*.js"
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
bundlecheck suggest dist/my-app/stats.json --entry src/main.ts
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
bundlecheck baseline save --entry src/main.ts                 # Save baseline scoped to an entrypoint
bundlecheck baseline save --ref release/v2.0 --name rel-v2   # Build branch in isolated worktree

# ─── 2. LIST & SWITCH SAVED BASELINES ───
bundlecheck baseline list                                     # List all saved baselines and active status
bundlecheck baseline use rel-v2                               # Switch active baseline for 'measure'

# ─── 3. REBUILD & UPDATE BASELINES ───
bundlecheck baseline rebuild rel-v2                           # Re-runs worktree build for latest branch commits

# ─── 4. CONTINUOUS LIVE DELTA MEASUREMENT ───
bundlecheck measure                                           # Measure current build against active baseline
bundlecheck measure --entry src/main.ts                       # Measure scoped to entrypoint
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

# Scope size budgets to a specific entrypoint by source file or chunk glob
bundlecheck check dist/my-app/stats.json --entry src/main.ts --max-initial 250kb
bundlecheck check dist/my-app/stats.json -e "main-*.js" --max-initial 250kb
bundlecheck check dist/my-app/stats.json -e "worker.js" --max-initial 100kb

# Enforce regression limits against a baseline snapshot
bundlecheck check dist/my-app/stats.json --baseline baseline.json --max-initial-delta 0kb
```
*Exits with status `0` on success, or status `1` when any budget is exceeded.*

---

### 8. `bundlecheck init`
Assisted setup to generate or propose a reviewable `.bundlecheck.yml` configuration:

```bash
# Preview proposed budgets with 5% headroom over measured size
bundlecheck init

# Import budgets directly from angular.json
bundlecheck init --from-angular-budgets

# Save proposed configuration to .bundlecheck.yml
bundlecheck init --write --headroom 10
```
*Creates `.bundlecheck.yml` only if it does not already exist.*

---

### 9. `bundlecheck workspace summary` (Nx & Monorepo Intelligence)

Compare app sizes, shared library costs, and duplicate npm dependencies across an Nx or Angular multi-app workspace:

```bash
# Analyze all applications in workspace
bundlecheck workspace summary

# Select specific projects
bundlecheck workspace summary --projects admin-dashboard,portal

# Export machine-readable JSON or markdown
bundlecheck workspace summary --format json -o workspace-report.json
bundlecheck workspace summary --format markdown --all
```

Supported builders include `@nx/angular:application`, `@nx/angular:browser-esbuild`, `@angular-devkit/build-angular:application`, `@angular-devkit/build-angular:browser-esbuild`, and `@angular/build:application`. Reports include app initial/lazy/total sizes, npm and source-built Nx library contribution matrices, repeated initial contributions, freshness indicators, and drill-down commands.

---

### 10. `bundlecheck mcp` (Model Context Protocol Server for AI Agents)

Launch a native [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server over standard I/O for AI coding assistants (**Claude Code**, **Antigravity**, **Cursor**, **Claude Desktop**).

```bash
bundlecheck mcp
```

**Exposed MCP Tools:**
- `bundle_summary`: Inspects bundle sizes, initial vs. lazy JS breakdown, and ranked npm contributors (with `path`, `project`, `entry`, `top`, `filter`).
- `bundle_why`: Traces dependency import paths from entrypoints to any target module.
- `bundle_suggest`: Proposes prioritized optimization recommendations with estimated byte savings.
- `bundle_check`: Evaluates absolute budgets and disallowed package rules.
- `bundle_measure`: Measures size deltas against baseline snapshots with regression thresholds.
- `workspace_summary`: Analyzes multi-app Nx workspaces, shared libraries, and cross-application duplicate dependencies.

All bundle MCP tools accept `entry`. If `index.html` contains an injected script such as `ENV_polyfills.js` that is absent from `stats.json`, the server retries with the configured Angular `browser`/`main` entry when it matches the stats; otherwise it returns valid entry selectors for an agent retry.

**Exposed MCP Resource:**
- `bundlecheck://rules`: Standard bundle optimization heuristics and modern replacement guidelines for common heavy packages.

---

## 🛡️ CI & GitHub Actions Integration

### Official GitHub Action (`uses: sonuKumar03/bundlecheck@v0.6.0`)

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
        uses: sonuKumar03/bundlecheck@v0.6.0
        with:
          stats: dist/my-app/stats.json
          entry: src/main.ts
          max-initial: '250kb'
          max-total: '1.2mb'
          max-initial-delta: '0B'
          post-comment: true
```

#### Action Inputs & Comparison Behavior

| Input | Default | Description |
|:---|:---:|:---|
| `stats` | *(auto)* | Path to `stats.json` (auto-detected if omitted). |
| `dist` | *(auto)* | Path to emitted `browser` dist with `index.html` (auto-detected if omitted). |
| `project` | `""` | Project name for multi-project or Nx workspaces. |
| `entry` | `""` | Scope bundlecheck analysis and budget enforcement to a specific entrypoint (e.g. `src/main.ts` or `main-*.js`). |
| `artifact-baseline` | `false` | Attempt to restore baseline summary JSON from a GitHub Actions workflow artifact on base-ref. |
| `artifact-name` | `""` | Name of the baseline workflow artifact (defaults to `bundlecheck-baseline` or `bundlecheck-baseline-<project>`). |
| `upload-artifact-baseline` | `false` | Save current bundle summary and upload as an immutable baseline workflow artifact. |
| `github-token` | `github.token` | Token used for downloading baseline artifacts and posting PR comments. |
| `base-ref` | `github.base_ref` | Git ref for baseline comparison in PRs. Automatically fetched in shallow checkouts (`fetch-depth: 1` or `0`). |
| `build-cmd` | `"npm run build"` | Command used to build `base-ref` inside an isolated temporary git worktree. |
| `max-initial-delta` | `""` | Maximum allowed increase in initial JS vs baseline (e.g. `0B`, `10KB`). |
| `max-total-delta` | `""` | Maximum allowed increase in total JS vs baseline. |
| `post-comment` | `false` | Automatically creates or updates a single sticky PR comment with visual diffs. Requires `pull-requests: write`. |

#### Persistent Memory with Workflow Artifacts (Fast & Secure)
Instead of rebuilding the base branch in an isolated Git worktree for every PR, you can enable persistent baseline memory:
1. **On `push` to `main`:** Set `upload-artifact-baseline: true` to save and upload the baseline snapshot as a GitHub workflow artifact.
2. **On `pull_request`:** Set `artifact-baseline: true` to automatically download the immutable baseline artifact using GitHub CLI. If the artifact is not found, it gracefully falls back to the Git worktree build.
3. **Multi-App Monorepos:** In multi-app workspaces, specifying `project: my-app` automatically namespaces the artifact to `bundlecheck-baseline-my-app`, enabling safe parallel matrix builds across applications.

**Fallback & Error Handling:**
- If comparing against an artifact baseline or base ref succeeds, full visual diffs and package deltas are posted to the PR.
- If base ref build fails and **no delta regression budgets** were requested, the Action warns and falls back to current build measurements without failing CI.
- If **delta regression limits** or explicit `base-ref` were requested and base analysis fails, the Action terminates with an error to ensure regression gates are never silently bypassed.

### ☁️ On-Demand Remote Audits
Audit any public open-source Angular repository directly via GitHub Actions without local installation:
1. Navigate to **Actions** → **Remote Bundle Audit**.
2. Click **Run workflow** and input the public repository URL (e.g. `https://github.com/user/angular-app`).
3. View the full bundle breakdown, top packages, and optimization recommendations directly in the **Job Summary**.

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

### Budget Precedence Table

Bundlecheck enforces limits strictly according to the following deterministic precedence:

| Priority | Source | Description |
|:---:|:---|:---|
| **1 (Highest)** | **CLI Flags** | Explicit command-line arguments (e.g., `--max-initial 200KB`, `--max-total 1MB`) override all configuration values. |
| **2** | **Explicit Config** | Configuration file specified explicitly via `--config <path>`. |
| **3** | **Auto-Loaded Config** | Automatically discovered `.bundlecheck.yml` / `.bundlecheck.yaml` in current or parent directory. |
| **4** | **Imported Budgets** | Budgets imported from `angular.json` (e.g. via `bundlecheck init --from-angular-budgets`). |
| **5 (Lowest)** | **No Limit** | Default for unconfigured projects: report-only mode with zero invented failures. |

### Outcome Semantics

| Outcome | Exit Status | Description |
|:---|:---:|:---|
| **Pass** | `0` | All configured budgets and package rules passed, or project was evaluated in unconfigured report-only mode. Full metrics and summary are output. |
| **Policy Violation** | `1` | One or more thresholds or disallowed package rules were breached. Detailed violations are printed alongside actual vs limit values, while preserving full bundle breakdown. |
| **Invalid Configuration** | `1` | Malformed YAML, unparseable byte values, or unknown configuration fields are rejected before analysis begins. |
| **Execution Failure** | `1` | Missing build artifacts, unparseable stats JSON, or missing baseline file prevents analysis from completing. |

---

## 🤖 AI Agent Integration

Give your coding agent a measured bundle optimization loop:

**baseline → diagnose → trace → edit → rebuild/test → measure → gate**

- **Agent Skill**: the reasoning and optimization workflow.
- **MCP**: the preferred structured tool interface when available.
- **CLI JSON**: the universal fallback, using `--format json`.

The installed `bundlecheck` binary and Angular esbuild `stats.json` are prerequisites. The Skill chooses MCP or CLI transport without changing the workflow.

### MCP server

Add the preferred structured interface to your agent's MCP configuration:

**Claude Code:**
```bash
claude mcp add bundlecheck -- bundlecheck mcp
```

**Claude Desktop / Cursor (`claude_desktop_config.json` / `.cursor/mcp.json`):**
```json
{
  "mcpServers": {
    "bundlecheck": {
      "command": "bundlecheck",
      "args": ["mcp"]
    }
  }
}
```

### Agent Skill ([`SKILL.md`](.agents/skills/bundlecheck/SKILL.md))

Install the binary and complete skill tree for generic agents, Claude Code, and Codex:
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

Use `--skill-dir <path>` to install only to an explicit custom skill location.

Try this prompt:

> Reduce the initial JavaScript bundle by at least 50 KB without changing
> application behavior. Establish a baseline first, identify the highest-confidence
> optimization, trace its import path, make the change, rebuild, run tests, and
> report the measured delta.

For complete agent documentation and JSON contracts, see [**docs/agents.md**](docs/agents.md).

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
# Run test suite (296 tests across 19 packages)
go test -v ./...

# Run linter
go vet ./...

# Build binary
go build -o bundlecheck .
```

---

## 📄 License

Released under the **MIT License**. Built with ❤️ for the Angular & developer performance community.

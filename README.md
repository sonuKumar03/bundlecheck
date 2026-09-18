<div align="center">

# 📦 bundlecheck

**Blazing-fast bundle analyzer, optimization advisor, dependency tracer, and CI budget tool for Angular.**

[![CI](https://github.com/sonuKumar03/bundlecheck/actions/workflows/ci.yml/badge.svg)](https://github.com/sonuKumar03/bundlecheck/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Agent Skill](https://img.shields.io/badge/Agent%20Skill-Ready-8A2BE2?style=flat)](.agents/skills/bundlecheck/SKILL.md)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

[Features](#-key-features) • [Installation](#-installation) • [Quick Start](#-quick-start) • [Command Reference](#-command-reference) • [CI & PR Reporting](#-ci--github-actions-integration) • [AI Agent Skill](#-ai-agent-skill-integration)

</div>

---

## ⚡ Overview

Angular's esbuild-based application builder produces rich `stats.json` metafiles, but interpreting them manually or catching bundle regressions in CI is tedious. 

`bundlecheck` is a native Go CLI and AI agent skill that turns raw Angular build artifacts into **actionable insights, root-cause dependency trees, automated optimization suggestions, and GitHub PR size diffs** in milliseconds.

```text
$ bundlecheck summary --suggest --gzip

## 📦 Angular Bundle Summary

| Category | Raw Size | Wire Size (Gzip) |
| :--- | :--- | :--- |
| **Initial JS** | `160 KB` | `51.2 KB` |
| **Lazy JS** | `300 KB` | `96.0 KB` |
| **Total JS** | `460 KB` | `147.2 KB` |

### 💡 Bundle Optimization Recommendations
> Potential Initial JS Reduction: ~100 KB across 2 opportunities

[HIGH] #1: Move 'pdfjs-dist' behind a dynamic import
  Potential Savings:  ~50 KB (~51.2 KB gzip)
  Target / Importer:  src/main.ts
  Action:             Replace 'import ... from "pdfjs-dist"' with 'const lib = await import("pdfjs-dist")'
```

---

## ✨ Key Features

| Feature | Description |
| :--- | :--- |
| 🔍 **Dependency Tracer (`why`)** | Traces exact static import paths from entrypoints (`src/main.ts`) down to any module with clean ASCII trees. |
| 💡 **Optimization Advisor (`suggest`)** | Detects heavy initial packages, eager feature routes, and duplicate packages with estimated byte savings. |
| 🗜️ **Gzip Wire Sizing (`--gzip`)** | Measures real physical Gzip compression and projects accurate wire transfer sizes per package. |
| 📊 **Baseline & Measure (`measure`)** | Captures baselines to `.bundlecheck/baseline.json` and tracks signed before/after deltas across refactors. |
| 🛡️ **CI Size Budgets (`check`)** | Enforces maximum bundle budgets and regression limits (`--max-initial-delta 0B`) in CI pipelines. |
| 📝 **PR Markdown Reporter (`-f md`)** | Generates formatted GitHub Pull Request comments with collapsible sections and visual status badges. |
| 🤖 **AI Agent Skill (`$bundlecheck`)** | Ships with universal agent skill definitions for Antigravity CLI, Claude Code, Cursor, and Codex. |
| ⚡ **Zero-Config Auto-Discovery** | Automatically detects Angular `stats.json` and matching `browser/index.html` across workspaces. |

---

## 🚀 Installation

### Option 1: One-Line Installer (CLI + AI Agent Skill)

Install the CLI binary and the AI Agent skill in a single command without cloning:

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

This installs `bundlecheck` to `$GOBIN` (or `$HOME/go/bin`) and installs the skill to `$HOME/.agents/skills/bundlecheck/SKILL.md` (and `$HOME/.gemini/antigravity-cli/skills/bundlecheck`).

### Option 2: Go Install

```bash
go install github.com/sonuKumar03/bundlecheck@latest
```

### Option 3: From Local Source

```bash
git clone https://github.com/sonuKumar03/bundlecheck.git
cd bundlecheck
./install.sh --with-skill
```

---

## 🚦 Quick Start

### 1. Build your Angular application with stats

```bash
ng build --configuration production --stats-json
```

### 2. Run bundlecheck

`bundlecheck` auto-discovers build outputs in `dist/`:

```bash
# Summary with Gzip sizes and optimization suggestions
bundlecheck summary --suggest --gzip

# Trace why a package is bundled in initial JS
bundlecheck why lodash

# Capture a reference baseline
bundlecheck baseline

# Make your Angular refactorings (@defer, dynamic imports, lazy routes)...

# Measure the exact byte delta against the baseline
bundlecheck measure
```

---

## 📖 Command Reference

### 1. `bundlecheck summary`
Analyzes initial vs. lazy chunks and attributes byte contributions to NPM packages.

```bash
# Basic summary
bundlecheck summary

# Include Gzip wire transfer estimation
bundlecheck summary --gzip

# Include immediate optimization recommendations
bundlecheck summary --suggest --gzip

# Filter packages and show top contributors
bundlecheck summary --filter angular --top 10

# Export clean JSON
bundlecheck summary --format json -o bundle-summary.json
```

---

### 2. `bundlecheck suggest`
Automated Optimization Advisor that identifies concrete refactoring opportunities to shrink initial JavaScript.

```bash
# View prioritized optimization suggestions
bundlecheck suggest

# Filter suggestions with a minimum savings threshold
bundlecheck suggest --min-savings 10KB --gzip

# Structured JSON for AI coding agents
bundlecheck suggest --format json
```

**Diagnostic Rules Included:**
- `heavy-initial-package`: Identifies large third-party libraries (e.g. `lodash`, `moment`, `xlsx`, `pdfjs-dist`) in the initial bundle that should be dynamically loaded.
- `eager-feature-component`: Flags feature components bundled in `main.js` that should use `loadComponent: () => import(...)`.
- `split-package`: Identifies libraries duplicated across initial and lazy chunks.

---

### 3. `bundlecheck why <package>`
Traces the static import chain from entrypoints (`src/main.ts`) to the target package or file.

```bash
# Trace import paths
bundlecheck why lodash

# Trace only initial bundle import paths with JSON output
bundlecheck why @angular/material --initial-only --format json
```

**Example ASCII Tree Output:**
```text
Dependency Trace for "lodash"
Initial JS:  128 B
Lazy JS:     0 B
Total JS:    128 B

Import Chain(s) (1 found):

#1 (chunk: main.js - INITIAL, size: 128 B)
src/main.ts
  └── src/app/app.component.ts
        └── node_modules/lodash/lodash.js
```

---

### 4. `bundlecheck baseline` & `measure`
Iterative refactoring workflow to measure the exact impact of code changes without manually managing temporary files.

```bash
# Step 1: Capture baseline before refactoring
bundlecheck baseline

# Step 2: Make code changes and rebuild
ng build --configuration production --stats-json

# Step 3: Measure signed before/after deltas
bundlecheck measure

# Enforce that initial JS did not grow (fails with exit code 1 on regression)
bundlecheck measure --max-initial-delta 0B
```

---

### 5. `bundlecheck compare`
Compares two arbitrary saved summary snapshots.

```bash
bundlecheck compare --before before.json --after after.json
bundlecheck compare --before before.json --after after.json --format json
```

---

### 6. `bundlecheck check`
Validates static size budgets and regression thresholds in CI pipelines.

```bash
# Validate static size budgets
bundlecheck check --max-initial 250KB --max-total 1MB

# Validate against a saved baseline
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 10KB
```

*Returns exit code `0` on pass, exit code `1` on budget violation.*

---

## 🛡️ CI & GitHub Actions Integration

### Method 1: Official GitHub Action (`uses: sonuKumar03/bundlecheck@v0.1.0`)

The fastest way to add bundle analysis, size budgets, and automatic PR commenting to any Angular repository:

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

      - name: Install dependencies & build
        run: |
          npm ci
          npx ng build --configuration production --stats-json

      - name: Run BundleCheck & Post PR Report
        uses: sonuKumar03/bundlecheck@v0.1.0
        with:
          post-comment: 'true'
          max-initial: '350KB'
          max-total: '1.2MB'
```

### Method 2: Direct CLI in CI Scripts

```yaml
- name: Install bundlecheck
  run: |
    curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh
    echo "$HOME/go/bin" >> $GITHUB_PATH

- name: Enforce Size Budget & Generate Report
  run: |
    bundlecheck summary --format markdown --gzip --suggest -o bundle-report.md
    bundlecheck check --max-initial 350KB --max-total 1.2MB
```

---

## 🤖 AI Agent Skill Integration

`bundlecheck` is designed from the ground up for AI coding assistants (**Antigravity CLI**, **Claude Code**, **OpenAI Codex**, **Cursor**).

### Install the Skill:
```bash
./install.sh --with-skill
```

### Example Prompt for Agents:
```text
Use $bundlecheck to baseline the Angular build, run suggestions, trace heavy initial imports with why, and measure the savings after refactoring.
```

The agent uses:
1. `bundlecheck baseline` to record the initial state.
2. `bundlecheck suggest --format json` to receive ranked refactoring targets with estimated savings.
3. `bundlecheck why <package> --format json` to find the exact import paths to refactor.
4. `bundlecheck measure --format json` to verify actual byte reductions.

---

## 🏗️ Architecture

```text
Angular/esbuild stats.json ──────────┐
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
                             internal/compression (Gzip Wire Sizing)
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

Requires Go 1.27.1 or newer.

```bash
# Run all unit and integration tests (171 tests across 14 packages)
go test -v ./...

# Run static analysis
go vet ./...

# Build binary
go build -o bundlecheck .
```

---

## 📄 License

MIT © Sonu Kumar

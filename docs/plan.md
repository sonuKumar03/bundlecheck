# Architecture & Implementation Plan: `bundlecheck`

**Last updated**: 2026-09-18  
**Status**: Implemented & Verified

---

## Overview & Vision

`bundlecheck` is designed to be both:
1. A **Developer CLI** providing fast, zero-config bundle analysis, automated optimization suggestions (`suggest`), Gzip wire sizing (`--gzip`), baseline tracking, dependency import path tracing (`why`), and CI regression checks.
2. An **AI Agent Skill** providing structured, deterministic byte facts, Angular optimization workflows, root cause discovery, and before/after verification for AI assistants (Antigravity CLI, Claude Code, OpenAI Codex, Cursor, Windsurf).

---

## Core Capabilities

### 1. Automated Optimization Advisor (`bundlecheck suggest` / `summary --suggest`)
- Inspects the bundle graph and input import chains to propose concrete size optimizations:
  - **Dynamic Imports**: Flags heavy third-party utilities in initial JS (e.g. `pdfjs-dist`, `xlsx`, `chart.js`, `moment`, `lodash`) with estimated initial byte savings.
  - **Eager Routes**: Flags page/feature components in `main.js` that should use `loadComponent: () => import(...)`.
  - **Duplicated Packages**: Flags libraries split across initial and lazy chunks.
- Emits prioritized severity tables in text mode and clean JSON for AI coding agents.

### 2. Gzip Wire Transfer Size Estimation (`--gzip` / `-g`)
- Measures actual `gzip` compressed sizes of emitted `.js` files using standard `compress/gzip`.
- Computes effective compression ratios and projects proportional package contributions.
- Displays wire transfer sizes in text mode (`160 KB (51.2 KB gzip)`) and structured JSON.

### 3. Dependency & Import Path Tracer (`bundlecheck why`)
- Traces the static import chains from root entrypoints (`src/main.ts`) down to any target package or module.
- Classifies whether each import chain ends up in **initial** bootstrap JS or **lazy** chunks.
- Emits clean ASCII tree diagrams in text mode and structured JSON (`--format json`) for AI agents.

### 4. Baseline Measurement & Iterative Change Tracking (`measure` workflow)
- **Establish Baseline**: `bundlecheck baseline` analyzes the current build and saves the reference snapshot to `.bundlecheck/baseline.json`.
- **Measure Subsequent Changes**: `bundlecheck measure` automatically compares the current build against `.bundlecheck/baseline.json`.
  - Eliminates the need to manually manage temporary `before.json` / `after.json` files during iterative refactoring.
  - Supports regression limits (e.g. `--max-initial-delta 0B`, `--max-total-delta 50KB`).

### 5. Zero-Config Artifact Auto-Discovery (`internal/discovery`)
- Automatically detects Angular esbuild `stats.json` and matching `browser/index.html` across single-app and multi-project Angular workspaces (`--project <name>`).
- If artifacts are missing, outputs actionable hints (`Run 'ng build --configuration production --stats-json'`).

### 6. Developer Ergonomics & Formatting
- **Short Flag Aliases**: `-s` (`--stats`), `-d` (`--dist`), `-p` (`--project`), `-f` (`--format`), `-o` (`--output`), `-b` (`--before`), `-a` (`--after`), `-g` (`--gzip`).
- **File Output (`-o, --output`)**: Write JSON snapshots or text directly to disk without shell redirection quirks.
- **Budget & CI Enforcement (`bundlecheck check`)**: Validates static budgets or delta budgets against a baseline.

### 7. Markdown PR & CI Reporting (`--format markdown`, `-f md`)
- Emits formatted GitHub Flavored Markdown tables with visual status badges (`🟢`, `⚠️`, `✅`, `❌`, `🔴`, `🟡`).
- Generates collapsible `<details>` blocks for unchanged package sections in PR comparisons.
- Supports all commands: `summary`, `compare`, `measure`, `suggest`, and `check`.

### 8. Multi-Platform AI Agent Skill Integration
- Universal skill definition in `.agents/skills/bundlecheck/SKILL.md` with full optimization playbooks.
- Multi-environment installer in `install.sh` supporting `$HOME/.agents/skills` and `$HOME/.gemini/antigravity-cli/skills`.

---

## Verification & Validation

- `rtk go test -v ./...`: 296 tests passed across 19 packages.
- `rtk go vet ./...`: Passed.
- `rtk go build -o bundlecheck .`: Passed.

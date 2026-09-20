---
name: bundlecheck
description: Use when inspecting Angular esbuild JavaScript sizes or npm contributions, getting optimization recommendations with 'suggest', tracing dependency import paths with 'why', capturing baselines, measuring subsequent changes, comparing saved summary snapshots, comparing Angular apps and shared-library/npm costs in Nx workspaces, or validating size budgets in CI/agent loops.
---

# bundlecheck

Fast bundle analysis, optimization advisor, dependency tracing, and change measurement CLI for Angular esbuild projects.
Use JSON output (`--format json`) for programmatic inspection and automated verification.

## 1. Install or locate

Check `command -v bundlecheck`. If missing, install it from the repository checkout:

```sh
./install.sh --with-skill
```

Installation respects `GOBIN` (or `$HOME/go/bin`). Ensure that directory is on your PATH.

## 2. Agent Iteration & Optimization Playbook

When tasked with optimizing an Angular application's bundle size:

### Step A: Establish Baseline
1. Build production stats: `ng build --configuration production --stats-json`
2. Capture baseline metrics:
   ```sh
   bundlecheck baseline --format json
   ```
   This automatically discovers artifacts and saves the baseline to `.bundlecheck/baseline.json`.

### Step B: Generate Optimization Suggestions
Run the automated advisor to receive ranked, actionable optimization opportunities:
```sh
bundlecheck suggest --format json
```
Or run `bundlecheck summary --suggest --gzip` to see both metrics and recommendations in one command.

### Step C: Trace & Refactor Targets
1. Trace **why** a heavy package is in the initial bundle:
   ```sh
   bundlecheck why <package-name> --format json
   ```
   Inspect `chains[].path` to see the exact sequence of source files that imported the package.

2. Apply targeted Angular refactorings:
   - Dynamic imports: `const lib = await import('<package>')` inside user action handlers or services.
   - Lazy routes: `loadComponent: () => import('./feature.component')` in route definitions.
   - Templates: Wrap heavy visual sections in `@defer (on viewport)`.

### Step D: Measure Subsequent Changes
After modifying code, rebuild and measure the exact impact against the baseline:
```sh
ng build --configuration production --stats-json
bundlecheck measure --format json
```
Check `summary.delta.initialJs`:
- Negative number = initial JS decreased (Success!).
- Positive number = initial JS increased (Regression).

### Step E: Enforce Regression Limits
Verify that the refactor did not introduce unexpected bundle growth:
```sh
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 0B
```

## 3. Command Reference

### Summary (`bundlecheck summary`)
```sh
# Zero-config auto-discovery
bundlecheck summary --format json

# Multi-app workspace: specify project positionally
bundlecheck summary portal
bundlecheck summary portal --format json

# Summary with Gzip wire transfer sizing
bundlecheck summary --gzip

# Summary with immediate optimization suggestions
bundlecheck summary --suggest --gzip
```

### Model Context Protocol Server (`bundlecheck mcp`)
For agents connecting via MCP over stdio:
```sh
bundlecheck mcp
```
Provides 6 tools (`bundle_summary`, `bundle_why`, `bundle_suggest`, `bundle_check`, `bundle_measure`, `workspace_summary`) and 1 resource (`bundlecheck://rules`).

### Nx Workspace Summary (`bundlecheck workspace summary`)
```sh
# Build explicitly before inspecting; bundlecheck does not build apps
nx run-many -t build --projects=shop,admin --configuration=production --stats-json
bundlecheck workspace summary --projects shop,admin --format json
```

Requires local Node/Nx. Finds the nearest `nx.json`; `--root`, `--target`, and `--configuration` override defaults. Only Angular application/browser-esbuild builders are supported. Use reported exact stats/dist paths for single-app drill-down and baselines; workspace baseline management is not included.

Human reports lead with key findings, readable sizes, and per-app startup shares; framework/runtime costs are separate context. Library sizes exclude imported npm dependencies. Use JSON for exact bytes.

Inspect `apps[]`, `packages[]`, `libraries[]`, and `findings[]`. Each analyzed app has `freshness.status`: `stale-suspected` means an input timestamp is newer than `stats.json`, while `unknown` is not proof of freshness. Library bytes come from emitted source contributions, not graph edges. Sums represent separate deployments, not deduplication savings. Build configuration and Git provenance are not verified. Keep unattributed/generated/compiled-library contributions unassigned.

A partial report exits 1 with `complete: false`; JSON remains available. Failed apps have `null` matrix values, while absent contributions in analyzed apps have zero values. Do not discard partial JSON or treat missing apps as zero. Unsupported apps are skipped by default; selecting one explicitly fails.

### Optimization Suggestions (`bundlecheck suggest`)
```sh
# Ranked recommendations
bundlecheck suggest

# Filter by minimum initial savings threshold
bundlecheck suggest --min-savings 5KB --gzip

# JSON format for automated agent planning
bundlecheck suggest --format json
```

### Trace Dependency Path (`bundlecheck why`)
```sh
# Trace why a package is in the bundle
bundlecheck why lodash

# Trace only initial bundle import paths in JSON
bundlecheck why @angular/material --initial-only --format json
```

### Inspect One Chunk (`bundlecheck inspect`)
```sh
# Show exact module and npm package contributions for one emitted chunk
bundlecheck inspect chunk-ABC123.js

# Use a full output path when filenames are ambiguous
bundlecheck inspect browser/chunk-ABC123.js --format json
```

### Baseline (`bundlecheck baseline`)
```sh
# Captures current build and writes to .bundlecheck/baseline.json
bundlecheck baseline
```

### Measure (`bundlecheck measure`)
```sh
# Compares current build vs .bundlecheck/baseline.json
bundlecheck measure --format json

# Fails if initial JS increased
bundlecheck measure --max-initial-delta 0B
```

### Compare (`bundlecheck compare`)
```sh
# Compares two arbitrary saved snapshots
bundlecheck compare --before before.json --after after.json --format json
```

### Check (`bundlecheck check`)
```sh
# Static size budget validation
bundlecheck check --max-initial 250KB --max-total 1MB

# Regression check against baseline
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 10KB
```

### PR and CI Markdown Reporting (`--format markdown`)
Generate ready-to-post GitHub Pull Request comments and CI summaries:
```sh
# Bundle summary in markdown table
bundlecheck summary --format markdown --gzip --suggest -o pr-comment.md

# Before/after comparison table with collapsible unchanged sections
bundlecheck compare --before before.json --after after.json --format markdown

# Measure current build against baseline as markdown
bundlecheck measure --format markdown -o pr-summary.md

# Budget check in markdown
bundlecheck check --max-initial 250KB --format markdown
```

## 4. JSON Schema Contract

### Summary JSON (`command: "summary"`)
- `summary.initialJs`, `summary.initialGzipJs`: Integer bytes of initial bundle.
- `summary.lazyJs`, `summary.lazyGzipJs`: Integer bytes of lazy chunks.
- `summary.totalJs`, `summary.totalGzipJs`: Total JavaScript bytes.
- `packages[]`: List of attributed packages with initial, lazy, and total bytes.

### Suggestions JSON (`command: "suggest"`)
- `suggestions[].rule`: `"heavy-initial-package"`, `"eager-feature-component"`, `"split-package"`.
- `suggestions[].severity`: `"HIGH"`, `"MEDIUM"`, `"LOW"`.
- `suggestions[].savingsBytes`, `savingsGzipBytes`: Estimated initial JS byte savings.
- `suggestions[].action`: Concrete refactoring guidance.
- `totalPotentialSavings`: Aggregated estimated savings in initial JS.

### Trace JSON (`command: "why"`)
- `target`, `packageName`: Queried module / package.
- `initialBytes`, `lazyBytes`, `totalBytes`: Byte sizes attributed to target.
- `chains[]`: List of import path chains from root entrypoints to target.

### Comparison JSON (`command: "compare"`)
- `summary.before`, `summary.after`, `summary.delta`: Each has `initialJs`, `lazyJs`, `totalJs`.
- `packages[].status`: `"added"`, `"removed"`, `"changed"`, or `"unchanged"`.
- `packages[].delta`: Signed integer byte deltas (negative = savings).

## 5. Error Handling & Troubleshooting
- Always capture exit status, stdout, and stderr separately.
- On exit code `0`, parse stdout as JSON. For workspace summaries, also parse available JSON on exit `1` and inspect `complete` and app diagnostics.
- On nonzero exit code, report stderr and diagnose:
  - If artifacts are missing, run `ng build --configuration production --stats-json`.
  - In multi-project monorepos, specify `--project <name>`.

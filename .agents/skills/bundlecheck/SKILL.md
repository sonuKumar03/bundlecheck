---
name: bundlecheck
description: Use when inspecting Angular esbuild JavaScript sizes or npm contributions, capturing baselines, measuring subsequent changes, comparing saved summary snapshots, or validating size budgets in CI/agent loops.
---

# bundlecheck

Fast bundle analysis and change measurement CLI for Angular esbuild projects.
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

### Step B: Identify Heavy Initial Contributors
Inspect `packages[]` in the summary JSON or run:
```sh
bundlecheck summary --top 10
```
Key patterns to investigate for initial bundle reduction:
- Static imports of feature components/routes -> Convert to lazy route `loadComponent: () => import('./...')` or `loadChildren`.
- Heavy third-party libraries in root modules (e.g. `lodash`, `moment`, `chart.js`, `pdfjs`) -> Convert to dynamic `await import(...)` inside event handlers or service methods.
- Heavy template elements -> Use Angular `@defer (on viewport)` blocks.

### Step C: Measure Subsequent Changes
After modifying code, rebuild and measure the exact impact against the baseline:
```sh
ng build --configuration production --stats-json
bundlecheck measure --format json
```
Check `summary.delta.initialJs`:
- Negative number = initial JS decreased (Success!).
- Positive number = initial JS increased (Regression).

### Step D: Enforce Regression Limits
Verify that the refactor did not introduce unexpected bundle growth:
```sh
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 0B
```

## 3. Command Reference

### Summary (`bundlecheck summary`)
```sh
# Zero-config auto-discovery
bundlecheck summary --format json

# Explicit paths and direct file export
bundlecheck summary --stats ./dist/app/stats.json --dist ./dist/app/browser -o snapshot.json --format json

# Filter packages in human text mode
bundlecheck summary --filter angular --top 10
```

### Baseline (`bundlecheck baseline`)
```sh
# Captures current build and writes to .bundlecheck/baseline.json
bundlecheck baseline

# Custom baseline path
bundlecheck baseline -o .bundlecheck/v1-baseline.json
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

## 4. JSON Schema Contract

### Summary JSON (`command: "summary"`)
- `schemaVersion`: `"1"`
- `toolVersion`: `"0.1.0"`
- `summary.initialJs`: Integer bytes in the static closure of local script roots in `index.html`.
- `summary.lazyJs`: Integer bytes of every other browser JS output.
- `summary.totalJs`: `initialJs + lazyJs`.
- `packages[]`: List of attributed packages sorted by initial bytes descending, then name ascending.

### Comparison JSON (`command: "compare"`)
- `summary.before`, `summary.after`, `summary.delta`: Each has `initialJs`, `lazyJs`, `totalJs`.
- `packages[].status`: `"added"`, `"removed"`, `"changed"`, or `"unchanged"`.
- `packages[].before`, `packages[].after`, `packages[].delta`: Each has `initialBytes`, `lazyBytes`, `totalBytes`.
- Deltas are **after minus before**: negative numbers indicate byte savings.

## 5. Error Handling & Troubleshooting
- Always capture exit status, stdout, and stderr separately.
- On exit code `0`, parse stdout as JSON.
- On nonzero exit code, report stderr and diagnose:
  - If artifacts are missing, run `ng build --configuration production --stats-json`.
  - In multi-project monorepos, specify `--project <name>`.

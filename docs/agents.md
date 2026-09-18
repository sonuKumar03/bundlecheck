# Using bundlecheck as an agent

Use bundlecheck to establish bundle-size facts for Angular applications using
an esbuild-based builder. Use `summary` to inspect builds, `baseline` to record reference metrics, `measure` to track changes against the baseline, `compare` to compare saved summary JSON snapshots, and `check` to validate size budgets in automated workflows.

For installation, see [the README](../README.md#install). The examples below
assume `bundlecheck` is on PATH; otherwise invoke the built binary by its path.

To install both the CLI and the [companion skill](../.agents/skills/bundlecheck/SKILL.md),
run `./install.sh --with-skill` from a bundlecheck checkout. The skill is copied
to `$HOME/.agents/skills/bundlecheck/SKILL.md` (and `$HOME/.gemini/antigravity-cli/skills/bundlecheck` when available) for use across Angular projects.

## Auto-Discovery & Zero-Config Usage

When run in an Angular application directory after `ng build --configuration production --stats-json`, bundlecheck automatically discovers `stats.json` and matching browser `dist` directories.

```sh
# Zero-config summary (JSON)
bundlecheck summary --format json

# Or specify paths explicitly
bundlecheck summary --stats ./dist/app/stats.json --dist ./dist/app/browser --format json
```

In multi-project workspaces, use `--project <name>` to select a specific application.

## Iteration & Measurement Workflow

### 1. Capture Baseline
```sh
bundlecheck baseline --format json
```
Saves the initial bundle state to `.bundlecheck/baseline.json`.

### 2. Measure Subsequent Changes
```sh
# Rebuild after making Angular optimizations
ng build --configuration production --stats-json

# Measure exact deltas against the baseline
bundlecheck measure --format json
```
Inspect `summary.delta`:
- `summary.delta.initialJs`: Negative = savings; Positive = growth.
- `packages[].delta`: Shows which packages grew or shrank.

### 3. Enforce Regression Budgets
```sh
# Fails with exit code 1 if initial JS grew
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 0B
```

## JSON Contracts

### Summary JSON (`command: "summary"`)

| Field | Meaning |
| --- | --- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"0.1.0"`. |
| `command` | The string `"summary"`. |
| `summary.initialJs` | Raw bytes of browser JS in the static closure of local script roots in `index.html`. |
| `summary.lazyJs` | Raw bytes of every other selected browser JS output. |
| `summary.totalJs` | `initialJs + lazyJs`. |
| `packages[].name` | npm package owner, including scopes; nested `node_modules` uses innermost owner. |
| `packages[].initialBytes` | Emitted input contributions from this package within initial JS outputs. |
| `packages[].lazyBytes` | Emitted input contributions from this package within lazy JS outputs. |
| `packages[].totalBytes` | `initialBytes + lazyBytes` for this package. |

### Comparison & Measure JSON (`command: "compare"`)

| Field | Meaning |
| --- | --- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"0.1.0"`. |
| `command` | The string `"compare"`. |
| `summary.before`, `summary.after`, `summary.delta` | Each holds `initialJs`, `lazyJs`, and `totalJs`. |
| `packages[].before`, `packages[].after`, `packages[].delta` | Each holds `initialBytes`, `lazyBytes`, and `totalBytes`. |
| `packages[].status` | `"added"`, `"removed"`, `"changed"`, or `"unchanged"`. |

Deltas are **after minus before**, in signed integer bytes.

## Troubleshooting

| Failure | Next check |
| --- | --- |
| `no Angular build artifacts found` | Run `ng build --configuration production --stats-json` first or pass `--stats` and `--dist`. |
| `multiple Angular build outputs found` | Pass `--project <name>` to disambiguate the target project. |
| `baseline file not found` | Run `bundlecheck baseline` to initialize the reference baseline. |
| `budget check failed` | Review `violations[]` in JSON or error table in stderr to see which threshold was breached. |

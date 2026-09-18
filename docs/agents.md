# Using bundlecheck as an agent

Use bundlecheck to establish bundle-size facts for Angular applications using
an esbuild-based builder. Use `summary` to inspect builds, `suggest` to generate optimization recommendations, `why` to trace dependency import chains, `baseline` to record reference metrics, `measure` to track changes against the baseline, `compare` to compare saved summary JSON snapshots, and `check` to validate size budgets in automated workflows.

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

# Summary with Gzip wire transfer estimation
bundlecheck summary --gzip --format json

# Or specify paths explicitly
bundlecheck summary --stats ./dist/app/stats.json --dist ./dist/app/browser --format json
```

In multi-project workspaces, use `--project <name>` to select a specific application.

## Automated Optimization Advisor (`suggest`)

Analyze the bundle to receive prioritized, actionable size optimization recommendations:

```sh
# Get ranked suggestions
bundlecheck suggest --format json

# Summary with suggestions appended
bundlecheck summary --suggest --format json
```

Each suggestion includes:
- `rule`: Category (`heavy-initial-package`, `eager-feature-component`, `split-package`).
- `severity`: Priority level (`HIGH`, `MEDIUM`, `LOW`).
- `savingsBytes`: Estimated byte reduction in initial JS.
- `file`: Source file / importing component.
- `action`: Concrete refactoring guidance.

## Dependency Path Tracing (`why`)

Trace the import path explaining why a package or module is in the bundle:

```sh
# Trace dependency path to lodash
bundlecheck why lodash --format json

# Trace only initial bundle import paths
bundlecheck why @angular/material --initial-only --format json
```

Inspect `chains[].path` to see the sequence of source files leading from root entrypoints (`src/main.ts`) to the target package.

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
Inspect `summary.delta`:
- `summary.delta.initialJs`: Negative = savings; Positive = growth.
- `packages[].delta`: Shows which packages grew or shrank.

### 3. Enforce Regression Budgets
```sh
# Fails with exit code 1 if initial JS grew
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 0B
```

### 4. PR & CI Markdown Comment Generation
```sh
# Generate markdown comments for PRs or CI summaries
bundlecheck summary --format markdown --gzip --suggest -o pr-summary.md
bundlecheck measure --format markdown -o pr-delta.md
bundlecheck check --max-initial 250KB --format markdown
```

## JSON Contracts

### Summary JSON (`command: "summary"`)

| Field | Meaning |
| --- | --- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"0.1.0"`. |
| `command` | The string `"summary"`. |
| `summary.initialJs`, `summary.initialGzipJs` | Raw & Gzip bytes of browser JS in static bootstrap closure. |
| `summary.lazyJs`, `summary.lazyGzipJs` | Raw & Gzip bytes of lazy JS outputs. |
| `summary.totalJs`, `summary.totalGzipJs` | Total uncompressed & Gzip JS bytes. |
| `packages[].name` | npm package owner, including scopes. |
| `packages[].initialBytes` | Emitted input contributions within initial JS outputs. |
| `packages[].lazyBytes` | Emitted input contributions within lazy JS outputs. |
| `packages[].totalBytes` | Combined initial + lazy bytes for this package. |

### Suggestions JSON (`command: "suggest"`)

| Field | Meaning |
| --- | --- |
| `suggestions[].rule` | Diagnosis rule name. |
| `suggestions[].severity` | Priority severity (`HIGH`, `MEDIUM`, `LOW`). |
| `suggestions[].title` | Short summary title. |
| `suggestions[].target` | Targeted package or component file. |
| `suggestions[].file` | Importing file to modify. |
| `suggestions[].savingsBytes` | Estimated initial JS reduction. |
| `suggestions[].action` | Specific code refactoring steps. |
| `totalPotentialSavings` | Combined potential initial JS savings. |

### Trace JSON (`command: "why"`)

| Field | Meaning |
| --- | --- |
| `target` | Queried target package or module path. |
| `packageName` | Normalized package name if recognized. |
| `found` | Boolean indicating presence in the bundle. |
| `initialBytes`, `lazyBytes`, `totalBytes` | Byte sizes attributed to target. |
| `chains[].output` | The output bundle chunk containing this instance. |
| `chains[].initial` | Whether the output chunk is in initial bootstrap closure. |
| `chains[].path` | Ordered array of module file paths from root to target. |
| `chains[].bytesInChunk` | Contributed bytes inside this specific chunk. |

### Comparison & Measure JSON (`command: "compare"`)

| Field | Meaning |
| --- | --- |
| `summary.before`, `summary.after`, `summary.delta` | Each holds `initialJs`, `lazyJs`, and `totalJs`. |
| `packages[].before`, `packages[].after`, `packages[].delta` | Each holds `initialBytes`, `lazyBytes`, and `totalBytes`. |
| `packages[].status` | `"added"`, `"removed"`, `"changed"`, or `"unchanged"`. |

Deltas are **after minus before**, in signed integer bytes.

## Troubleshooting

| Failure | Next check |
| --- | --- |
| `no Angular build artifacts found` | Run `ng build --configuration production --stats-json` first or pass `--stats` and `--dist`. |
| `multiple Angular build outputs found` | Pass `--project <name>` to disambiguate the target project. |
| `target package not found` | Verify spelling or check `bundlecheck summary` for attributed packages. |
| `baseline file not found` | Run `bundlecheck baseline` to initialize the reference baseline. |
| `budget check failed` | Review `violations[]` in JSON or error table in stderr to see which threshold was breached. |

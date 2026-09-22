# Using bundlecheck as an AI Coding Agent

Give your coding agent a measured bundle optimization loop for Angular applications using esbuild:

**baseline → diagnose → trace → edit → rebuild/test → measure → gate**

- **Agent Skill**: the reasoning and optimization workflow.
- **MCP**: the preferred structured tool interface when available.
- **CLI JSON**: the universal fallback, using `--format json`.

The installed `bundlecheck` binary and compatible Angular esbuild `stats.json` are prerequisites. Install the binary and complete skill tree for generic agents, Claude Code, and Codex with:

```sh
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/install.sh | sh -s -- --with-skill
```

Use `--skill-dir <path>` to install only to a custom skill location. Do not assume the BundleCheck source checkout is available.

Example prompt:

> Reduce the initial JavaScript bundle by at least 50 KB without changing
> application behavior. Establish a baseline first, identify the highest-confidence
> optimization, trace its import path, make the change, rebuild, run tests, and
> report the measured delta.

---

## 🔌 Model Context Protocol (MCP) Server

`bundlecheck` includes a native, high-performance MCP server built into the binary:

```sh
bundlecheck mcp
```

### Agent Client Configuration

#### 1. Claude Code
Add to your project's `.claude.json` or run:
```sh
claude mcp add bundlecheck -- bundlecheck mcp
```

#### 2. Antigravity / Gemini CLI
Add to `~/.gemini/antigravity-cli/mcp/bundlecheck.json` (or project MCP settings):
```json
{
  "command": "bundlecheck",
  "args": ["mcp"]
}
```

#### 3. Claude Desktop (`claude_desktop_config.json`)
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

#### 4. Cursor (`.cursor/mcp.json`)
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

---

### Exposed MCP Tools

The MCP server provides 6 typed tools formatted for LLM consumption:

| Tool | Parameters | Description |
| :--- | :--- | :--- |
| `bundle_summary` | `path` (opt), `project` (opt), `entry` (opt), `top` (opt, def 10), `filter` (opt) | Summarizes initial, lazy, and total JS sizes and top npm package contributors. |
| `bundle_why` | `package` (required), `path` (opt), `project` (opt), `entry` (opt), `initial_only` (opt), `max_chains` (opt) | Traces exact static & dynamic import chains from entrypoints to target package. |
| `bundle_suggest` | `path` (opt), `project` (opt), `entry` (opt), `min_savings` (opt) | Generates prioritized, actionable bundle optimization recommendations. |
| `bundle_check` | `path` (opt), `project` (opt), `entry` (opt), `max_initial` (opt), `max_total` (opt), `disallowed_packages` (opt) | Enforces size budgets and verifies disallowed package constraints. |
| `bundle_measure` | `baseline` (required), `path` (opt), `project` (opt), `entry` (opt), `max_initial_delta` (opt) | Measures size deltas against a saved baseline and gates regressions. |
| `workspace_summary`| `root` (opt), `projects` (opt array) | Analyzes all Angular applications and shared libraries across an Nx workspace. |

When `index.html` references a script absent from `stats.json`, bundle tools retry with the `browser` or `main` entry from `angular.json`/Nx `project.json` when it matches the stats. Otherwise the error lists valid selectors so the agent can retry with `entry`.

### Exposed MCP Resources

- **`bundlecheck://rules`** (`application/json`): Standard optimization heuristics, replacement guidelines for heavy libraries (`moment`, `lodash`, `exceljs`, `pdfjs-dist`), and default budget guidelines.

---

## ⚡ Auto-Discovery & Zero-Config CLI Usage

When executed in an Angular application directory after `ng build --configuration production --stats-json`, `bundlecheck` automatically locates `stats.json` and matching browser `dist` directories.

```sh
# Zero-config summary (JSON)
bundlecheck summary --format json

# Summary with Gzip wire transfer estimation
bundlecheck summary --gzip --format json

# Multi-app workspace: specify project positionally or with flag
bundlecheck summary portal
bundlecheck summary --project portal --format json
```

---

## 🧭 Automated Optimization Advisor (`suggest`)

Analyze the bundle to receive prioritized, actionable size optimization recommendations:

```sh
# Get ranked suggestions
bundlecheck suggest --format json

# Summary with suggestions appended
bundlecheck summary --suggest --format json
```

Each suggestion includes:
- `rule`: Diagnostic rule name (`heavy-initial-package`, `eager-feature-component`, `split-package`).
- `severity`: Priority level (`HIGH`, `MEDIUM`, `LOW`).
- `savingsBytes`: Estimated byte reduction in initial JS.
- `file`: Source file / importing component.
- `action`: Concrete refactoring guidance.

---

## 🔍 Dependency Path Tracing (`why`)

Trace the import path explaining why a package or module is in the bundle:

```sh
# Trace dependency path to lodash
bundlecheck why lodash --format json

# Trace in a multi-app monorepo
bundlecheck why portal lodash --format json

# Trace only initial bundle import paths
bundlecheck why @angular/material --initial-only --format json
```

Inspect `chains[].path` to see the sequence of source files leading from root entrypoints (`src/main.ts`) to the target package.

---

## 🏢 Multi-App Monorepos (`workspace summary`)

In Nx or Angular multi-app monorepos:

```sh
# Compare all applications, shared library costs, and duplicated dependencies
bundlecheck workspace summary --format json

# Filter specific applications
bundlecheck workspace summary --projects admin-dashboard,portal --format json
```

---

## 🔄 Iteration & Measurement Playbook for Agents

### 1. Establish Baseline Before Modifying Code
```sh
bundlecheck baseline save --name pre-refactor --format json
```
Saves the initial bundle state to `.bundlecheck/baselines/pre-refactor.json`.

### 2. Trace & Apply Optimization
1. Run `bundlecheck suggest --format json` to find high-impact targets.
2. Run `bundlecheck why <package> --format json` to identify the importing component.
3. Replace heavy modules with lazy loading (`import(...)`, `@defer`, `loadComponent`).

### 3. Measure Changes & Verify
```sh
ng build --configuration production --stats-json
bundlecheck measure -b pre-refactor --format json
```
Inspect `summary.delta`:
- `summary.delta.initialJs`: Negative = savings (success); Positive = regression.
- `packages[].delta`: Shows which packages grew or shrank.

### 4. Gate Regressions
```sh
# Fails with exit code 1 if initial JS grew
bundlecheck measure -b pre-refactor --max-initial-delta 0B
```

---

## 📋 JSON Schema Contracts

### Summary JSON (`command: "summary"`)

| Field | Meaning |
| :--- | :--- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"0.6.2"`. |
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
| :--- | :--- |
| `suggestions[].rule` | Diagnosis rule name (`heavy-initial-package`, `duplicate-package`, etc.). |
| `suggestions[].severity` | Priority severity (`HIGH`, `MEDIUM`, `LOW`). |
| `suggestions[].title` | Short summary title. |
| `suggestions[].target` | Targeted package or component file. |
| `suggestions[].file` | Importing file to modify. |
| `suggestions[].savingsBytes` | Estimated initial JS reduction. |
| `suggestions[].action` | Specific code refactoring steps. |
| `totalPotentialSavings` | Combined potential initial JS savings. |

### Trace JSON (`command: "why"`)

| Field | Meaning |
| :--- | :--- |
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
| :--- | :--- |
| `summary.before`, `summary.after`, `summary.delta` | Each holds `initialJs`, `lazyJs`, and `totalJs`. |
| `packages[].before`, `packages[].after`, `packages[].delta` | Each holds `initialBytes`, `lazyBytes`, and `totalBytes`. |
| `packages[].status` | `"added"`, `"removed"`, `"changed"`, or `"unchanged"`. |
| `findings[]` | Optional deterministic regression findings (`name`, `deltaBytes`, `kind`, `chunks`, `tracePath`, `reason`). |

---

## 🔢 CLI Exit Codes Contract

Integrations and CI scripts can rely on stable, numeric exit codes:

| Code | Name | Meaning |
| :---: | :--- | :--- |
| `0` | Success | Analysis completed successfully, policy satisfied, or report-only check. |
| `1` | Policy Violation | Configured budget threshold was breached or disallowed package detected. |
| `2` | Usage / Config Error | Invalid CLI flags, missing required arguments, or invalid configuration YAML. |
| `3` | Execution Failure | Runtime failure, missing build artifacts, I/O errors, or incomplete workspace report. |

---

## 🛡️ Schema Compatibility & Versioning Guarantees

- **`schemaVersion: "1"`**: Guarantees backwards compatibility for external automation, CI pipelines, and MCP clients.
- **Additive Changes**: New optional fields (such as `findings`, `initialGzipJs`, `gzipBytes`, `drillDown`) may be introduced in minor updates. Automation consumers must accept additive keys without breaking.
- **Breaking Changes**: Modifying existing keys, changing field types, or removing fields will increment `schemaVersion` (e.g., `"2"`).
- **Deterministic Ordering**: Packages, outputs, findings, and trace chains are sorted deterministically with stable secondary tie-breakers across repeated runs.

## 🛠️ Troubleshooting

| Failure | Next check |
| :--- | :--- |
| `no Angular build artifacts found` | Run `ng build --configuration production --stats-json` first or pass `--stats` and `--dist`. |
| `multiple Angular build outputs found` | Specify the project: `bundlecheck summary <project>` or `--project <name>`. |
| `target package not found` | Verify spelling or check `bundlecheck summary` for attributed packages. |
| `baseline file not found` | Run `bundlecheck baseline save` to initialize the reference baseline. |
| `budget check failed` | Review `violations[]` in JSON or error table in stderr to see which threshold was breached. |

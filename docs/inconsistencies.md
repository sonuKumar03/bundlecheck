# Inconsistency Audit & Remediation Plan

This document details all discovered inconsistencies across the `bundleradar` codebase (CLI UX, report formatting, data models, GitHub Action, MCP server, documentation, and error handling) and provides an actionable remediation plan.

## User Review Required

> [!IMPORTANT]
> Some inconsistencies involve public contracts (CLI flags, JSON schema field names, config YAML keys, Action inputs). Changing or aligning them requires deciding between:
> 1. **Additive aliases** (backwards-compatible): Add missing flags/fields while keeping existing names.
> 2. **Breaking cleanups** (major version bump): Consolidate to a single consistent naming standard across all interfaces.
>
> We recommend **additive aliases with deprecation notices** for v0.6.x, preserving backwards compatibility while achieving full consistency.

---

## Audit Findings: Detailed Categorization

### 1. CLI Flags & UX Inconsistencies

| Command | `--format` Options | `--output` Phrasing | `--gzip` | `--entry` | `--project` | Positional Arguments |
| :--- | :--- | :--- | :---: | :---: | :---: | :--- |
| `summary` | `text, json, markdown` | "specified file path instead of stdout" | ✅ Yes | ✅ Yes | ✅ Yes | `[stats.json] [dist]` |
| `check` | `text, json, markdown, github-pr` | "file instead of stdout" | ❌ No | ✅ Yes | ✅ Yes | `[stats.json] [dist]` |
| `measure` | `text, json, markdown, github-pr` | "specified file path instead of stdout" | ❌ No | ✅ Yes | ✅ Yes | **None** (`cobra.NoArgs`) |
| `compare` | `text, json, markdown, github-pr` *(flag help says text, json, markdown)* | "specified file path instead of stdout" | ❌ No | ❌ No | Partial (title only) | `[before.json] <after.json\|stats.json>` |
| `suggest` | `text, json, markdown` | "specified file path instead of stdout" | ✅ Yes | ✅ Yes | ✅ Yes | `[stats.json] [dist]` |
| `why` | `text, json` *(no markdown)* | "specified file path instead of stdout" | ❌ No | ✅ Yes | ✅ Yes | `[stats.json] <package>` *(or `<package>`)* |
| `inspect` | `text, json` *(no markdown)* | "specified file path instead of stdout" | ❌ No | ❌ No | ✅ Yes | `<chunk>` (required) |
| `workspace` | `text, json, markdown` | "report to a file instead of stdout" | ✅ Yes | ❌ No | `--projects` (plural, no `-p`) | **None** (`cobra.NoArgs`) |
| `baseline` | `text, json` *(subcommands vary)* | "Custom path to save baseline JSON file" | ❌ No | ✅ Yes | ✅ Yes | Varies per subcommand (`list`/`show` lack `-o`) |
| `benchmark` | `text, json, markdown` | "specified file path instead of stdout" | ❌ No | ❌ No | ❌ No | `[benchmark-output.txt]` |
| `init` | *(None - YAML preview/write)* | *(None - uses positional & `--write`)* | ❌ No | ❌ No | ✅ Yes | `[path]` |

#### Specific Inconsistencies:
1. **[compare.go:115](file:///Users/sonukumar/project/bundleradar/cmd/compare.go#L115) Help Text Omission**: Flag description states `"Output format: text, json, or markdown"`, but the command accepts and implements `github-pr`.
2. **Markdown Support Gaps**: `inspect` and `why` only support `text` and `json`, whereas all other inspection commands support `markdown`.
3. **`--gzip` Missing from Comparison Workflows**: Neither `measure` nor `compare` supports `--gzip` / `-g`, preventing users from measuring Gzip wire transfer deltas, despite `action.yml` setting `gzip: 'true'` by default.
4. **`--entry` Missing from `compare` and `init`**: A user comparing two builds or initializing `.bundleradar.yml` for an app with a custom entrypoint cannot pass `--entry`.
5. **`--project` in `compare` vs Discovery**: In `cmd/compare.go:147`, `loadSnapshotOrBuildWithSnapshot` passes `""` for project to `resolveBuildArtifactsInDir`, meaning `--project` is ignored for artifact resolution when directories are supplied to `compare`.
6. **Positional Arguments Discrepancy**: `measure` strictly rejects positional arguments (`cobra.NoArgs`), whereas `summary`, `check`, `suggest`, `why`, and `compare` all allow positional build paths.
7. **`workspace summary` Flag Naming**: Uses `--projects` (plural, comma-separated, no `-p` shorthand), whereas all other commands use `--project` / `-p`.

---

### 2. Report Formatting & Calculation Inconsistencies

1. **Lazy JS Regression Collapsing Flaw**:
   - In [internal/report/markdown.go:143](file:///Users/sonukumar/project/bundleradar/internal/report/markdown.go#L143) and [internal/report/github_pr.go:98](file:///Users/sonukumar/project/bundleradar/internal/report/github_pr.go#L98):
     ```go
     delta := r.Summary.Delta.InitialJS
     isMinor := delta <= threshold
     ```
   - `isMinor` only inspects `InitialJS` delta. If Initial JS didn't change (delta = 0) but Lazy JS grew by 10 MB, `isMinor` evaluates to `true` (0 <= 1024).
   - This causes massive Lazy JS regressions to be classified as `⚪ Size Unchanged` / `⚪ Neutral` and collapsed under `<details><summary>🔎 Minor Source Changes (+0 B)</summary>`.
2. **`opts.Project` Ignored in CLI Text Mode**:
   - `TextOptions.Project` is passed from `cmd/summary.go:99` and `cmd/measure.go:104`, but [internal/report/text.go:30](file:///Users/sonukumar/project/bundleradar/internal/report/text.go#L30) (`TextWithOptions`) and [internal/report/comparison.go:19](file:///Users/sonukumar/project/bundleradar/internal/report/comparison.go#L19) (`ComparisonTextWithOptions`) completely ignore `opts.Project`. It only renders in Markdown headers.
3. **`opts.DriftThreshold` Ignored in CLI Text Mode**:
   - Accepted by `measure` and `compare` flags, but [internal/report/comparison.go:57](file:///Users/sonukumar/project/bundleradar/internal/report/comparison.go#L57) always prints all findings uncollapsed.
4. **Unit Formatting (Metric `KB` vs Binary `KiB`, Spacing)**:
   - `budget.FormatBytes`: `"1KB"` (no space).
   - `report.formatBytes`: `"1 KB"` (with space).
   - `workspace.go:301`: Replaces `" KB"` with `" KiB"` (IEC binary units), while all other reports use metric `"KB"` / `"MB"`. Both divide by 1024.
5. **Redundant Threshold Code**:
   - In `markdown.go:171` and `github_pr.go:126`: `thresholdLabel := formatBytes(threshold); if threshold == 1024 { thresholdLabel = "1 KB" }` is redundant because `formatBytes(1024)` already produces `"1 KB"`.

---

### 3. Data Model & Configuration Schema Inconsistencies

1. **JSON Field Naming Mismatches**:
   - `snapshot.Totals`: `initialJs`, `lazyJs`, `totalJs` (camelCase with `Js`).
   - `comparison.Bytes`: `initialBytes`, `lazyBytes`, `totalBytes` (camelCase with `Bytes`).
   - `comparison.Finding`: `deltaBytes`, `initialDelta`, `lazyDelta` (inconsistent mix of `Bytes` and no `Bytes`).
   - `budget.Violation`: `metric` field uses camelCase (`"initialJs"`, `"totalJsDelta"`) for size breaches, but a human sentence (`"Disallowed package \"foo\""`) for disallowed packages.
2. **Config (`.bundleradar.yml`) vs CLI Flag Naming**:
   - CLI flags: `--max-initial`, `--max-lazy`, `--max-total`, `--max-initial-delta`, `--max-total-delta`.
   - Config YAML: `initial_js_max`, `lazy_js_max`, `total_max` (drops `_js`!), `max_initial_delta`, `max_total_delta`, `max_delta_increase`.
   - Absolute budgets use `_max` as a suffix; delta budgets use `max_` as a prefix. `total_max` omits `_js` while `initial_js_max` and `lazy_js_max` include `_js`.

---

### 4. GitHub Action (`action.yml`) Inconsistencies

1. **Default Format vs Sticky PR Comment**:
   - `action.yml:30`: Default format is `markdown`.
   - When `measure` runs with `format: markdown`, it produces `ComparisonMarkdown`, which lacks the sticky comment marker `<!-- bundleradar-comment -->` and does not include the budget violation breakdown table.
   - `action.yml:416` searches for `<!-- bundleradar-comment -->`; since it's missing, it has to prepend the marker manually.
   - If a budget fails during `bundleradar measure`, the generated markdown report fails to show which budget failed.
2. **Fallback Summary Crash on `format: github-pr`**:
   - In `action.yml:320`, if no baseline is found, it falls back to `bundleradar summary -f "$INPUT_FORMAT"`. If a user set `format: github-pr`, `summary` crashes because `summary` does not support `github-pr`.
3. **Missing Gzip Action Outputs**:
   - Action outputs `initial-bytes`, `lazy-bytes`, `total-bytes`, but does not output `initial-gzip-bytes` or `total-gzip-bytes`, even though `gzip: 'true'` is default.
4. **Stale Action Documentation Examples**:
   - `docs/index.html:844-846` shows `actions/checkout@v4` and `actions/setup-node@v4` (`node-version: 20`), whereas CI uses `v7` and `node-version: 22`.

---

### 5. MCP Server vs CLI Inconsistencies

1. **Missing Parameters in MCP Tools**:
   - `bundle_summary`: Missing `gzip` parameter.
   - `bundle_suggest`: Missing `severity` parameter.
   - `bundle_measure`: Missing `drift_threshold`, `top`, `filter` parameters.
   - `workspace_summary`: Missing `top`, `all`, `gzip`, `target`, `configuration` parameters.
2. **Resource Budget Conflict**:
   - `bundleradar://rules` resource (`internal/mcp/server.go:163`) sets `initial_js_recommended: "500KB"`, whereas `.bundleradar.yml` examples and documentation use `250KB`.

---

### 6. Documentation & Mockup Inconsistencies

1. **Fictional CLI Terminal Mockups**:
   - `docs/index.html` (lines 520, 555, 581, 610, 665, 690, 715) and `README.md` (lines 34-52) show terminal mockups (`BUNDLE SUMMARY`, `CHUNK INSPECTION: ...`, `IMPORT CHAIN TRACE: ...`, `[!] 3 OPTIMIZATION OPPORTUNITIES DETECTED`, `MEASURING AGAINST ACTIVE BASELINE`, `BRANCH DELTA COMPARISON`, `✓ BUDGET CHECK PASSED`) that do not match the real output produced by `internal/report/`.
2. **Completely Undocumented `benchmark` Command**:
   - `bundleradar benchmark` (`cmd/benchmark.go`) is fully implemented and tested, but omitted from `docs/index.html`, `README.md`, `SKILL.md`, and `docs_contract_test.go`.
3. **Stale Progress & Plan Documents**:
   - `docs/progress.md`: Lists "tool version 0.3.0" (current: 0.6.2), "296 tests passed" (current: 464 tests), "Last updated: 2026-09-18".
   - `docs/plan.md`: Lists "296 tests passed", "Last updated: 2026-09-18".
4. **Antigravity Skill Install Path**:
   - `install.sh:185`: Checks `$HOME/.gemini/antigravity-cli/skills`, but the current Antigravity configuration root is `$HOME/.gemini/config/skills` (or `.agents/skills`).

---

### 7. Error Handling & Exit Code Inconsistencies

1. **Fragile Substring Matching in `MapErrorToExitCode`**:
   - [cmd/root.go:68-93](file:///Users/sonukumar/project/bundleradar/cmd/root.go#L68-L93) matches generic error substrings (`"required"`, `"accepts "`, `"must be positive"`).
   - Only `init.go` and `measure.go`/`check.go` use `UsageError` or `PolicyViolationError`.
   - Other commands return raw `fmt.Errorf`. A runtime execution error containing `"required"` (e.g. "required stats.json file missing on disk") is miscategorized as `ExitCodeUsage` (code 2) instead of `ExitCodeExecution` (code 3).

---

### 8. Golden Contract Test Coverage Inconsistencies

1. **Missing Golden Contract Tests**:
   - Golden contracts in `testdata/contracts/v1/` cover only `check`, `compare`, `suggest`, `summary`, and `why`.
   - No golden contract tests exist for `inspect`, `measure`, `workspace summary`, `baseline`, or `benchmark`.

---

## Proposed Remediation Plan

Grouped into 5 focused components:

### Component 1: CLI Flags, UX & Positional Arguments
#### [MODIFY] [cmd/compare.go](file:///Users/sonukumar/project/bundleradar/cmd/compare.go)
- Update format flag help text to include `github-pr`.
- Add `--entry` / `-e` flag to `compare` and plumb to `loadSnapshotOrBuildWithSnapshot`.
- Plumb `project` into `resolveBuildArtifactsInDir(pathOrName, "", "", project)`.
- Add `--gzip` / `-g` flag to display Gzip wire transfer deltas.

#### [MODIFY] [cmd/measure.go](file:///Users/sonukumar/project/bundleradar/cmd/measure.go)
- Allow optional positional arguments `measure [stats.json] [dist]` matching `summary`, `check`, and `suggest`.
- Add `--gzip` / `-g` flag to display Gzip wire transfer deltas.

#### [MODIFY] [cmd/inspect.go](file:///Users/sonukumar/project/bundleradar/cmd/inspect.go) & [cmd/why.go](file:///Users/sonukumar/project/bundleradar/cmd/why.go)
- Add Markdown report support (`--format markdown`) and implement `InspectMarkdown` and `WhyMarkdown`.

#### [MODIFY] [cmd/workspace.go](file:///Users/sonukumar/project/bundleradar/cmd/workspace.go)
- Add alias `--project, -p` for `--projects` so single-project filtering uses standard flag ergonomics.
- Allow `--top 0` to mean default (10) or all packages instead of throwing an error.

---

### Component 2: Report Formatting & Calculation Fixes
#### [MODIFY] [internal/report/markdown.go](file:///Users/sonukumar/project/bundleradar/internal/report/markdown.go) & [internal/report/github_pr.go](file:///Users/sonukumar/project/bundleradar/internal/report/github_pr.go)
- Fix lazy regression collapsing: evaluate both `InitialJS` delta and `TotalJS` delta (or individual finding byte deltas) when determining `isMinor`. Never collapse a multi-kilobyte or megabyte Lazy JS growth into "Minor Source Changes".
- Fix badge generation in `ComparisonMarkdown` and `ComparisonGitHubPR` to reflect Total JS / Lazy JS growth when Initial JS is neutral.
- Include sticky comment marker and budget violations table in `ComparisonMarkdown` when budget limits are evaluated.
- Remove redundant `if threshold == 1024 { thresholdLabel = "1 KB" }`.

#### [MODIFY] [internal/report/text.go](file:///Users/sonukumar/project/bundleradar/internal/report/text.go) & [internal/report/comparison.go](file:///Users/sonukumar/project/bundleradar/internal/report/comparison.go)
- Render `opts.Project` in CLI text headers when specified (e.g. `Angular Bundle Summary (portal)`).
- Honor `opts.DriftThreshold` in `ComparisonTextWithOptions` to group minor drift in text mode when requested.

#### [MODIFY] [internal/report/workspace.go](file:///Users/sonukumar/project/bundleradar/internal/report/workspace.go)
- Align unit formatting to metric `KB` / `MB` (or provide configurable unit display) so `workspace summary` does not clash with all other commands.

---

### Component 3: GitHub Action & MCP Server Alignment
#### [MODIFY] [action.yml](file:///Users/sonukumar/project/bundleradar/action.yml)
- Set default format in action to `github-pr` (or ensure `markdown` format includes the sticky comment marker and budget violations).
- Add `summary` support for `github-pr` format so fallback execution does not fail.
- Expose `initial-gzip-bytes` and `total-gzip-bytes` as action outputs.

#### [MODIFY] [internal/mcp/server.go](file:///Users/sonukumar/project/bundleradar/internal/mcp/server.go)
- Add `gzip` parameter to `bundle_summary`.
- Add `severity` parameter to `bundle_suggest`.
- Add `drift_threshold`, `top`, `filter` parameters to `bundle_measure`.
- Add `top`, `all`, `gzip`, `target`, `configuration` parameters to `workspace_summary`.
- Align `bundleradar://rules` default recommended initial JS budget to 250KB.

---

### Component 4: Error Handling & Exit Codes
#### [MODIFY] [cmd/root.go](file:///Users/sonukumar/project/bundleradar/cmd/root.go) & other `cmd/*.go`
- Consistently wrap CLI argument/flag validation errors in `&UsageError{Err: ...}` across all subcommands.
- Consistently wrap budget/policy violations in `&PolicyViolationError{Err: ...}`.
- Remove fragile substring matching in `MapErrorToExitCode` that falsely maps execution errors containing words like `"required"` to code 2.

---

### Component 5: Documentation, Scripts & Contract Tests
#### [MODIFY] [docs/index.html](file:///Users/sonukumar/project/bundleradar/docs/index.html) & [README.md](file:///Users/sonukumar/project/bundleradar/README.md)
- Update terminal output mockups to match real CLI output from `internal/report/`.
- Document `bundleradar benchmark` in `docs/index.html` and `README.md`.
- Update GitHub Action example snippets to use modern action versions (`v7`).

#### [MODIFY] [docs/progress.md](file:///Users/sonukumar/project/bundleradar/docs/progress.md) & [docs/plan.md](file:///Users/sonukumar/project/bundleradar/docs/plan.md)
- Refresh stale test count (`464 tests passed`), version (`0.6.2`), and dates.

#### [MODIFY] [install.sh](file:///Users/sonukumar/project/bundleradar/install.sh)
- Add `$HOME/.gemini/config/skills/bundleradar` to skill installation search directories.

#### [MODIFY] [cmd/contracts_test.go](file:///Users/sonukumar/project/bundleradar/cmd/contracts_test.go) & [docs_contract_test.go](file:///Users/sonukumar/project/bundleradar/docs_contract_test.go)
- Add golden contract tests for `inspect`, `measure`, `workspace summary`, and `benchmark`.
- Add `benchmark` command to `docs_contract_test.go` expected specs.

---

## Verification Plan

### Automated Tests
```bash
# Run all Go unit and integration tests across packages
rtk go test -v -race -count=1 ./...

# Verify documentation and schema contracts
rtk go test -v . -run TestDocumentationContract -count=1

# Verify skill contract and evaluation suite
rtk go test -v . -run TestAgentSkill -count=1
rtk go test -v . -run TestSkillEval -count=1

# Run golangci-lint
rtk golangci-lint run

# Validate shell installer syntax
rtk proxy sh -n install.sh
```

### Manual & Smoke Verification
- Run `bundleradar measure` and `bundleradar compare` with `--gzip` and verify Gzip deltas.
- Test a large lazy package regression against a baseline and verify that Markdown and GitHub PR reports do NOT collapse it as minor drift.
- Verify `bundleradar summary --format github-pr` outputs clean PR sticky markdown table.
- Verify MCP server tool declarations and verify all new parameters via MCP test suite.

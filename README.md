# bundlecheck

[![CI](https://github.com/sonuKumar03/bundlecheck/actions/workflows/ci.yml/badge.svg)](https://github.com/sonuKumar03/bundlecheck/actions/workflows/ci.yml)

Fast Go CLI and AI agent skill for reporting initial/lazy JavaScript sizes and npm package
contributions from Angular's esbuild-based builds, providing automated optimization suggestions with `suggest`, tracing dependency import paths with `why`, estimating Gzip wire transfer sizes, tracking baselines across code changes, comparing saved summaries, and enforcing size budgets in CI.

See the [progress tracker](docs/progress.md) for completed work, verification,
and upcoming milestones.

For agents consuming bundlecheck, see the [agent CLI guide](docs/agents.md)
for the JSON contract, interpretation rules, and troubleshooting. A
[bundled agent skill](.agents/skills/bundlecheck/SKILL.md) provides the baseline, install,
advisory, tracing, and optimization workflows across projects.

## Build and test

Requires Go 1.27.1 or newer. Dependencies are Cobra and `golang.org/x/net/html`.

```sh
go test ./...
go vet ./...
go build -o bundlecheck .
```

## Install

From the checkout, run:

```sh
./install.sh
bundlecheck --help
```

The script builds and installs with `go install .`. It respects Go's configured
`GOBIN`, otherwise using the first `GOPATH` directory's `bin` directory
(normally `$HOME/go/bin`). Ensure that directory is on your PATH.

### Install the CLI and agent skill together

```sh
./install.sh --with-skill
bundlecheck --help
```

This copies the self-contained skill to `$HOME/.agents/skills/bundlecheck/SKILL.md` (and `$HOME/.gemini/antigravity-cli/skills/bundlecheck` when available). You can also provide a custom directory via `--skill-dir <path>`.

Codex, Antigravity, and Claude Code discover skills in `.agents/skills` and user skill directories.

Example agent prompt:

```text
Use $bundlecheck to baseline the build, run suggestions, trace heavy initial imports, and measure the savings.
```

## Quick Start & Commands

### 1. Summarize an Angular Build (`summary`)

Generate production artifacts in your Angular application:

```sh
ng build --configuration production --stats-json
```

Then run `summary` (auto-detects artifacts in `dist/`):

```sh
# Zero-config auto-discovery
bundlecheck summary

# Include Gzip wire transfer estimations
bundlecheck summary --gzip

# Include immediate optimization suggestions
bundlecheck summary --suggest --gzip

# Machine-readable JSON output
bundlecheck summary --format json

# Export to file directly
bundlecheck summary -o snapshot.json --format json

# Filter packages in text mode
bundlecheck summary --filter angular --top 10
```

Explicit flags `-s, --stats` and `-d, --dist` can still be provided. In multi-project monorepos, pass `-p, --project <name>`.

### 2. Automated Optimization Advisor (`suggest`)

Discover prioritized, concrete opportunities to reduce initial bundle weight:

```sh
# Ranked optimization opportunities
bundlecheck suggest

# Filter suggestions with minimum initial savings threshold
bundlecheck suggest --min-savings 5KB --gzip

# Structured JSON for AI agents
bundlecheck suggest --format json
```

```text
Bundle Optimization Recommendations
Found 2 optimization opportunity(ies) with potential initial JS savings of ~100 KB:

[HIGH] #1: Move 'pdfjs-dist' behind a dynamic import
  Potential Savings:  ~50 KB (~51.2 KB gzip)
  Target / Importer:  src/main.ts
  Rationale:          Package 'pdfjs-dist' is bundled in initial JS. If not critical for first paint, dynamic loading can directly reduce initial bundle size.
  Action:             Replace static 'import ... from "pdfjs-dist"' with dynamic 'const lib = await import("pdfjs-dist")'.
```

### 3. Trace Dependency Import Paths (`why`)

Discover why a package or module is in the bundle and which source files imported it:

```sh
# Trace dependency chains
bundlecheck why lodash

# Trace only initial bundle import paths with JSON output
bundlecheck why @angular/material --initial-only --format json
```

```text
Dependency Trace for "lodash"
Initial JS:  128 B
Lazy JS:     0 B
Total JS:    128 B

Import Chain(s) (1 found):

#1 (chunk: main.js - INITIAL, size: 128 B)
src/main.ts
  └── node_modules/lodash/lodash.js
```

### 4. Capture Baseline & Measure Subsequent Changes (`baseline` & `measure`)

Track iterative code refactoring and optimization:

```sh
# 1. Capture initial baseline to .bundlecheck/baseline.json
bundlecheck baseline

# 2. Make your Angular changes (lazy routes, @defer blocks, dynamic imports)

# 3. Rebuild and measure exact deltas against the baseline
ng build --configuration production --stats-json
bundlecheck measure

# Measure with failure on initial bundle size regression:
bundlecheck measure --max-initial-delta 0B
```

### 5. Compare Saved Snapshots (`compare`)

Compare two arbitrary saved summary snapshots:

```sh
bundlecheck compare --before before.json --after after.json
bundlecheck compare --before before.json --after after.json --format json
```

### 6. Enforce Budgets & CI Checks (`check`)

Enforce bundle budgets and size regression limits in CI pipelines or pre-commit hooks:

```sh
# Static size budget validation
bundlecheck check --max-initial 250KB --max-total 1MB

# Regression check against baseline
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 10KB
```

Returns exit status `0` on compliance and `1` on budget breach.

### 7. Markdown PR & CI Reporting (`--format markdown`)

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

## Architecture

```text
Angular/esbuild stats -> angular adapter -> BundleSnapshot
                                              |
browser dist + index.html -> artifact matching |
                                              v
                              compression & graph classification
                                              |
                                              v
                                    byte/package analysis
                                          /       \
                        AnalysisResult             Optimization Advisor
                         /          \                       |
                       text         JSON                suggestions
```

- `internal/discovery` auto-detects Angular build artifacts (`stats.json` & browser dist).
- `internal/compression` calculates real and estimated Gzip wire transfer sizes.
- `internal/advisor` evaluates bundle rules to generate prioritized refactoring suggestions.
- `internal/baseline` manages `.bundlecheck/baseline.json` storage and resolution.
- `internal/budget` parses byte strings (`200KB`, `1.5MB`) and evaluates budget/regression violations.
- `internal/angular` parses/validates the raw metafile and normalizes paths, contributions, module imports, and dependency edges.
- `internal/snapshot` owns portable modules, outputs, imports, totals, and package models.
- `internal/artifact` parses `index.html` with an HTML parser and matches scripts/stats to emitted browser JavaScript files.
- `internal/graph` traverses static imports, classifies initial/lazy chunks, and traces dependency import paths (`why`).
- `internal/analysis` calculates byte totals and package contributions.
- `internal/comparison` compares byte facts and produces signed before/after deltas.
- `internal/report` formats rich human text, ASCII trees, advice tables, and strict JSON.
- `cmd` orchestrates commands through Cobra.

## Files

```text
.gitignore                 Generated binary/test/coverage artifacts
install.sh                 Build/install CLI and agent skills
install_test.go            Isolated CLI/skill installation tests
.agents/skills/bundlecheck/ Universal installable agent skill (SKILL.md)
go.mod, go.sum             Go module and dependencies
main.go                    Process entry point
cmd/                       Cobra CLI commands (summary, compare, baseline, measure, check, why, suggest)
internal/discovery/        Angular build artifact auto-discovery
internal/compression/      Gzip wire transfer measurement
internal/advisor/          Automated optimization advisor engine
internal/baseline/         Baseline persistence and loading
internal/budget/           Threshold parsing and budget verification
internal/angular/          Raw metadata, parser, normalization
internal/snapshot/         Framework-independent bundle model
internal/artifact/         Browser artifact matching
internal/graph/            Dependency classification, graph traversal, and why path tracing
internal/analysis/         Totals and npm attribution
internal/comparison/       Snapshot comparison and delta computation
internal/report/           Text/JSON reporters and recommendation formatters
testdata/                  Synthetic test fixtures
README.md                  Architecture, commands, contracts, assumptions
docs/agents.md             Agent guide and JSON contracts
docs/plan.md               Architecture and design plan
docs/progress.md           Milestone and progress tracker
```

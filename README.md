# bundlecheck

Fast Go CLI and AI agent skill for reporting initial/lazy JavaScript sizes and npm package
contributions from Angular's esbuild-based builds, tracking baselines across code changes, comparing saved summaries, and enforcing size budgets in CI.

See the [progress tracker](docs/progress.md) for completed work, verification,
and upcoming milestones.

For agents consuming bundlecheck, see the [agent CLI guide](docs/agents.md)
for the JSON contract, interpretation rules, and troubleshooting. A
[bundled agent skill](.agents/skills/bundlecheck/SKILL.md) provides the baseline, install,
and optimization workflows across projects.

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
Use $bundlecheck to baseline the build, optimize lazy routes, and measure the byte savings.
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

# Machine-readable JSON output
bundlecheck summary --format json

# Export to file directly
bundlecheck summary -o snapshot.json --format json

# Filter packages in text mode
bundlecheck summary --filter angular --top 10
```

Explicit flags `-s, --stats` and `-d, --dist` can still be provided. In multi-project monorepos, pass `-p, --project <name>`.

### 2. Capture Baseline & Measure Subsequent Changes (`baseline` & `measure`)

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

### 3. Compare Saved Snapshots (`compare`)

Compare two arbitrary saved summary snapshots:

```sh
bundlecheck compare --before before.json --after after.json
bundlecheck compare --before before.json --after after.json --format json
```

### 4. Enforce Budgets & CI Checks (`check`)

Enforce bundle budgets and size regression limits in CI pipelines or pre-commit hooks:

```sh
# Static size budget validation
bundlecheck check --max-initial 250KB --max-total 1MB

# Regression check against baseline
bundlecheck check --baseline .bundlecheck/baseline.json --max-initial-delta 10KB
```

Returns exit status `0` on compliance and `1` on budget breach.

## Architecture

```text
Angular/esbuild stats -> angular adapter -> BundleSnapshot
                                              |
browser dist + index.html -> artifact matching |
                                              v
                                   graph classification
                                              |
                                              v
                                    byte/package analysis
                                              |
                                              v
                                       AnalysisResult
                                         /       \
                                       text      JSON
```

- `internal/discovery` auto-detects Angular build artifacts (`stats.json` & browser dist).
- `internal/baseline` manages `.bundlecheck/baseline.json` storage and resolution.
- `internal/budget` parses byte strings (`200KB`, `1.5MB`) and evaluates budget/regression violations.
- `internal/angular` parses/validates the raw metafile and normalizes paths, contributions, and dependency edges.
- `internal/snapshot` owns portable modules, outputs, imports, totals, and package models.
- `internal/artifact` parses `index.html` with an HTML parser and matches scripts/stats to emitted browser JavaScript files.
- `internal/graph` traverses static imports from script roots with a visited set.
- `internal/analysis` calculates byte totals and package contributions.
- `internal/comparison` compares byte facts and produces signed before/after deltas.
- `internal/report` formats rich human text and strict JSON.
- `cmd` orchestrates commands through Cobra.

## Fixture example

No Angular installation or build is needed to try the synthetic fixtures:

```sh
bundlecheck summary \
  --stats testdata/lazy-import/stats.json \
  --dist testdata/lazy-import/browser
```

```text
Angular Bundle Summary
Initial JS  160 KB
Lazy JS     300 KB
Total JS    460 KB

Largest initial packages
pdfjs-dist     50 KB    (31.2%)
@angular/core  30 KB    (18.8%)
rxjs           25 KB    (15.6%)
```

## Files

```text
.gitignore                 Generated binary/test/coverage artifacts
install.sh                 Build/install CLI and agent skills
install_test.go            Isolated CLI/skill installation tests
.agents/skills/bundlecheck/ Universal installable agent skill (SKILL.md)
go.mod, go.sum             Go module and dependencies
main.go                    Process entry point
cmd/                       Cobra CLI commands (summary, compare, baseline, measure, check)
internal/discovery/        Angular build artifact auto-discovery
internal/baseline/         Baseline persistence and loading
internal/budget/           Threshold parsing and budget verification
internal/angular/          Raw metadata, parser, normalization
internal/snapshot/         Framework-independent bundle model
internal/artifact/         Browser artifact matching
internal/graph/            Dependency classification and graph traversal
internal/analysis/         Totals and npm attribution
internal/comparison/       Snapshot comparison and delta computation
internal/report/           Text/JSON reporters
testdata/                  Synthetic test fixtures
README.md                  Architecture, commands, contracts, assumptions
docs/agents.md             Agent guide and JSON contracts
docs/plan.md               Architecture and design plan
docs/progress.md           Milestone and progress tracker
```

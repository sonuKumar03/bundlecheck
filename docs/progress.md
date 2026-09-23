# bundleradar progress

Last updated: 2026-09-18

Update this document when a milestone changes. Mark work complete only after
implementation and verification; record remaining limitations separately.

## Current status

Optimization Advisor (`bundleradar suggest`), Markdown PR / CI Reporter (`--format markdown`), Gzip wire transfer estimation (`--gzip`), baseline measurement, zero-config artifact auto-discovery, dependency import path tracing (`why`), CI budget validation, and universal AI agent skill workflows are fully implemented and verified.

## Completed

- [x] Parse and validate Angular/esbuild metafiles; normalize into portable snapshot types.
- [x] Match local index script roots to emitted browser artifacts.
- [x] Classify initial JS through static imports, handling shared chunks and cycles.
- [x] Treat remaining browser JS as lazy; exclude CSS, maps, assets, and server outputs.
- [x] Calculate raw output bytes and emitted npm package contributions.
- [x] Handle scoped, nested, and Windows-style package paths and normalization collisions.
- [x] Expose `summary --stats --dist --format text|json|markdown` with auto-discovery, short flags (`-s`, `-d`, `-p`, `-f`, `-o`), `--gzip`, and `--suggest`.
- [x] Provide deterministic JSON with schema version `1`, tool version `0.3.0`, and all contributing packages.
- [x] Provide text totals, percentage shares, and configurable `--top`, `--filter`, `--all` flags.
- [x] Add zero-config artifact auto-discovery (`internal/discovery`) for single and multi-project workspaces.
- [x] Implement baseline capture (`bundleradar baseline`) and `.bundleradar/baseline.json` management.
- [x] Implement iterative change measurement (`bundleradar measure`) against baselines.
- [x] Implement bundle size budgets and regression checks (`bundleradar check`).
- [x] Implement dependency and import path tracer (`bundleradar why`) with ASCII trees and JSON output.
- [x] Implement automated optimization advisor (`bundleradar suggest`) for dynamic imports, eager routes, and duplicate packages.
- [x] Implement real and estimated Gzip wire transfer sizing (`internal/compression`).
- [x] Implement GitHub Flavored Markdown PR & CI reporting (`--format markdown`, `-f md`) across summary, compare, measure, suggest, and check.
- [x] Add direct file output support (`-o, --output`) across CLI commands.
- [x] Add universal AI agent skill (`.agents/skills/bundleradar/SKILL.md`) with complete optimization playbooks.
- [x] Implement Model Context Protocol (MCP) server (`bundleradar mcp`) exposing 6 core analysis tools and `bundleradar://rules` resource.
- [x] Update `install.sh` to support multi-environment skill installation (`--with-skill`, `--skill-dir`).
- [x] Compare saved summary snapshots with signed JS/package deltas and deterministic text/JSON/markdown.
- [x] Warn when Nx workspace bundle inputs are newer than an app's `stats.json`, without changing report completeness or exit status.

## Verification recorded

The latest implementation checks on 2026-09-21 passed:

| Check | Result |
| --- | --- |
| `rtk go test ./...` | 296 tests passed across 19 packages |
| `rtk go vet ./...` | Passed |
| `rtk go build -o bundleradar .` | Passed |
| `rtk proxy sh -n install.sh` | Passed |
| Skill frontmatter validation | Passed |
| Isolated CLI/skill install and reinstall | Passed, including custom `--skill-dir` |
| Markdown PR & CI Reporter | Verified tables, collapsible sections, badges, and recommendations across all commands |
| Optimization Advisor (`suggest`) | Verified dynamic import rules, eager routes, and duplicate package detection |
| Gzip wire transfer estimation | Verified real file compression and contribution estimation |
| Dependency tracer (`why`) | Verified with single-hop, multi-hop, and chunk-level fallback traces |
| Baseline and Measure workflow | Verified with synthetic fixtures and delta validations |
| CI Budget checks | Verified for both pass and fail thresholds |
| Auto-discovery | Verified for single-app and multi-project structures |

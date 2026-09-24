# bundleradar progress

Last updated: 2026-09-24

Update this document when a milestone changes. Mark work complete only after
implementation and verification; record remaining limitations separately.

## Current status

BundleRadar v2 is officially released (`v2.0.0`) with a clean-slate hexagonal architecture centered on 5 orthogonal verbs: `scan`, `diff`, `gate`, `workspace`, and `mcp`. All CI pipelines, cross-platform build matrices, Go microbenchmarks, and GitHub Actions integrations are fully verified and passing.

## Completed

- [x] Clean-slate v2 architecture designed and implemented across `internal/core`, `internal/adapters`, and `cmd/`.
- [x] Streamlined CLI verbs: `scan`, `diff`, `gate`, `workspace`, and `mcp`.
- [x] Universal Bundle AST in `internal/core/bundle.go` with zero external dependencies.
- [x] Pluggable parsers registry in `internal/adapters/parsers/` supporting Angular, Esbuild, Vite, and Webpack.
- [x] Universal Diff & Source Attribution Engine in `internal/core/diff/` with micro-drift bucketing.
- [x] Policy & Budget Evaluation Gate in `internal/core/policy/`.
- [x] Multi-format reporters in `internal/adapters/reporters/` (Terminal, Markdown, GitHub PR, JSON).
- [x] Monorepo & multi-app workspace discovery in `internal/adapters/workspaces/`.
- [x] Public Go SDK in `pkg/bundleradar/`.
- [x] Modernized MCP server (`bundleradar mcp`) with updated tools (`scan_bundle`, `diff_bundles`, `gate_bundle`, `workspace_scan`).
- [x] Full GitHub Action (`action.yml`) modernization and automated test suite.
- [x] Documentation & schema contract test suite passing.
- [x] Cross-platform build and release automation for Linux, macOS, and Windows.
- [x] Official `v2.0.0` release published on GitHub Releases with floating major tag `v2`.

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

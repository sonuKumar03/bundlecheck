# Baselines, gates, and CI

## Local baseline loop

```sh
bundlecheck baseline save --name pre-change --format json
# edit, rebuild production stats, and run project tests
bundlecheck measure --baseline pre-change --format json
bundlecheck check --baseline pre-change --max-initial-delta 0B --format json
```

Use `baseline save --ref <git-ref> --name <name>` only when Git is available and the build can run in an isolated worktree. `baseline list`, `use`, `show`, `rebuild`, and `delete` manage named snapshots. Use `compare` for two arbitrary saved snapshots.

Measure and check the same project and entry scope captured by the baseline. A negative signed delta is a reduction; a positive delta is growth.

## Reports

`summary`, `compare`, `measure`, and `check` support Markdown output for CI summaries or PR comments:

```sh
bundlecheck measure --baseline pre-change --format markdown -o bundle-report.md
bundlecheck check --max-initial 250KB --format markdown
```

Do not claim runtime-performance improvement from these reports. Include the measured byte delta and build/test evidence.

## GitHub Action

The repository action can analyze current stats, build a base-ref baseline, enforce absolute or delta budgets, and maintain a PR comment. Build current production stats before invoking it. Delta gates must fail closed when base analysis is unavailable; report-only runs may fall back to current-build measurements with a warning.

Prefer an immutable released action tag. Configure only inputs declared by `action.yml`, especially `stats`, `dist`, `project`, `entry`, `base-ref`, `build-cmd`, size/delta limits, and `post-comment`.

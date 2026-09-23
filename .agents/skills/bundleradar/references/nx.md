# Nx and multi-app analysis

Build selected Angular applications with production stats before analysis. BundleRadar supports Angular application and browser-esbuild builders; it does not build apps itself.

```sh
nx run-many -t build --projects=shop,admin --configuration=production --stats-json
bundleradar workspace summary --projects shop,admin --format json
```

Use the report's exact stats/dist paths for single-app summary, why, baseline, and measure commands.

## Interpret the matrix

- `apps[]` contains per-deployment initial, lazy, and total bytes.
- `packages[]` and `libraries[]` compare attributed contributions across apps.
- Shared-library bytes exclude npm dependencies imported by that library.
- Cross-app sums are separate deployments, not deduplication savings.
- Keep generated, compiled-library, and unattributed inputs unassigned when ownership is unknown.

## Freshness and partial results

`freshness.status: stale-suspected` means an input is newer than `stats.json`; rebuild before making decisions. `unknown` is not proof that artifacts are fresh. Bundle configuration and Git provenance are not verified.

When `complete` is false, preserve successful app results and report failed apps separately. Failed apps have null matrix values; a zero means an analyzed app had no contribution. Unsupported apps are skipped by default, while explicitly selecting one fails.

Static project parsing is preferred. Allow an Nx CLI fallback only for a trusted workspace because it executes workspace-installed code.

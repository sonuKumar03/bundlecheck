# JSON contracts

BundleRadar v2 formats output as structured JSON for automation and AI agents.

## Scan JSON (`bundleradar scan -f json`)

- `metadata`: `bundler`, `timestamp`, `commitSha`.
- `entrypoints`: map of entrypoint name to `initialBytes`, `initialGzipBytes`, `asyncBytes`, `chunkIds`.
- `chunks[]`: `id`, `name`, `path`, `sizeBytes`, `gzipBytes`, `type` (`initial` / `async`), `entry`, `moduleIds`.
- `modules[]`: `id`, `package`, `version`, `sizeBytes`, `gzipBytes`, `isAppCode`, `chunkIds`.
- `assets[]`: compiled auxiliary files (CSS, WASM, images).

## Diff JSON (`bundleradar diff -f json`)

- `initialDelta`: signed delta in initial bytes (negative is reduction, positive is growth).
- `asyncDelta`: signed delta in async/lazy bytes.
- `totalDelta`: signed delta in total bundle bytes.
- `entrypointDeltas`: per-entrypoint initial and async deltas.
- `packageDeltas[]`: npm packages added, removed, or changed with byte differences.

## Gate JSON (`bundleradar gate -f json`)

- `passed`: boolean indicating whether all policy limits passed.
- `violations[]`: list of specific policy breaches (`rule`, `metric`, `actual`, `limit`, `message`).

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | analysis succeeded and policy passed |
| 1 | budget, regression, or package policy violation |
| 2 | invalid usage or configuration flags |
| 3 | execution failure, missing files, or build error |

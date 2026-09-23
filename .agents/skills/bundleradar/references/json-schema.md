# JSON contracts

Consumers must accept additive fields. `schemaVersion` changes for breaking contracts; `toolVersion` identifies the producer. Collections use deterministic ordering.

## Summary

- `summary.initialJs`, `lazyJs`, `totalJs`: exact emitted JavaScript bytes.
- `summary.initialGzipJs`, `lazyGzipJs`, `totalGzipJs`: gzip estimates when requested.
- `packages[]`: attributed package `initialBytes`, `lazyBytes`, and `totalBytes`.

## Suggestions

- `suggestions[].rule`, `severity`, `target`, `file`, and `action` describe the candidate.
- `suggestions[].savingsBytes` and `savingsGzipBytes` are estimates, not measured results.
- `totalPotentialSavings` aggregates estimates and can overlap; do not present it as achieved savings.

## Trace

- `target`, `packageName`, `found`, `initialBytes`, `lazyBytes`, `totalBytes` identify the subject.
- `chains[].path` is the ordered import path; `output`, `initial`, and `bytesInChunk` describe its emitted location.

## Comparison and measure

- `summary.before`, `after`, and `delta` contain `initialJs`, `lazyJs`, and `totalJs`.
- Deltas are signed integers: negative means fewer bytes, positive means growth.
- `packages[].status` is `added`, `removed`, `changed`, or `unchanged`; each package has before/after/delta byte fields.
- `findings[]` may explain deterministic regressions.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | analysis succeeded and any policy passed |
| 1 | budget, regression, or package policy violation |
| 2 | invalid usage or configuration |
| 3 | execution failure, missing artifacts, or incomplete workspace analysis |

On code 0 parse stdout. On code 1 preserve the JSON report when present and report the violated policy. On codes 2 or 3 report stderr and correct the input or build state before retrying.

# CLI fallback

Use `--format json` for agent decisions. BundleRadar does not build the application; create fresh production `stats.json` first when discovery reports missing or stale artifacts.

## Install and discover

```sh
command -v bundleradar
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh
ng build --configuration production --stats-json
```

The release installer downloads a prebuilt binary; Go is not required. Auto-discovery finds Angular esbuild stats and the matching browser output. In ambiguous workspaces pass `--project`, or explicit `--stats` and `--dist`. Use `--entry` for a source entrypoint or emitted chunk glob when analysis must be scoped.

## Commands

```sh
bundleradar summary --format json
bundleradar summary --gzip --suggest --format json
bundleradar inspect <chunk> --format json
bundleradar suggest --format json
bundleradar why <package-or-source> --initial-only --format json
bundleradar baseline save --name pre-change --format json
bundleradar measure --baseline pre-change --format json
bundleradar compare before.json after.json --format json
bundleradar check --max-initial 250KB --max-total 1MB --format json
bundleradar workspace summary --projects shop,admin --format json
```

`summary`, `why`, `suggest`, `measure`, and `check` accept artifact/project selection flags. `inspect` accepts a unique output filename or full output path. `compare` accepts two snapshots or a snapshot and current stats. Run `bundleradar <command> --help` instead of guessing less-common flags.

## Failures

Capture stdout, stderr, and exit status separately. Missing artifacts require a production build. Ambiguous artifacts require `--project` or explicit paths. A missing target should be checked against summary attribution. A missing baseline requires `baseline save` or selecting an existing named baseline.

Workspace summary is special: an incomplete multi-app analysis can still return useful JSON. Inspect `complete` and per-app diagnostics instead of treating missing apps as zero.

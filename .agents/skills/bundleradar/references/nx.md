# Nx and multi-app analysis

BundleRadar discovers and scans application targets across Nx, pnpm, npm, and yarn workspaces.

```sh
# Discover all application targets in the workspace
bundleradar workspace list --root .

# Scan all targets or pass explicit target mappings
bundleradar workspace scan --root .
bundleradar workspace scan --app "shop=apps/shop/dist/stats.json" --app "admin=apps/admin/dist/stats.json" -f json
```

## Structure

`workspace scan -f json` produces:
- `targets[]`: each target's `name`, `statsPath`, `initialBytes`, `asyncBytes`, `totalBytes`, `chunkCount`.
- `totalInitialBytes`: sum of initial bytes across all targets.
- `totalAsyncBytes`: sum of async bytes across all targets.
- `totalBytes`: sum of total bytes across all targets.

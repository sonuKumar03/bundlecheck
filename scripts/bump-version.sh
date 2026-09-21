#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# bundlecheck version bump & sync utility
# Single source of truth: root VERSION file
# ─────────────────────────────────────────────────────────────────────────────

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ $# -lt 1 ]; then
  CURRENT_VERSION="$(cat VERSION | tr -d '[:space:]')"
  echo "Usage: ./scripts/bump-version.sh <new-version> [--release]"
  echo "Current version: $CURRENT_VERSION"
  exit 1
fi

NEW_VER="${1#v}" # strip leading 'v' if provided
DO_RELEASE=false

if [ "${2:-}" = "--release" ] || [ "${2:-}" = "--tag" ]; then
  DO_RELEASE=true
fi

# Validate semver format (e.g. 0.3.0 or 1.0.0-rc.1)
if ! [[ "$NEW_VER" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "Error: '$NEW_VER' is not a valid semantic version (e.g. 0.4.0 or 1.0.0)" >&2
  exit 1
fi

OLD_VER="$(cat VERSION | tr -d '[:space:]')"

echo "⚡ Bumping bundlecheck version: $OLD_VER -> $NEW_VER"

# 1. Update VERSION file (Single Source of Truth)
echo "$NEW_VER" > VERSION
echo "  ✓ Updated VERSION"

# 2. Update Go internal/analysis/summary.go
python3 -c "
with open('internal/analysis/summary.go', 'r') as f:
    content = f.read()
import re
new_content = re.sub(r'var ToolVersion = \"[^\"]+\"', f'var ToolVersion = \"$NEW_VER\"', content)
with open('internal/analysis/summary.go', 'w') as f:
    f.write(new_content)
"
echo "  ✓ Updated internal/analysis/summary.go"

# 3. Update npm/bundlecheck/package.json
python3 -c "
import json
with open('npm/bundlecheck/package.json', 'r') as f:
    pkg = json.load(f)
pkg['version'] = '$NEW_VER'
with open('npm/bundlecheck/package.json', 'w') as f:
    json.dump(pkg, f, indent=2)
    f.write('\n')
"
echo "  ✓ Updated npm/bundlecheck/package.json"

# 4. Update docs/index.html
python3 -c "
with open('docs/index.html', 'r') as f:
    html = f.read()
import re
html = re.sub(r'\"softwareVersion\": \"[^\"]+\"', f'\"softwareVersion\": \"$NEW_VER\"', html)
html = re.sub(r'v$OLD_VER', f'v$NEW_VER', html)
html = re.sub(r'bundlecheck v$OLD_VER', f'bundlecheck v$NEW_VER', html)
with open('docs/index.html', 'w') as f:
    f.write(html)
"
echo "  ✓ Updated docs/index.html"

# 5. Update docs/agents.md
python3 -c "
with open('docs/agents.md', 'r') as f:
    doc = f.read()
import re
doc = re.sub(
    r'(\| \x60toolVersion\x60 \| Tool release version; currently )\x60\"[^\"]+\"\x60(\. \|)',
    lambda match: match.group(1) + '\x60\"$NEW_VER\"\x60' + match.group(2),
    doc,
)
with open('docs/agents.md', 'w') as f:
    f.write(doc)
"
echo "  ✓ Updated docs/agents.md"

# 6. Update README.md and sync npm/bundlecheck/README.md
python3 -c "
with open('README.md', 'r') as f:
    readme = f.read()
import re
readme = re.sub(r'uses: sonuKumar03/bundlecheck@v[0-9.]+', f'uses: sonuKumar03/bundlecheck@v$NEW_VER', readme)
with open('README.md', 'w') as f:
    f.write(readme)
"
cp README.md npm/bundlecheck/README.md
echo "  ✓ Updated README.md and npm/bundlecheck/README.md"

# 7. Update golden contracts testdata
python3 -c "
import glob, json
for path in glob.glob('testdata/contracts/v1/*.json'):
    with open(path, 'r') as f:
        data = json.load(f)
    if 'toolVersion' in data:
        data['toolVersion'] = '$NEW_VER'
        with open(path, 'w') as f:
            json.dump(data, f, indent=2)
            f.write('\n')
"
echo "  ✓ Updated testdata/contracts/v1/*.json"

# 8. Run test verification
echo "Running test suite verification..."
go test ./... > /dev/null
echo "  ✓ All Go tests pass with version $NEW_VER"

echo ""
echo "🎉 Version successfully bumped to $NEW_VER across all targets!"
echo ""

if [ "$DO_RELEASE" = true ]; then
  echo "🚀 Committing, tagging, and pushing release v$NEW_VER..."
  git commit -am "chore(release): bump version to $NEW_VER"
  git tag -a "v$NEW_VER" -m "Release v$NEW_VER"
  git push origin master --tags
  echo "  ✓ Release v$NEW_VER pushed to origin!"
else
  echo "To commit and tag this release manually, run:"
  echo "  git commit -am \"chore(release): bump version to $NEW_VER\""
  echo "  git tag -a \"v$NEW_VER\" -m \"Release v$NEW_VER\""
  echo "  git push origin master --tags"
fi

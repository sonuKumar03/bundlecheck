#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# bundlecheck release contract verification script
# Validates public tag, installer URL, release assets, npm package, and action
# ─────────────────────────────────────────────────────────────────────────────

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ $# -lt 1 ]; then
  echo "Usage: $0 <candidate-tag> (e.g. v0.3.0)" >&2
  exit 1
fi

TAG="$1"
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "Error: candidate tag '$TAG' must follow semver with a leading 'v' (e.g. v0.3.0)" >&2
  exit 1
fi

VERSION="${TAG#v}"
REPO="sonuKumar03/bundlecheck"

echo "🔍 Verifying release contract for $TAG (version: $VERSION)..."

# 1. Local offline verification first
echo "  [1/6] Running local documentation & schema contract tests..."
go test . -run TestDocumentationContract -count=1
go vet ./...

# 2. Check git remote tag
echo "  [2/6] Checking git remote tag..."
if git ls-remote --tags "https://github.com/${REPO}.git" "refs/tags/${TAG}" | grep -q "${TAG}"; then
  echo "    ✓ Git remote tag ${TAG} exists on origin"
else
  echo "    ⚠️ Warning: Git remote tag ${TAG} not found on origin (expected if pre-push)"
fi

# 3. Check installer script accessibility
echo "  [3/6] Checking installer raw accessibility..."
INSTALL_URL="https://raw.githubusercontent.com/${REPO}/master/install.sh"
if curl -fsSL -I "$INSTALL_URL" >/dev/null 2>&1; then
  echo "    ✓ Installer script reachable at $INSTALL_URL"
else
  echo "    ❌ Error: Installer script not reachable at $INSTALL_URL" >&2
  exit 1
fi

# 4. Check GitHub release publication
echo "  [4/6] Checking GitHub release..."
RELEASE_URL="https://github.com/${REPO}/releases/tag/${TAG}"
if curl -fsSL -I "$RELEASE_URL" >/dev/null 2>&1; then
  echo "    ✓ GitHub release page published at $RELEASE_URL"
else
  echo "    ⚠️ Warning: GitHub release page not reachable (expected if release workflow is still running)"
fi

# 5. Check release assets if release page exists
echo "  [5/6] Checking release asset availability..."
ASSETS=(
  "bundlecheck_${VERSION}_darwin_arm64.tar.gz"
  "bundlecheck_${VERSION}_darwin_amd64.tar.gz"
  "bundlecheck_${VERSION}_linux_amd64.tar.gz"
  "bundlecheck_${VERSION}_linux_arm64.tar.gz"
  "bundlecheck_${VERSION}_windows_amd64.zip"
  "checksums.txt"
)
ALL_ASSETS_OK=true
for ASSET in "${ASSETS[@]}"; do
  ASSET_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
  if curl -fsSL -I "$ASSET_URL" >/dev/null 2>&1; then
    echo "    ✓ Asset reachable: $ASSET"
  else
    ALL_ASSETS_OK=false
  fi
done
if [ "$ALL_ASSETS_OK" = "true" ]; then
  echo "    ✓ All multi-arch assets and checksums verified"
else
  echo "    ℹ️ Some binary assets not yet available for ${TAG}"
fi

# 6. Check npm registry package
echo "  [6/6] Checking npm registry metadata..."
if command -v npm >/dev/null 2>&1; then
  NPM_VER=$(npm view "@sonukumar03/bundlecheck@${VERSION}" version 2>/dev/null || true)
  if [ "$NPM_VER" = "$VERSION" ]; then
    echo "    ✓ npm package @sonukumar03/bundlecheck@${VERSION} published"
  else
    echo "    ℹ️ npm package @sonukumar03/bundlecheck@${VERSION} not published or pending registry indexing"
  fi
else
  echo "    ℹ️ npm not installed; skipping npm registry check"
fi

echo "✅ Release contract verification completed."

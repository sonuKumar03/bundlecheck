#!/bin/sh
set -eu

REPO="sonuKumar03/bundleradar"

usage() {
  printf 'Usage: %s [--with-skill] [--skill-dir <path>]\n' "$0"
}

with_skill=false
custom_skill_dir=""

while [ $# -gt 0 ]; do
  case "$1" in
    --with-skill)
      with_skill=true
      shift
      ;;
    --skill-dir)
      if [ $# -lt 2 ]; then
        usage >&2
        exit 2
      fi
      custom_skill_dir="$2"
      with_skill=true
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

script_dir=""
if [ -n "${0:-}" ] && [ -f "$0" ]; then
  script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
fi

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *) echo "unknown" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "unknown" ;;
  esac
}

download_file() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    printf 'Error: curl or wget is required\n' >&2
    return 1
  fi
}

install_binary() {
  if [ -n "$script_dir" ] && [ -f "$script_dir/go.mod" ]; then
    (cd "$script_dir" && go install .)
    return
  fi

  OS="$(detect_os)"
  ARCH="$(detect_arch)"

  if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
    if command -v go >/dev/null 2>&1; then
      printf 'Unrecognized OS/Arch, building from source via go install...\n'
      go install github.com/sonuKumar03/bundleradar@latest
      return 0
    else
      printf 'Error: Unsupported OS (%s) or Architecture (%s)\n' "$OS" "$ARCH" >&2
      exit 1
    fi
  fi

  # Determine destination directory
  INSTALL_DIR="${GOBIN:-/usr/local/bin}"
  if [ -z "${GOBIN:-}" ] && [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
  fi
  mkdir -p "$INSTALL_DIR"

  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

  EXT="tar.gz"
  BINARY_NAME="bundleradar"
  if [ "$OS" = "windows" ]; then
    EXT="zip"
    BINARY_NAME="bundleradar.exe"
  fi
  printf 'Downloading precompiled bundleradar binary for %s/%s...\n' "$OS" "$ARCH"

  TAG=""
  if download_file "https://api.github.com/repos/${REPO}/releases/latest" "$TMP_DIR/release.json"; then
    # Read GitHub's tag_name field only; use a JSON parser if release metadata changes.
    TAG="$(sed -n 's/.*"tag_name":[[:space:]]*"\(v[^" ]*\)".*/\1/p' "$TMP_DIR/release.json" | head -n 1)"
  fi
  case "$TAG" in
    v*[!a-zA-Z0-9._-]*|v|"") TAG="" ;;
  esac
  ARCHIVE="$TMP_DIR/bundleradar.$EXT"
  if [ -n "$TAG" ] && download_file "https://github.com/${REPO}/releases/download/${TAG}/bundleradar_${TAG#v}_${OS}_${ARCH}.${EXT}" "$ARCHIVE"; then
    if [ "$EXT" = "zip" ]; then
      if command -v unzip >/dev/null 2>&1; then
        unzip -oq "$ARCHIVE" -d "$TMP_DIR"
      else
        if command -v cygpath >/dev/null 2>&1; then
          ARCHIVE="$(cygpath -w "$ARCHIVE")"
          ZIP_DEST="$(cygpath -w "$TMP_DIR")"
        else
          ZIP_DEST="$TMP_DIR"
        fi
        BUNDLECHECK_INSTALL_ARCHIVE="$ARCHIVE" BUNDLECHECK_INSTALL_DEST="$ZIP_DEST" \
          powershell -NoProfile -Command 'Expand-Archive -LiteralPath $env:BUNDLECHECK_INSTALL_ARCHIVE -DestinationPath $env:BUNDLECHECK_INSTALL_DEST -Force'
      fi
    else
      tar -xzf "$ARCHIVE" -C "$TMP_DIR"
    fi
    cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    printf 'Installed bundleradar to %s/%s\n' "$INSTALL_DIR" "$BINARY_NAME"
  elif command -v go >/dev/null 2>&1; then
    printf 'Precompiled release download failed, installing via go install...\n'
    go install github.com/sonuKumar03/bundleradar@latest
  else
    printf 'Error: Failed to download precompiled binary from GitHub Releases\n' >&2
    exit 1
  fi
}

skill_files='SKILL.md
references/cli.md
references/json-schema.md
references/nx.md
references/ci.md'

install_skill_tree() {
  skill_dest="$1"
  printf '%s\n' "$skill_files" | while IFS= read -r rel; do
    target="$skill_dest/$rel"
    mkdir -p "$(dirname "$target")"
    if [ -n "$script_dir" ] && [ -f "$script_dir/.agents/skills/bundleradar/$rel" ]; then
      cp "$script_dir/.agents/skills/bundleradar/$rel" "$target"
    else
      download_file "https://raw.githubusercontent.com/${REPO}/master/.agents/skills/bundleradar/$rel" "$target"
    fi
  done
}

install_binary

if [ "$with_skill" = true ]; then
  if [ -n "$custom_skill_dir" ]; then
    install_skill_tree "$custom_skill_dir"
    printf 'Installed bundleradar skill at %s\n' "$custom_skill_dir"
  else
    for skill_dir in \
      "$HOME/.agents/skills/bundleradar" \
      "$HOME/.claude/skills/bundleradar" \
      "$HOME/.codex/skills/bundleradar"
    do
      install_skill_tree "$skill_dir"
      printf 'Installed bundleradar skill at %s\n' "$skill_dir"
    done

    if [ -d "$HOME/.gemini/antigravity-cli/skills" ]; then
      agy_dir="$HOME/.gemini/antigravity-cli/skills/bundleradar"
      install_skill_tree "$agy_dir"
      printf 'Installed bundleradar skill at %s\n' "$agy_dir"
    fi
  fi
fi

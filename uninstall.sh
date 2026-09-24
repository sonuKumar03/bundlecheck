#!/bin/sh
set -eu

usage() {
  cat <<'EOF'
Usage: uninstall.sh [options]

Cleanly removes bundleradar (and legacy bundlecheck) binaries and installed agent skills.

Options:
  --dry-run       Print what would be removed without deleting
  --bin-dir <dir> Check an explicit directory for binaries
  --keep-skills   Keep installed AI agent skills (only remove binaries)
  --quiet, -q     Suppress non-error messages
  --help, -h      Show this help message
EOF
}

dry_run=false
keep_skills=false
quiet=false
custom_bin_dir=""

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run)
      dry_run=true
      shift
      ;;
    --bin-dir)
      if [ $# -lt 2 ]; then
        usage >&2
        exit 2
      fi
      custom_bin_dir="$2"
      shift 2
      ;;
    --keep-skills)
      keep_skills=true
      shift
      ;;
    --quiet|-q)
      quiet=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      printf 'Error: Unknown option %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

log() {
  if [ "$quiet" = false ]; then
    printf '%s\n' "$1"
  fi
}

remove_target() {
  target="$1"
  desc="$2"

  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ "$dry_run" = true ]; then
      log "Would remove $desc: $target"
    else
      if [ -w "$(dirname "$target")" ] || [ -w "$target" ]; then
        rm -rf "$target"
        log "✓ Removed $desc: $target"
      else
        log "⚠ Permission denied (requires sudo to remove $target)"
      fi
    fi
  fi
}

# 1. Search and remove binary targets
bin_dirs="/usr/local/bin $HOME/.local/bin"

if [ -n "${GOBIN:-}" ]; then
  bin_dirs="$bin_dirs $GOBIN"
fi

if command -v go >/dev/null 2>&1; then
  gopath="$(go env GOPATH 2>/dev/null || true)"
  if [ -n "$gopath" ]; then
    bin_dirs="$bin_dirs $gopath/bin"
  fi
elif [ -d "$HOME/go/bin" ]; then
  bin_dirs="$bin_dirs $HOME/go/bin"
fi

if [ -n "$custom_bin_dir" ]; then
  bin_dirs="$bin_dirs $custom_bin_dir"
fi

log "Scanning for bundleradar and legacy bundlecheck installations..."

binary_names="bundleradar bundleradar.exe bundlecheck bundlecheck.exe"

for dir in $bin_dirs; do
  for name in $binary_names; do
    remove_target "$dir/$name" "binary"
  done
done

# Check if any binary is still accessible in PATH
for name in bundleradar bundlecheck; do
  if bin_path="$(command -v "$name" 2>/dev/null || true)"; then
    if [ -n "$bin_path" ]; then
      remove_target "$bin_path" "binary in PATH"
    fi
  fi
done

# 2. Search and remove agent skills (unless --keep-skills requested)
if [ "$keep_skills" = false ]; then
  skill_dirs="
    $HOME/.agents/skills/bundleradar
    $HOME/.claude/skills/bundleradar
    $HOME/.codex/skills/bundleradar
    $HOME/.gemini/antigravity-cli/skills/bundleradar
    $HOME/.agents/skills/bundlecheck
    $HOME/.claude/skills/bundlecheck
    $HOME/.codex/skills/bundlecheck
    $HOME/.gemini/antigravity-cli/skills/bundlecheck
  "

  for dir in $skill_dirs; do
    remove_target "$dir" "agent skill"
  done
fi

log "Uninstall complete."

#!/bin/sh
set -eu

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

if [ -n "$script_dir" ] && [ -f "$script_dir/go.mod" ]; then
  (cd "$script_dir" && go install .)
  skill_src="$script_dir/.agents/skills/bundlecheck/SKILL.md"
else
  go install github.com/sonuKumar03/bundlecheck@latest
  skill_src=""
fi

install_skill_file() {
  dest="$1"
  mkdir -p "$(dirname "$dest")"
  if [ -n "$skill_src" ] && [ -f "$skill_src" ]; then
    cp "$skill_src" "$dest"
  else
    if command -v curl >/dev/null 2>&1; then
      curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/.agents/skills/bundlecheck/SKILL.md -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
      wget -qO "$dest" https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/.agents/skills/bundlecheck/SKILL.md
    else
      printf 'Error: curl or wget is required to download skill file\n' >&2
      return 1
    fi
  fi
}

if [ "$with_skill" = true ]; then
  if [ -n "$custom_skill_dir" ]; then
    install_skill_file "$custom_skill_dir/SKILL.md"
    printf 'Installed bundlecheck skill at %s\n' "$custom_skill_dir"
  else
    skill_dir="$HOME/.agents/skills/bundlecheck"
    install_skill_file "$skill_dir/SKILL.md"
    printf 'Installed bundlecheck skill at %s\n' "$skill_dir"

    if [ -d "$HOME/.gemini/antigravity-cli/skills" ]; then
      agy_dir="$HOME/.gemini/antigravity-cli/skills/bundlecheck"
      install_skill_file "$agy_dir/SKILL.md"
      printf 'Installed bundlecheck skill at %s\n' "$agy_dir"
    fi
  fi
fi

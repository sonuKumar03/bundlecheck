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

CDPATH= cd -- "$(dirname -- "$0")"
go install .

if [ "$with_skill" = true ]; then
  if [ -n "$custom_skill_dir" ]; then
    mkdir -p "$custom_skill_dir"
    cp .agents/skills/bundlecheck/SKILL.md "$custom_skill_dir/SKILL.md"
    printf 'Installed bundlecheck skill at %s\n' "$custom_skill_dir"
  else
    skill_dir="$HOME/.agents/skills/bundlecheck"
    mkdir -p "$skill_dir"
    cp .agents/skills/bundlecheck/SKILL.md "$skill_dir/SKILL.md"
    printf 'Installed bundlecheck skill at %s\n' "$skill_dir"

    if [ -d "$HOME/.gemini/antigravity-cli/skills" ]; then
      agy_dir="$HOME/.gemini/antigravity-cli/skills/bundlecheck"
      mkdir -p "$agy_dir"
      cp .agents/skills/bundlecheck/SKILL.md "$agy_dir/SKILL.md"
      printf 'Installed bundlecheck skill at %s\n' "$agy_dir"
    fi
  fi
fi

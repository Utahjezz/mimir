#!/usr/bin/env bash
# Install the mimir skill for Claude Code (global) or OpenCode (project-local).
#
# Usage:
#   bash skills/install.sh             # Claude Code — installs to ~/.claude/skills/mimir/
#   bash skills/install.sh --opencode  # OpenCode    — installs to .opencode/skills/mimir/ in cwd

set -euo pipefail

SKILL_NAME="mimir"
SKILLS_DIR="$(cd "$(dirname "$0")" && pwd)"
OPENCODE=false

for arg in "$@"; do
  case "$arg" in
    --opencode) OPENCODE=true ;;
    --help|-h)
      echo "Usage: bash skills/install.sh [--opencode]"
      echo ""
      echo "  (no flag)    Install for Claude Code globally (~/.claude/skills/mimir/)"
      echo "  --opencode   Install for OpenCode in the current project (.opencode/skills/mimir/)"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $arg" >&2
      echo "Usage: bash skills/install.sh [--opencode]" >&2
      exit 1
      ;;
  esac
done

if [ "$OPENCODE" = true ]; then
  TARGET_DIR="$(pwd)/.opencode/skills/${SKILL_NAME}"
else
  TARGET_DIR="${HOME}/.claude/skills/${SKILL_NAME}"
fi

SOURCE_DIR="${SKILLS_DIR}/${SKILL_NAME}"

if [ ! -d "$SOURCE_DIR" ]; then
  echo "ERROR: skill source not found at ${SOURCE_DIR}" >&2
  exit 1
fi

mkdir -p "${TARGET_DIR}/references"
cp "$SOURCE_DIR/SKILL.md" "$TARGET_DIR/SKILL.md"
for f in "$SOURCE_DIR/references/"*; do
  [ -f "$f" ] && cp "$f" "$TARGET_DIR/references/"
done

echo "Installed skill '${SKILL_NAME}' to ${TARGET_DIR}"
if [ "$OPENCODE" = true ]; then
  echo "You can now use the mimir skill in OpenCode for this project."
else
  echo "You can now use /mimir in any Claude Code session."
fi

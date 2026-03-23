#!/usr/bin/env bash
# Install the mimir skill for Claude Code globally.
# Usage: bash skills/install.sh
#   or:  ./skills/install.sh

set -euo pipefail

SKILL_NAME="mimir"
SOURCE_DIR="$(cd "$(dirname "$0")/${SKILL_NAME}" && pwd)"
TARGET_DIR="${HOME}/.claude/skills/${SKILL_NAME}"

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
echo "You can now use /mimir in any Claude Code session."

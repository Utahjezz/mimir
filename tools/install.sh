#!/usr/bin/env bash
# Install the mimir OpenCode tool plugin.
#
# Usage:
#   bash tools/install.sh             # project-local: <cwd>/.opencode/tools/mimir.ts
#   bash tools/install.sh --global    # global:         ~/.config/opencode/tools/mimir.ts
#
# The tool file is automatically discovered by OpenCode from either location —
# no changes to opencode.json are required.

set -euo pipefail

TOOLS_DIR="$(cd "$(dirname "$0")" && pwd)"
TOOL_SRC="${TOOLS_DIR}/opencode/mimir.ts"
GLOBAL=false

for arg in "$@"; do
  case "$arg" in
    --global) GLOBAL=true ;;
    --help|-h)
      echo "Usage: bash tools/install.sh [--global]"
      echo ""
      echo "  (no flag)  Install for current project  (<cwd>/.opencode/tools/mimir.ts)"
      echo "  --global   Install globally              (~/.config/opencode/tools/mimir.ts)"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $arg" >&2
      echo "Usage: bash tools/install.sh [--global]" >&2
      exit 1
      ;;
  esac
done

if [ ! -f "$TOOL_SRC" ]; then
  echo "ERROR: tool source not found at ${TOOL_SRC}" >&2
  exit 1
fi

if [ "$GLOBAL" = true ]; then
  TARGET_DIR="${HOME}/.config/opencode/tools"
else
  TARGET_DIR="$(pwd)/.opencode/tools"
fi

mkdir -p "$TARGET_DIR"
cp "$TOOL_SRC" "$TARGET_DIR/mimir.ts"

echo "Installed mimir tool to ${TARGET_DIR}/mimir.ts"
if [ "$GLOBAL" = true ]; then
  echo "The mimir tools (mimir_index, mimir_search, etc.) are now available in all OpenCode sessions."
else
  echo "The mimir tools (mimir_index, mimir_search, etc.) are now available in OpenCode for this project."
fi

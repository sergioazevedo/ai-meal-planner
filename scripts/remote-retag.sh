#!/bin/bash
set -e

# Regenerate only the bilingual tags and embeddings on a deployed server.
# Usage: ./scripts/remote-retag.sh <TARGET_HOST> <GHOST_ID|--all>

TARGET="${1}"
SELECTION="${2}"

if [ -z "$TARGET" ] || [ -z "$SELECTION" ]; then
    echo "Usage: $0 <TARGET_HOST> <GHOST_ID|--all>"
    exit 1
fi

if [ "$SELECTION" = "--all" ]; then
    RETAG_ARGS="-all"
else
    RETAG_ARGS="-id '$SELECTION'"
fi

ssh "$TARGET" "cd /home/ubuntu && set -a && . ./.env && set +a && ./ai-meal-planner-linux retag $RETAG_ARGS"

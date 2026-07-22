#!/bin/bash
set -e

# Regenerate only the bilingual tags and embedding for one deployed recipe.
# Usage: ./scripts/remote-retag.sh <TARGET_HOST> <GHOST_ID>

TARGET="${1}"
ID="${2}"

if [ -z "$TARGET" ] || [ -z "$ID" ]; then
    echo "Usage: $0 <TARGET_HOST> <GHOST_ID>"
    exit 1
fi

ssh "$TARGET" "cd /home/ubuntu && set -a && . ./.env && set +a && ./ai-meal-planner-linux retag -id '$ID'"

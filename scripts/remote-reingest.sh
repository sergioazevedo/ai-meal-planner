#!/bin/bash
# Remote control for re-ingesting a single recipe by ID
# Usage: ./scripts/remote-reingest.sh <TARGET_HOST> <GHOST_ID>
# Example: ./scripts/remote-reingest.sh personal-blog p123

TARGET="${1}"
ID="${2}"

if [ -z "$TARGET" ] || [ -z "$ID" ]; then
    echo "Usage: $0 <TARGET_HOST> <GHOST_ID>"
    exit 1
fi

ssh "$TARGET" "export \$(grep -v '^#' /home/ubuntu/.env | xargs) && /home/ubuntu/ai-meal-planner-linux reingest -id $ID"

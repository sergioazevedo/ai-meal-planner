#!/bin/bash
# Remote control for Ingesting recipes
# Usage: ./scripts/remote-ingest.sh <TARGET_HOST>
# Example: ./scripts/remote-ingest.sh personal-blog

TARGET="${1}"

FORCE="${2}"



if [ -z "$TARGET" ]; then

    echo "Usage: $0 <TARGET_HOST> [-force]"

    exit 1

fi



ARGS="ingest"

if [ "$FORCE" == "-force" ]; then

    ARGS="ingest -force"

fi



ssh "$TARGET" "export \$(grep -v '^#' /home/ubuntu/.env | xargs) && /home/ubuntu/ai-meal-planner-linux $ARGS"

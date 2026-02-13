#!/bin/bash
# Local script to re-ingest a single recipe by ID
# Usage: ./scripts/reingest.sh <GHOST_ID>

ID="${1}"

if [ -z "$ID" ]; then
    echo "Usage: $0 <GHOST_ID>"
    exit 1
fi

# Ensure the binary is built
make build

# Run the command with environment variables
export $(grep -v '^#' .env | xargs)
./bin/ai-meal-planner reingest -id "$ID"

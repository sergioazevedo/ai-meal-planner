#!/bin/bash
set -e

# Regenerate only the bilingual tags and embeddings locally.
# Usage: ./scripts/retag.sh <GHOST_ID|--all>

SELECTION="${1}"

if [ -z "$SELECTION" ]; then
    echo "Usage: $0 <GHOST_ID|--all>"
    exit 1
fi

if [ ! -f .env ]; then
    echo "Error: .env file not found. Please create one based on .env.example."
    exit 1
fi

make build

set -a
. ./.env
set +a

if [ "$SELECTION" = "--all" ]; then
    ./bin/ai-meal-planner retag -all
else
    ./bin/ai-meal-planner retag -id "$SELECTION"
fi

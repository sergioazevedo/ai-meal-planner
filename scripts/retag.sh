#!/bin/bash
set -e

# Regenerate only the bilingual tags and embedding for one local recipe.
# Usage: ./scripts/retag.sh <GHOST_ID>

ID="${1}"

if [ -z "$ID" ]; then
    echo "Usage: $0 <GHOST_ID>"
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

./bin/ai-meal-planner retag -id "$ID"

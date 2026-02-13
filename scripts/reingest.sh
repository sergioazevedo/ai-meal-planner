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
if [ ! -f .env ]; then
    echo "Error: .env file not found. Please create one based on .env.example."
    exit 1
fi

export $(grep -v '^#' .env | xargs)
./bin/ai-meal-planner reingest -id "$ID"

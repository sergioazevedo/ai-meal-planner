#!/bin/bash

# Metrics Cleanup Script
# This script should be run via cron daily.
# Example crontab entry:
# 0 0 * * * /home/ubuntu/projects/meal-planner/scripts/metrics-cleanup.sh >> /home/ubuntu/projects/meal-planner/data/cleanup.log 2>&1

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo "--- $(date) ---"
echo "Running metrics cleanup (retention: 30 days)..."

# Ensure we have the latest binary or run via go run
if [ -f "./bin/ai-meal-planner" ]; then
    ./bin/ai-meal-planner metrics-cleanup -days 30
else
    go run cmd/ai-meal-planner/main.go metrics-cleanup -days 30
fi

echo "Cleanup complete."

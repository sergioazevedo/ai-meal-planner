#!/bin/bash
set -e

# Configuration
CLI_BINARY="ai-meal-planner-linux"
REMOTE_USER="ubuntu"
REMOTE_HOST="${1}"
PEM_KEY="${2}"
FORCE_FLAG="${3}"

if [ -z "$REMOTE_HOST" ]; then
    echo "Usage: ./scripts/remote-ingest.sh <REMOTE_IP_OR_ALIAS> [PATH_TO_PEM] [-force]"
    exit 1
fi

ARGS="ingest"
if [ "$FORCE_FLAG" == "-force" ] || [ "$PEM_KEY" == "-force" ]; then
    ARGS="ingest -force"
    # If the second argument was -force, nullify the PEM key to avoid ssh errors
    if [ "$PEM_KEY" == "-force" ]; then
        PEM_KEY=""
    fi
fi

echo "Running remote ingestion on $REMOTE_HOST (Args: $ARGS)..."

CMD_INGEST="cd /home/$REMOTE_USER && set -a && . ./.env && set +a && ./$CLI_BINARY $ARGS"

if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "$CMD_INGEST"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "$CMD_INGEST"
fi

echo "Ingestion completed successfully."
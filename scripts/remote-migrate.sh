#!/bin/bash
set -e

# Configuration
CLI_BINARY="ai-meal-planner-linux"
REMOTE_USER="ubuntu"
REMOTE_HOST="${1}"
PEM_KEY="${2}"

if [ -z "$REMOTE_HOST" ]; then
    echo "Usage: ./scripts/remote-migrate.sh <REMOTE_IP_OR_ALIAS> [PATH_TO_PEM]"
    echo "Example with IP and Key: ./scripts/remote-migrate.sh 1.2.3.4 /path/to/key.pem"
    echo "Example with SSH Config: ./scripts/remote-migrate.sh my-server-alias"
    exit 1
fi

echo "Running database migrations on $REMOTE_HOST..."

CMD_MIGRATE="cd /home/$REMOTE_USER && set -a && . ./.env && set +a && ./$CLI_BINARY migrate up"

if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "$CMD_MIGRATE"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "$CMD_MIGRATE"
fi

echo "Migrations completed successfully."

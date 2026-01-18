#!/bin/bash
set -e

# Configuration
BINARY_NAME="ai-meal-planner-linux"
REMOTE_USER="ubuntu"
REMOTE_HOST="${1}"
PEM_KEY="${2}"

if [ -z "$REMOTE_HOST" ]; then
    echo "Usage: ./deploy.sh <REMOTE_IP_OR_ALIAS> [PATH_TO_PEM]"
    echo "Example with IP and Key: ./deploy.sh 1.2.3.4 /path/to/key.pem"
    echo "Example with SSH Config: ./deploy.sh my-server-alias"
    exit 1
fi

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o "$BINARY_NAME" ./cmd/ai-meal-planner

echo "Uploading to $REMOTE_HOST..."

if [ -n "$PEM_KEY" ]; then
    # Use provided key
    scp -i "$PEM_KEY" "$BINARY_NAME" "$REMOTE_USER@$REMOTE_HOST:/home/$REMOTE_USER/"
else
    # Use SSH config or agent
    scp "$BINARY_NAME" "$REMOTE_USER@$REMOTE_HOST:/home/$REMOTE_USER/"
fi

echo "Making the binary executable on $REMOTE_HOST..."
if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "chmod +x /home/$REMOTE_USER/$BINARY_NAME"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "chmod +x /home/$REMOTE_USER/$BINARY_NAME"
fi

echo "Deploy complete. Binary is now executable at /home/$REMOTE_USER/$BINARY_NAME"
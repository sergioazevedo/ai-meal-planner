#!/bin/bash
set -e

# Configuration
BINARY_NAME="ai-meal-planner-linux"
REMOTE_USER="ubuntu"
# Default to an IP passed as argument, or fallback to a placeholder
REMOTE_HOST="${1:-your-lightsail-ip}"
PEM_KEY="${2:-/path/to/your-key.pem}"

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o "$BINARY_NAME" ./cmd/ai-meal-planner

echo "Uploading to $REMOTE_HOST..."
if [ "$REMOTE_HOST" == "your-lightsail-ip" ]; then
    echo "Usage: ./deploy.sh <REMOTE_IP> <PATH_TO_PEM>"
    echo "Using placeholder values (dry run of build)."
    exit 0
fi

scp -i "$PEM_KEY" "$BINARY_NAME" "$REMOTE_USER@$REMOTE_HOST:/home/$REMOTE_USER/"

echo "Deploy complete. Binary is at /home/$REMOTE_USER/$BINARY_NAME"
echo "Don't forget to run: chmod +x $BINARY_NAME"

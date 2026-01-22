#!/bin/bash
set -e

# Configuration
CLI_BINARY="ai-meal-planner-linux"
BOT_BINARY="telegram-bot-linux"
REMOTE_USER="ubuntu"
REMOTE_HOST="${1}"
PEM_KEY="${2}"

if [ -z "$REMOTE_HOST" ]; then
    echo "Usage: ./deploy.sh <REMOTE_IP_OR_ALIAS> [PATH_TO_PEM]"
    echo "Example with IP and Key: ./deploy.sh 1.2.3.4 /path/to/key.pem"
    echo "Example with SSH Config: ./deploy.sh my-server-alias"
    exit 1
fi

echo "Building binaries for Linux..."
mkdir -p bin
GOOS=linux GOARCH=amd64 go build -o "bin/$CLI_BINARY" ./cmd/ai-meal-planner
GOOS=linux GOARCH=amd64 go build -o "bin/$BOT_BINARY" ./cmd/telegram-bot

echo "Stopping remote service to allow binary update..."
CMD_STOP="sudo systemctl stop meal-planner-bot || true"
if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "$CMD_STOP"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "$CMD_STOP"
fi

echo "Uploading to $REMOTE_HOST..."
if [ -n "$PEM_KEY" ]; then
    # Use provided key
    scp -i "$PEM_KEY" "bin/$CLI_BINARY" "bin/$BOT_BINARY" "$REMOTE_USER@$REMOTE_HOST:/home/$REMOTE_USER/"
else
    # Use SSH config or agent
    scp "bin/$CLI_BINARY" "bin/$BOT_BINARY" "$REMOTE_USER@$REMOTE_HOST:/home/$REMOTE_USER/"
fi

echo "Making binaries executable on $REMOTE_HOST..."
CMD_CHMOD="chmod +x /home/$REMOTE_USER/$CLI_BINARY /home/$REMOTE_USER/$BOT_BINARY"
if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "$CMD_CHMOD"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "$CMD_CHMOD"
fi

echo "Deploy complete."
echo "CLI: /home/$REMOTE_USER/$CLI_BINARY"
echo "BOT: /home/$REMOTE_USER/$BOT_BINARY"
echo ""
echo "Starting the meal-planner-bot service..."
if [ -n "$PEM_KEY" ]; then
    ssh -i "$PEM_KEY" "$REMOTE_USER@$REMOTE_HOST" "sudo systemctl start meal-planner-bot"
else
    ssh "$REMOTE_USER@$REMOTE_HOST" "sudo systemctl start meal-planner-bot"
fi

echo "Service started successfully."
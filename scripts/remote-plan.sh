#!/bin/bash
# Remote control for Generating a Meal Plan
# Usage: ./scripts/remote-plan.sh <TARGET_HOST> [REQUEST]
# Example: ./scripts/remote-plan.sh personal-blog "Vegetarian dinner"

TARGET="${1}"

REQUEST="${2:-Healthy meal plan for 3 days}"

USER_ID="${3:-cli_user}"



if [ -z "$TARGET" ]; then

    echo "Usage: $0 <TARGET_HOST> [REQUEST] [USER_ID]"

    exit 1

fi



ssh "$TARGET" "export \$(grep -v '^#' /home/ubuntu/.env | xargs) && /home/ubuntu/ai-meal-planner-linux plan -request \"$REQUEST\" -user \"$USER_ID\""

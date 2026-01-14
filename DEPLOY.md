# Deployment Guide

This guide covers two methods to deploy the AI Meal Planner:
1.  **Direct Binary (Recommended):** Simplest, lowest resource usage. Ideal for small Lightsail instances.
2.  **Docker:** Best for portability and isolation.

---

## Method 1: Direct Binary (Recommended)

Since Go compiles into a single static binary, you can use the provided scripts to build and upload it to your server.

### 1. Initial Setup on Server
SSH into your server once to set up your keys:

```bash
ssh your-server-alias
# Create the local data directory
mkdir -p data/recipes

# Create a secure .env file with your API keys
cat << 'EOF' > .env
GHOST_API_URL="https://your-blog.com"
GHOST_CONTENT_API_KEY="your_key"
GEMINI_API_KEY="your_key"
EOF
chmod 600 .env
```

### 2. Build and Deploy
From your **local machine**, use the deployment script:

```bash
# Using an IP and PEM key
./scripts/deploy.sh <REMOTE_IP> /path/to/key.pem

# OR Using an SSH config alias
./scripts/deploy.sh your-server-alias
```

### 3. Remote Management
Use the remote control scripts to manage the app without manually SSHing in:

```bash
# Ingest (Update Cache)
./scripts/remote-ingest.sh your-server-alias

# Generate Plan
./scripts/remote-plan.sh your-server-alias "Vegetarian dinner for 2"
```

---

## Method 2: Docker (Alternative)

Useful if you don't want to manage binary versions or environment dependencies.

### 1. Build Image
```bash
docker build -t ai-meal-planner .
```

### 2. Run Container
Create a `.env` file with your keys, then run:

```bash
# Ingest
docker run --env-file .env -v $(pwd)/data:/root/data ai-meal-planner ingest

# Plan
docker run --env-file .env -v $(pwd)/data:/root/data ai-meal-planner plan -request "..."
```

---

## Automation: Keeping Recipes in Sync

The application does not run a background scheduler. To automatically fetch new recipes from Ghost, set up a **Cron Job** on your server.

### 1. Open the Crontab Editor
```bash
crontab -e
```

### 2. Add an Hourly Sync
Add one of the following lines to the end of the file to sync every hour at minute 0:

**If using Binary:**
```bash
0 * * * * cd /home/ubuntu && GHOST_URL="..." GHOST_CONTENT_API_KEY="..." GEMINI_API_KEY="..." ./ai-meal-planner-linux ingest >> /home/ubuntu/ingest.log 2>&1
```

**If using Docker:**
```bash
0 * * * * docker run --rm --env-file /home/ubuntu/.env -v /home/ubuntu/data:/root/data ai-meal-planner ingest >> /home/ubuntu/ingest.log 2>&1
```

*Note: The `>> ingest.log 2>&1` part saves all output (and errors) to a log file so you can check if it's working.
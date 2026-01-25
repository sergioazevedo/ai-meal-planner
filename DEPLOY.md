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
GHOST_ADMIN_API_KEY="your_admin_key"
GEMINI_API_KEY="your_key"
GROQ_API_KEY="your_key"

# Household Settings
DEFAULT_ADULTS=2
DEFAULT_CHILDREN=1
DEFAULT_COOKING_FREQUENCY=4

# Access
TELEGRAM_ALLOWED_USER_IDS="12345678,87654321"
EOF
chmod 600 .env

### 💡 Migration Tip (v1.1)
If you are upgrading from an older version that used `recipes_data/`, move your files to the new unified path:
```bash
mkdir -p data/recipes
mv recipes_data/*.json data/recipes/ 2>/dev/null || true
```
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
0 * * * * cd /home/ubuntu && set -a && . ./.env && set +a && ./ai-meal-planner-linux ingest >> /home/ubuntu/ingest.log 2>&1
```

### 3. Cleanup Metrics (Optional but Recommended)
To keep your database small, schedule a daily cleanup of metrics older than 30 days:
```bash
0 0 * * * cd /home/ubuntu && set -a && . ./.env && set +a && ./ai-meal-planner-linux metrics-cleanup -days 30 >> /home/ubuntu/metrics-cleanup.log 2>&1
```

**If using Docker:**
```bash
0 * * * * docker run --rm --env-file /home/ubuntu/.env -v /home/ubuntu/data:/root/data ai-meal-planner ingest >> /home/ubuntu/ingest.log 2>&1
```

*Note: The `>> ingest.log 2>&1` part saves all output (and errors) to a log file so you can check if it's working.

---

## 🤖 Setting up the Telegram Bot (VPS)

Unlike the CLI, the bot needs to run continuously to listen for messages.

### 1. Update Environment Variables
SSH into your server and add the new Telegram variables to your `.env` file:

```bash
nano .env

# Add these lines:
TELEGRAM_BOT_TOKEN="your_token"
TELEGRAM_ALLOW_USER_ID="12345678"
# This must match your domain and the path in Nginx below
TELEGRAM_WEBHOOK_URL="https://your-blog.com/webhook"
```

### 2. Configure Systemd
Create a service file to keep the bot running 24/7 and restart it if it crashes.

```bash
sudo nano /etc/systemd/system/meal-planner-bot.service
```

Paste this content (adjust paths if needed):

```ini
[Unit]
Description=AI Meal Planner Telegram Bot
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu
# Load environment variables from the file we created
EnvironmentFile=/home/ubuntu/.env
# Run the binary
ExecStart=/home/ubuntu/telegram-bot-linux
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable meal-planner-bot
sudo systemctl start meal-planner-bot
```

### 3. Configure Nginx (Reverse Proxy)
You need to pass the HTTPS request from Telegram to your local bot.

**Important:** If you run multiple blogs, ensure this block is added to the specific server configuration for the subdomain used in your `TELEGRAM_WEBHOOK_URL` (e.g., `/etc/nginx/sites-available/youtblog.me`).

```nginx
server {
    server_name yourblog.me; # Your specific subdomain

    # ... existing ghost configuration ...

    # Add this location block for the bot
    location /webhook {
        proxy_pass http://127.0.0.1:8080/webhook;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Verify and Reload Nginx:**
```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 4. Deploy Updates
The `./scripts/deploy.sh` script is now configured to build both binaries, upload them, and automatically restart the `meal-planner-bot` service.

```bash
./scripts/deploy.sh your-server-alias
```

*Note: The very first time you deploy, you must manually create the systemd service file as described in Step 2 above.*
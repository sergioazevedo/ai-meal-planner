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
EMBEDDING_API_KEY="your_key"
GROQ_API_KEY="your_key"

# Optional model overrides. Omit these to use the application defaults.
# GROQ_ANALYST_MODEL="openai/gpt-oss-120b"
# GROQ_REVIEWER_MODEL="openai/gpt-oss-120b"
# GROQ_CHEF_MODEL="openai/gpt-oss-20b"
# GROQ_NORMALIZER_MODEL="openai/gpt-oss-20b"
# GROQ_TAGGER_MODEL="qwen/qwen3.6-27b"

# Household Settings
DEFAULT_ADULTS=2
DEFAULT_CHILDREN=1
DEFAULT_COOKING_FREQUENCY=5

# Access
TELEGRAM_ALLOWED_USER_IDS="12345678,87654321"
EOF
chmod 600 .env
```

### 💡 Migration Tip (v1.1)
If you are upgrading from an older version that used `recipes_data/`, move your files to the new unified path:
```bash
mkdir -p data/recipes
mv recipes_data/*.json data/recipes/ 2>/dev/null || true
```

### 2. Build and Deploy
From your **local machine**, use the deployment script:

```bash
# Using an IP and PEM key
./scripts/deploy.sh <REMOTE_IP> /path/to/key.pem

# OR Using an SSH config alias
./scripts/deploy.sh your-server-alias
```

### 2.5. Run Database Migrations (Important!)
After deploying new code, you will often need to apply database schema changes. This must be run **separately** after deployment and before starting the bot.

```bash
# Using an IP and PEM key
./scripts/remote-migrate.sh <REMOTE_IP> /path/to/key.pem

# OR Using an SSH config alias
./scripts/remote-migrate.sh your-server-alias
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

### 3. Cleanup Metrics & Audit Logs (Recommended)
To keep your database healthy while maintaining enough history for the PlanReviewer agent, schedule a daily cleanup of metrics and audit logs older than 60 days:
```bash
0 0 * * * cd /home/ubuntu && set -a && . ./.env && set +a && ./ai-meal-planner-linux metrics-cleanup -days 60 >> /home/ubuntu/metrics-cleanup.log 2>&1
```

**If using Docker:**
```bash
0 * * * * docker run --rm --env-file /home/ubuntu/.env -v /home/ubuntu/data:/root/data ai-meal-planner ingest >> /home/ubuntu/ingest.log 2>&1
```

*Note: The `>> ingest.log 2>&1` part saves all output (and errors) to a log file so you can check if it's working.

## Method 3: Automated Continuous Deployment (GitHub Actions)

The project is configured to automatically deploy to production whenever changes are merged into the `main` branch. This process is handled by a GitHub Action workflow.

### 1. Prerequisites
You must have the "Direct Binary" method (Method 1) setup completed on your server once (e.g., initial directory structure, `.env` file, and `systemd` service).

### 2. Configure GitHub Secrets
To enable the automated deployment, add the following secrets to your GitHub repository (`Settings > Secrets and variables > Actions`):

**Deployment Credentials:**
*   `DEPLOY_HOST`: The IP address or DNS name of your production server.
*   `DEPLOY_KEY`: The **entire contents** of the private SSH key used to access the server.

**Application Configuration (.env):**
*   `GROQ_API_KEY`: API key for Groq (required for evaluations and production).
*   `EMBEDDING_API_KEY`: API key for Embeddings (required for evaluations and production).
*   `GHOST_API_URL`: The URL of your Ghost blog.
*   `GHOST_CONTENT_API_KEY`: Your Ghost Content API key.
*   `GHOST_ADMIN_API_KEY`: Your Ghost Admin API key.
*   `TELEGRAM_BOT_TOKEN`: Your Telegram Bot token.
*   `TELEGRAM_ALLOW_USER_ID`: Telegram ID used by the current deployment workflow. The application also accepts the preferred `TELEGRAM_ALLOWED_USER_IDS` variable directly.
*   `TELEGRAM_WEBHOOK_URL`: The full URL to your bot's webhook.

**Optional Defaults (Overrides):**
*   `DEFAULT_ADULTS`, `DEFAULT_CHILDREN`, `DEFAULT_COOKING_FREQUENCY`, etc.

Add role-specific model overrides as GitHub Actions **variables**, not secrets, only when production should differ from the defaults in [GROQ.md](GROQ.md):

*   `GROQ_ANALYST_MODEL`
*   `GROQ_REVIEWER_MODEL`
*   `GROQ_CHEF_MODEL`
*   `GROQ_NORMALIZER_MODEL`
*   `GROQ_TAGGER_MODEL`

3. **How it Works**
1.  **Trigger:** On every push to `main` (including PR merges). **Note:** The pipeline is optimized with *path filters* to only trigger when relevant files change (Go code, SQL, dependencies, or workflow config), ignoring documentation-only or script-only changes.
2.  **Test:** Runs standard unit tests.
3.  **Evaluate:** Runs all planner and recipe live evals, followed by the retrieval quality eval.
4.  **Deploy:** If all tests pass, it:

    *   Securely injects the `DEPLOY_KEY`.
    *   Runs `./scripts/deploy.sh` to build, upload, and restart the service.
    *   Runs database migrations automatically via the script.

---

## 🤖 Setting up the Telegram Bot (VPS)

Unlike the CLI, the bot needs to run continuously to listen for messages.

### 1. Update Environment Variables
SSH into your server and add the new Telegram variables to your `.env` file:

```bash
nano .env

# Add these lines:
TELEGRAM_BOT_TOKEN="your_token"
TELEGRAM_ALLOWED_USER_IDS="12345678"
# This must match your domain and the path in Nginx below
TELEGRAM_WEBHOOK_URL="https://your-blog.com/webhook"
```

### 2. Configure Systemd (Rootless)
Create a user-level service file. This allows the bot to run without root privileges and enables the CI/CD pipeline to restart it securely.

```bash
mkdir -p ~/.config/systemd/user
nano ~/.config/systemd/user/meal-planner-bot.service
```

Paste this content (note that `User=` is removed as it's implicit):

```ini
[Unit]
Description=AI Meal Planner Telegram Bot
After=network.target

[Service]
Type=simple
WorkingDirectory=%h
# Load environment variables
EnvironmentFile=%h/.env
# Run the binary
ExecStart=%h/telegram-bot-linux
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

**Enable lingering** (crucial for user services to run without an active session):
```bash
sudo loginctl enable-linger $USER
```

Enable and start the service:
```bash
systemctl --user daemon-reload
systemctl --user enable meal-planner-bot
systemctl --user start meal-planner-bot
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

**Recommended Workflow:**
1.  **Deploy Code:** `./scripts/deploy.sh your-server-alias`
2.  **Run Migrations:** `./scripts/remote-migrate.sh your-server-alias` (if database changes are present)

*Note: The very first time you deploy, you must manually create the systemd service file as described in Step 2 above.*

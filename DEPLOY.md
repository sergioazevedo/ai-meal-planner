# Deployment Guide

This guide covers two methods to deploy the AI Meal Planner:
1.  **Direct Binary (Recommended):** Simplest, lowest resource usage. Ideal for small Lightsail instances.
2.  **Docker:** Best for portability and isolation.

---

## Method 1: Direct Binary (Recommended)

Since Go compiles into a single static binary, you can simply copy the file to your server.

### 1. Build for Linux
Run this command on your local machine (macOS/Windows) to cross-compile for Linux:

```bash

# For standard Intel/AMD servers (e.g., most Lightsail instances)
GOOS=linux GOARCH=amd64 go build -o ai-meal-planner-linux ./cmd/ai-meal-planner

# For ARM servers (e.g., AWS Graviton)
# GOOS=linux GOARCH=arm64 go build -o ai-meal-planner-linux ./cmd/ai-meal-planner

```

### 2. Copy to Server
Use `scp` to upload the binary. Replace `your-key.pem` and the IP address with your own.

```bash

scp -i /path/to/your-key.pem ai-meal-planner-linux ubuntu@your-lightsail-ip:/home/ubuntu/

```

### 3. Setup on Server
SSH into your server and run these commands once:

```bash

# Make the binary executable
chmod +x ai-meal-planner-linux

# Create the local data directory for recipes
mkdir -p data/recipes

```

### 4. Run the Application
You need to provide the API keys. You can run it inline:

```bash

# Ingest (Update Cache)

export GHOST_URL="https://your-blog.com"
export GHOST_API_KEY="your_ghost_key"
export GEMINI_API_KEY="your_gemini_key"
./ai-meal-planner-linux ingest

# Generate Plan

export GHOST_URL="https://your-blog.com"
export GHOST_API_KEY="your_ghost_key"
export GEMINI_API_KEY="your_gemini_key"
./ai-meal-planner-linux plan -request "Vegetarian dinner for 2"

```

*Tip: Add the `export VAR="..."` lines to your `~/.bashrc` file so you don't have to type them every time.

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

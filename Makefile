# AI Meal Planner Makefile

.PHONY: build test test-short eval ingest plan help migrate-up migrate-down migrate-create

# Default target
help:
	@echo "Usage:"
	@echo "  make build             - Build all binaries"
	@echo "  make build-linux       - Build all linux binaries"
	@echo "  make test              - Run all unit tests (skipping live LLM evals)"
	@echo "  make eval              - Run live LLM evaluation tests (costs money!)"
	@echo "  make ingest            - Run local ingestion"
	@echo "  make metrics-cleanup   - Clean up old metrics data (30 days)"
	@echo "  make migrate-up        - Apply all pending database migrations"
	@echo "  make migrate-down      - Revert the last applied database migration"
	@echo "  make migrate-create NAME=<name> - Create a new migration file"

# --- Development ---

build:
	go build -o bin/ai-meal-planner ./cmd/ai-meal-planner
	go build -o bin/telegram-bot ./cmd/telegram-bot

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "bin/ai-meal-planner-linux" ./cmd/ai-meal-planner
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "bin/telegram-bot-linux" ./cmd/telegram-bot

test:
	go test -short -v ./internal/...

# Run only the live LLM evaluation tests
eval:
	go test -v ./internal/planner -run "_Eval"

# Database Migrations
migrate-up:
	go run ./cmd/ai-meal-planner migrate up

migrate-down:
	go run ./cmd/ai-meal-planner migrate down

migrate-create:
	migrate create -ext sql -dir internal/database/migrations ${NAME}

# --- Local Execution ---

ingest:
	go run cmd/ai-meal-planner/main.go ingest

metrics-cleanup:
	go run cmd/ai-meal-planner/main.go metrics-cleanup -days 30

# --- Remote Scripts ---

remote-ingest:
	./scripts/remote-ingest.sh

remote-plan:
	./scripts/remote-plan.sh

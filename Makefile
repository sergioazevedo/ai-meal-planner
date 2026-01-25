# AI Meal Planner Makefile

.PHONY: build test test-short eval ingest plan help

# Default target
help:
	@echo "Usage:"
	@echo "  make build         - Build all binaries"
	@echo "  make test          - Run all unit tests (skipping live LLM evals)"
	@echo "  make eval          - Run live LLM evaluation tests (costs money!)"
	@echo "  make ingest        - Run local ingestion"
	@echo "  make plan          - Run local planning"

# --- Development ---

build:
	go build -o bin/ai-meal-planner ./cmd/ai-meal-planner
	go build -o bin/telegram-bot ./cmd/telegram-bot

test:
	go test -short -v ./internal/...

# Run only the live LLM evaluation tests
eval:
	go test -v ./internal/planner -run "_Eval"

# --- Local Execution ---

ingest:
	go run cmd/ai-meal-planner/main.go ingest

plan:
	@read -p "What would you like to eat? " prompt; \
	go run cmd/ai-meal-planner/main.go plan "$$prompt"

# --- Remote Scripts ---

remote-ingest:
	./scripts/remote-ingest.sh

remote-plan:
	./scripts/remote-plan.sh

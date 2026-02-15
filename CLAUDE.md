# Claude Code Project Guide: AI Meal Planner

## 🎯 Project Overview

**AI-Assisted Recipe Meal Planner** is a Go-based intelligent system that:
- Fetches recipes from Ghost CMS blogs
- Uses Google Gemini & Groq LLMs to normalize and embed recipes
- Generates personalized weekly meal plans via a **multi-agent architecture**
- Provides a Telegram bot interface for real-time interaction

**Key Philosophy**: Cost-conscious design optimized for free-tier AI APIs and minimal resource usage (~15MB memory).

---

## 🏗️ Architecture Overview

### Multi-Agent System
The planner uses a **handover pattern** between specialized agents:

1. **Analyst Agent** (`internal/planner/analyst.go`)
   - Strategic reasoning about meal preferences
   - Selects recipes using semantic search (RAG)
   - Enforces batch-cooking cadence (5-session weekly rhythm)
   - Ensures variety and nutritional balance

2. **Chef Agent** (`internal/planner/chef.go`)
   - Scales ingredient quantities (handles adults + children)
   - Consolidates shopping lists
   - Formats final markdown output
   - Handles leftovers and meal timing

### Data Flow
```
Ghost Blog → Ingestion → Normalization (LLM) → SQLite
                                                    ↓
                                            Vector Embeddings (Cached)
                                                    ↓
User Request → RAG Search → Analyst Agent → Chef Agent → Telegram Response
```

---

## 📁 Directory Structure

### Core Directories
- **`cmd/`** - Entry points
  - `ai-meal-planner/` - CLI tool (ingest, migrate, metrics-cleanup)
  - `telegram-bot/` - Telegram bot service

- **`internal/`** - Application logic
  - `app/` - Orchestration (ingestion pipeline, recipe processing)
  - `planner/` - Multi-agent planning system (Analyst, Chef)
  - `recipe/` - Recipe data model & repository
  - `llm/` - LLM integrations (Gemini, Groq) + caching
  - `ghost/` - Ghost CMS API client
  - `telegram/` - Telegram bot handler
  - `clipper/` - Recipe URL extraction & normalization
  - `database/` - SQLite setup + migrations
  - `metrics/` - Usage tracking & observability
  - `config/` - Configuration management

- **`internal/database/migrations/`** - SQL migration files (schema evolution)
- **`acceptance_tests/`** - Integration tests (RAG pipeline, caching, etc.)
- **`scripts/`** - Deployment & automation scripts
- **`data/db/`** - SQLite database (created at runtime, not in git)

---

## 🔧 Development Commands

All commands run via `make` (see `Makefile`):

### Building
```bash
make build              # Build both binaries (native)
make build-linux       # Build for Linux deployment
```

### Testing
```bash
make test              # Run unit tests (skips expensive evals)
make eval              # Run live LLM evaluations (costs API credits!)
```

### Database Management
```bash
make migrate-up        # Apply all pending migrations
make migrate-down      # Revert last migration
make migrate-create NAME=my_feature  # Create new migration file
```

### Local Execution
```bash
make ingest            # Fetch & ingest recipes from Ghost
make metrics-cleanup   # Clean up metrics older than 30 days
```

### Remote Scripts
```bash
make remote-ingest     # Run ingestion on deployed server
make remote-plan       # Run planning on deployed server
```

---

## ⚙️ Configuration

### Environment Variables (`.env` file)
**Required:**
- `GHOST_URL` - Your Ghost blog URL
- `GHOST_CONTENT_API_KEY` - Ghost Content API key
- `GEMINI_API_KEY` - Google Gemini API key
- `GROQ_API_KEY` - Groq API key (for LLM inference)

**Optional (Telegram):**
- `TELEGRAM_BOT_TOKEN` - Bot token from @BotFather
- `TELEGRAM_ALLOWED_USER_IDS` - Comma-separated user IDs
- `GHOST_ADMIN_API_KEY` - For Recipe Clipper feature

**Optional (Customization):**
- `DATABASE_PATH` - SQLite DB location (default: `data/db/planner.db`)
- `DEFAULT_ADULTS`, `DEFAULT_CHILDREN`, `DEFAULT_CHILDREN_AGES` - Household composition
- `DEFAULT_COOKING_FREQUENCY` - Sessions per week (default: 4)

---

## 🔑 Key Files to Know

### Ingestion Pipeline
- **`internal/app/ingestion.go`** - Main ingestion orchestration
- **`internal/recipe/extractor.go`** - LLM-based recipe normalization
- **`internal/ghost/ghost.go`** - Ghost CMS client

### Multi-Agent Planning
- **`internal/planner/planner.go`** - Main planner orchestrator
- **`internal/planner/analyst.go`** - Recipe selection logic
- **`internal/planner/chef.go`** - Scaling & formatting logic
- **`internal/planner/analyst_eval_test.go`** - Quality tests for Analyst

### RAG & Embeddings
- **`internal/llm/vector_repository.go`** - Semantic search interface
- **`internal/llm/cached_embedding_generator.go`** - Caching layer for embeddings
- **`internal/llm/gemini.go`** - Gemini LLM client
- **`internal/llm/groq.go`** - Groq LLM client

### Database
- **`internal/database/db.go`** - Database initialization & migrations runner
- **`internal/database/migrations/`** - SQL migration files (version-controlled)

### Telegram Bot
- **`internal/telegram/bot.go`** - Bot command handlers
- **`cmd/telegram-bot/main.go`** - Bot entry point

### Testing
- **`acceptance_tests/`** - Integration tests
  - `caching_acceptance_test.go` - Tests caching behavior
  - `vector_repository_integration_test.go` - Tests RAG search

---

## 📝 Development Conventions

### Code Style
- **Go idioms**: Follow standard Go conventions (effective Go)
- **Error handling**: Explicit error returns (no panics in critical paths)
- **Testing**: Unit tests for business logic, integration tests for pipelines
- **Comments**: Minimal but clear; explain *why*, not *what*

### Git Workflow
- Commits should be atomic and focused
- Use imperative mood: "Add feature" not "Added feature"
- Example good commits:
  - `feat: add reingest command to CLI for single recipe processing`
  - `fix: implement pagination handling in Ghost recipe fetching`
  - `chore: add .env check to reingest script`

### Testing Strategy
- **Unit Tests** (`*_test.go` files): Test business logic in isolation
- **Integration Tests** (`acceptance_tests/`): Test full pipelines with real SQLite
- **Live Evals** (`*_eval_test.go`): Test LLM quality (slow, costs API calls)
  - Run with: `go test -v ./internal/planner -run "_Eval"`
  - Only run during development, not in CI

### Database Migrations
- Every schema change requires a migration file
- Files in `internal/database/migrations/` are auto-discovered
- Migrations are idempotent and reversible
- Always test migrations on a copy of production data
- Pattern: `001_initial_schema.up.sql`, `001_initial_schema.down.sql`

---

## 🧪 Testing & Quality

### Running Tests
```bash
# Fast unit tests (most common)
make test

# Live LLM evaluations (expensive but necessary for quality)
make eval

# Specific test
go test -v ./internal/planner -run TestAnalyst_LiveEval
```

### Test Organization
- **Unit tests**: Mock external dependencies
- **Acceptance tests**: Use real SQLite (in-memory or temp files)
- **Live evals**: Hit real APIs, track cost, only during active development

### Coverage Notes
- Focus on business logic (planner, recipe extraction, RAG search)
- Telegram bot is tested manually due to stateful webhook interactions
- LLM agent quality is validated via live eval tests (non-deterministic)

---

## 🚀 Common Tasks

### Adding a New Feature
1. Create feature branch: `git checkout -b feat/my-feature`
2. Write tests first (TDD preferred)
3. Implement feature
4. Run `make test` to verify
5. Commit with descriptive message
6. If schema changes needed: `make migrate-create NAME=my_feature`

### Debugging LLM Integration
- Check embedding caching in `internal/llm/cached_embedding_generator.go`
- Review LLM prompts in `internal/planner/analyst.go` and `chef.go`
- Use live evals to validate agent behavior: `make eval`
- Monitor metrics in Telegram `/metrics` command

### Working with Ghost CMS
- Ghost Content API is read-only (used for ingestion)
- Ghost Admin API is used for Recipe Clipper feature
- Pagination is handled in `internal/ghost/ghost.go`
- API keys are case-sensitive and environment-specific

### Database Issues
1. Check migration status: Review `internal/database/migrations/`
2. Apply pending migrations: `make migrate-up`
3. Rollback if needed: `make migrate-down`
4. Delete and recreate database if corrupted (will re-ingest)

---

## 💡 Important Patterns

### Caching Strategy
- **Embedding caching**: Uses `text_hash` to avoid re-embedding identical content
- **Location**: `internal/llm/cached_embedding_generator.go`
- **Impact**: Dramatically reduces Gemini API usage during re-ingestion

### RAG Pipeline
- Semantic search via vector embeddings
- Located in `internal/llm/vector_repository.go`
- Returns K most similar recipes based on cosine similarity

### Agent Handover
- Analyst selects recipes → returns structured selection
- Chef receives selection → formats and scales
- No loops or back-and-forth; one-pass execution

### Household Scaling
- Adults contribute full portion
- Children scale based on age (see `internal/planner/chef.go`)
- Ingredients are proportionally adjusted

---

## 🎯 Quick Reference

| Task | Command |
|------|---------|
| Build project | `make build` |
| Run tests | `make test` |
| Ingest recipes | `make ingest` |
| Check metrics | Run `/metrics` in Telegram bot |
| Apply DB changes | `make migrate-up` |
| Create new migration | `make migrate-create NAME=feature_name` |
| Evaluate LLM quality | `make eval` |
| Deploy to Linux | `make build-linux` then copy binary |

---

## 📖 Further Reading

- **README.md** - User-facing project overview
- **DEPLOY.md** - Deployment guide
- **AI_AGENT_ROADMAP.md** - Future agent roles & capabilities
- **TODO.md** - Planned features and architectural decisions
- **Recent Commits** - Check git log for implementation patterns

---

## ⚠️ Important Notes

1. **Free-Tier APIs**: Project optimized for free tiers; monitor usage limits
2. **Database**: SQLite file at `data/db/planner.db`; not in git
3. **Secrets**: `.env` file is gitignored (see `.geminiignore`)
4. **Memory**: Designed for ~15MB footprint; efficient for low-resource servers
5. **LLM Costs**: Live evals (`make eval`) hit real APIs; can be expensive
6. **Migrations**: Always reversible; never delete migration files

---

Generated for Claude Code. Last updated: February 2026.

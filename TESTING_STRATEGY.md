# Testing strategy

The test suite separates deterministic application behavior from live model and retrieval quality. This keeps normal development fast while still checking the production integrations that mocks cannot represent.

## Test layers

### Unit and acceptance tests

```bash
make test
```

This runs `go test -short -v ./internal/...`. Short mode excludes live API calls. Tests cover parsing, validation, persistence, agent boundaries, migrations, and internal workflows using deterministic LLM mocks.

For the complete short suite, including acceptance tests and packages outside `internal`, run:

```bash
go test -short -v ./...
```

### Live evaluations

```bash
make eval
```

The target runs three groups:

1. Planner evals against Groq
2. Recipe extraction and tagging evals against Groq
3. Retrieval quality against the embedding provider

These tests consume API quota and can be affected by provider latency or rate limits. They use bounded timeouts and rate-limit pacing.

## Current live scenarios

| Area | What is checked |
| --- | --- |
| Analyst | Dietary constraints, nine-slot structure, cooking/leftover cadence, recipe reuse, and a light Sunday dinner |
| PlanReviewer | Targeted replacement, preservation of unchanged meals, recipe identity, tool use, original constraints, and recent-recipe exclusion |
| Chef | Output structure, cook/leftover labels, leftover preparation time, and shopping-list quantities |
| Normalizer | Portuguese salmon extraction, side-dish separation, ingredients, preparation time, and servings |
| Tagger | Ordered Portuguese/English tag pairs and rejection of unsupported dietary tags |
| Retrieval | Semantic ranking across a curated 48-recipe, 16-query bilingual dataset |

The retrieval thresholds are:

- Hit@1 at least `0.60`
- Recall@3 at least `0.70`
- MRR@3 at least `0.70`

The curated retrieval fixtures live in `internal/llm/testdata/`. Relevance labels are maintained independently from the retrieval implementation to avoid evaluating the system against labels it generated itself.

## Running one evaluation

```bash
# Planner roles
go test -v ./internal/planner -run TestAnalyst_LiveEval -count=1
go test -v ./internal/planner -run TestPlanReviewer_LiveEval -count=1
go test -v ./internal/planner -run TestPlanReviewer_BugReproduction_LiveEval -count=1
go test -v ./internal/planner -run TestChef_LiveEval -count=1

# Recipe roles
go test -v ./internal/recipe -run TestExtractor_LiveEval -count=1
go test -v ./internal/recipe -run TestTagger_LiveEval -count=1

# Retrieval
go test -v ./internal/llm -run TestVectorSearchQualityIntegration -count=1
```

Planner and recipe evals require `GROQ_API_KEY`. Retrieval requires `EMBEDDING_API_KEY`. Role-specific model overrides are documented in [GROQ.md](GROQ.md).

## CI behavior

Pull requests that change executable code, prompts, fixtures, the Makefile, or the workflow run:

1. The complete short test suite
2. All planner live evals
3. All recipe live evals
4. The retrieval quality eval

Live evals fail closed in CI when their required credential is missing. Production deployment runs only after the short tests and all live eval groups pass on a push to `main`.

Documentation-only changes do not currently trigger the pipeline.

## Interpreting failures

A live-eval failure is evidence, not automatically a model defect. Check which boundary failed:

- API errors such as `model_not_found` indicate provider availability or access problems.
- Timeouts and `429` responses usually indicate rate-limit pressure.
- Invalid JSON can require a repair attempt or stronger output validation.
- A valid response that violates a scenario indicates either unsuitable model behavior, a prompt gap, or an application boundary bug.

The PlanReviewer regression is an example of the last case. A replacement model preserved unchanged meals, but application code rebuilt their IDs from a lookup containing only newly retrieved recipes. The live eval exposed the missing identity mapping; the fix belonged in application code rather than in the model selection.

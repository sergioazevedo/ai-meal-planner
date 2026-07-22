# Groq integration and model configuration

The application uses Groq through the `llm.TextGenerator` interface. Each workload has its own model setting so a model can be replaced without changing the other roles.

## Authentication

Set the API key before running the application or live LLM evaluations:

```bash
export GROQ_API_KEY="your_api_key"
```

`config.NewFromEnv` requires this key for the application. Local live evals skip without it, while CI fails so missing credentials cannot produce a false green result.

## Models by role

| Role | Environment variable | Default |
| --- | --- | --- |
| Analyst | `GROQ_ANALYST_MODEL` | `openai/gpt-oss-120b` |
| PlanReviewer | `GROQ_REVIEWER_MODEL` | `openai/gpt-oss-120b` |
| Chef | `GROQ_CHEF_MODEL` | `openai/gpt-oss-20b` |
| Normalizer | `GROQ_NORMALIZER_MODEL` | `openai/gpt-oss-20b` |
| Tagger | `GROQ_TAGGER_MODEL` | `qwen/qwen3.6-27b` |

The defaults were selected against the repository's live scenarios. They remain configurable because provider availability, free-tier limits, and model behavior can change.

For example, to evaluate another Analyst without changing production code:

```bash
GROQ_ANALYST_MODEL="provider/model-id" \
go test -v ./internal/planner -run TestAnalyst_LiveEval -count=1
```

Run the matching live eval before changing a role's production default. The Tagger in particular must preserve ordered Portuguese/English pairs, which is not guaranteed by general model quality alone.

## Client usage

Create a client with an explicit model and temperature:

```go
client := llm.NewGroqClient(
    cfg,
    cfg.AnalystModel, // Model selected for this role.
    0.1,
)

response, err := client.GenerateContent(
    ctx,
    llm.Conversation{{Role: "user", Content: "Plan a week of dinners."}},
    llm.NoTools,
)
```

The client applies bounded retries to rate-limit responses. It also lowers or disables reasoning effort for supported model families to keep requests within the available token budget. Callers must still use context deadlines.

## Replacing a deprecated model

1. Override only the affected role through its environment variable.
2. Run that role's live eval several times if quota permits.
3. Run `make eval` to detect effects across the full pipeline.
4. Change the default only after the candidate passes the relevant behavior checks.

Model replacement is an operational change and an evaluation exercise. A model returning valid JSON is not enough if it loses plan identity, ignores constraints, or produces incomplete bilingual tags.

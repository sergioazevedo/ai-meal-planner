# Retrieval evaluation dataset

`rag_eval_recipes.json` is a stable, curated dataset containing the recipe fields
used for retrieval. `retrieval_queries.json` contains manually labelled Portuguese
and English queries with one or more relevant recipe IDs. The dataset intentionally
preserves mixed-language and inconsistent tags; normalizing only the fixture would
hide real retrieval and filtering problems.

Run the deterministic metric and fixture checks with the normal test suite:

```bash
go test -v ./internal/llm
```

Run the live embedding evaluation explicitly:

```bash
export EMBEDDING_API_KEY="..."
make eval-retrieval
```

The live evaluation embeds 48 recipes and 16 queries, reports Hit@1, Recall@3,
and MRR@3, and fails if a metric drops below its baseline in
`vector_repository_integration_test.go`.

When changing the fixture, keep ambiguous alternatives and update the relevance
labels deliberately. Do not generate labels from the retrieval system being
evaluated.

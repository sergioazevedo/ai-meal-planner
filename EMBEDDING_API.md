# Embedding API Integration Guide

This document provides technical details on how our Go application generates vector embeddings for semantic search.

---

## Architecture Overview

The application is provider-agnostic for embeddings. It uses a generic HTTP client (`internal/llm/embedding_client.go`) to communicate with standard OpenAI-compatible `/v1/embeddings` endpoints. 

By default, the application is configured to use **Mixedbread AI** (`mxbai-embed-large-v1`), which provides high-performance 1024-dimension vectors.

---

## Configuration

To use the embedding service, you must set the following environment variable:

```bash
export EMBEDDING_API_KEY="your_api_key_here"
```

The application will fail to start if this key is not provided.

---

## Go Client Usage

Interaction with the Embedding API is handled by the `EmbeddingGenerator` interface defined in `internal/llm/llm.go`.

### Initialization
To get a new client, use the `llm.NewEmbeddingClient` function.

```go
package main

import (
    "ai-meal-planner/internal/config"
    "ai-meal-planner/internal/llm"
)

func main() {
    cfg, err := config.NewFromEnv()
    // ... handle error

    embedClient := llm.NewEmbeddingClient(cfg)
    defer embedClient.Close()

    // ... use client
}
```

### Generating Embeddings (Vectors)
Embeddings are generated via the `GenerateEmbedding` method. This is used by the `recipe.Extractor` during ingestion and by the `recipe.SearchService` during meal planning.

```go
text := "Spaghetti Carbonara with eggs and bacon"
vector, err := embedClient.GenerateEmbedding(ctx, text)
if err != nil {
    log.Printf("Failed to generate embedding: %v", err)
} else {
    // vector is []float32
    // Mixedbread mxbai-embed-large-v1 returns a 1024-length vector.
    fmt.Printf("Generated vector of length: %d\n", len(vector))
}
```

---

## Troubleshooting

### Dimension Mismatch
If you change embedding providers (e.g., from one model with 768-dim vectors to another with 1024-dim vectors), you **must** re-ingest your recipes to rebuild the local vector database:

```bash
go run cmd/ai-meal-planner/main.go ingest --force
```

Failure to do so will result in empty or inaccurate search results because the cosine similarity calculation requires vectors of equal length.

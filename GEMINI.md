# Gemini API Integration Guide

This document provides technical details on how our Go application interacts with the Google Gemini API.

---

## Operational Constraints

*   **No Unsanctioned Coding:** You MUST NOT modify any files without a user-approved Plan.
*   **Planning First:** For every implementation request (Directive), you must first use the `enter_plan_mode` tool to research, design, and present a strategy.
*   **Explicit Consent:** After writing a plan to a file, you MUST present a high-level summary of the plan in the chat and stop to wait for the user to explicitly approve it. You are strictly forbidden from "self-approving" or proceeding to the 'Execution/Act' phase until a human provides approval.
*   **Tutor Defaults:** When in doubt, default to the 'tutor' persona (analyzing and proposing) rather than 'engineer' (implementing).

---

## Configuration

To use the available LLMs, you must set the following environment variables:

### Gemini

```bash
export GEMINI_API_KEY="your_api_key_here"
```

See the [Groq API Integration Guide](GROQ.md) for details on the Groq API.

### Groq

```bash
export GROQ_API_KEY="your_api_key_here"
```

The application will fail to start if the required keys are not provided.

---

## Go Client Usage

Interaction with the Gemini API is handled by the `LLMClient` interface (for embeddings) defined in `internal/llm/gemini.go`.

### Initialization
To get a new client, use the `llm.NewGeminiClient` function. It's crucial to `defer` the `Close()` method to release resources.

```go
package main

import (
    "context"
    "log"
    "ai-meal-planner/internal/config"
    "ai-meal-planner/internal/llm"
)

func main() {
    ctx := context.Background()
    cfg, err := config.NewFromEnv()
    // ... handle error

    geminiClient, err := llm.NewGeminiClient(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to create Gemini client: %v", err)
    }
    defer geminiClient.Close()

    // ... use client
}
```

### Generating Embeddings (Vectors)
Gemini is used exclusively for generating vector embeddings for semantic search using the **embedding-001** model via the `GenerateEmbedding` method. Note that the full recipe embedding generation and saving flow (including caching and vector storage interaction) is encapsulated in the `recipe.Extractor` struct.

```go
text := "Spaghetti Carbonara with eggs and bacon"
vector, err := geminiClient.GenerateEmbedding(ctx, text)
if err != nil {
    log.Printf("Failed to generate embedding: %v", err)
} else {
    // vector is []float32
    fmt.Printf("Generated vector of length: %d\n", len(vector))
}
```

---

## Prompt Engineering

### Forcing JSON Output

To ensure the model returns a valid JSON object without extra formatting or explanatory text, prompts should be very explicit.

**Good Practice:**

` ` `
You are a helpful assistant that only returns valid JSON. Do not add any other text.
...
Return the output as a JSON object with the following structure:
{
  "key": "value"
}

Input data:
...
` ` `

This direct instruction helps constrain the model's output to exactly what our application can parse.

```
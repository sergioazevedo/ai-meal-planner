# Gemini API Integration Guide

This document provides technical details on how our Go application interacts with the Google Gemini API.

---

## Configuration

To use the Gemini API, you must set the following environment variable:

```bash
export GEMINI_API_KEY="your_api_key_here"
```

The application will fail to start if this key is not provided.

---

## Go Client Usage

Interaction with the Gemini API is handled by the `LLMClient` interface defined in `internal/llm/gemini.go`.

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

### Generating Content (Text)
To generate content using **Gemini 1.5 Pro**, use the `GenerateContent` method.

```go
prompt := "Tell me a joke about a programmer."
response, err := geminiClient.GenerateContent(ctx, prompt)
if err != nil {
    log.Printf("Failed to generate content: %v", err)
} else {
    fmt.Println(response)
}
```

### Generating Embeddings (Vectors)
To generate vector embeddings for semantic search using the **embedding-001** model, use the `GenerateEmbedding` method.

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
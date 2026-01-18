# Groq API Integration Guide

This document provides technical details on how our Go application interacts with the Groq API.

---

## Configuration

To use the Groq API, you must set the following environment variable:

```bash
export GROQ_API_KEY="your_api_key_here"
```

The application will fail to start if this key is not provided.

---

## Go Client Usage

Interaction with the Groq API is handled by the `TextGenerator` interface defined in `internal/llm/llm.go`. The `groqClient` in `internal/llm/groq.go` implements this interface.

### Initialization
To get a new client, use the `llm.NewGroqClient` function.

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

    groqClient := llm.NewGroqClient(cfg)
    
    // ... use client
}
```

### Generating Content (Text)
To generate content using **Llama 3 70b**, use the `GenerateContent` method.

```go
prompt := "Tell me a joke about a programmer."
response, err := groqClient.GenerateContent(ctx, prompt)
if err != nil {
    log.Printf("Failed to generate content: %v", err)
} else {
    fmt.Println(response)
}
```

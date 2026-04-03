---
title: "The Hangover of Vibecoding: Paying Down AI-Generated Tech Debt"
custom_excerpt: "Vibecoding gives you incredible momentum, but eventually, you hit a wall of tight coupling. Here is how I use AI to refactor the technical debt it helped me create."
tags: [AI, Go, Refactoring, Architecture, Vibecoding]
meta_title: "Refactoring AI-Generated Tech Debt in Go"
meta_description: "A personal journey on balancing the speed of vibecoding with the necessity of clean architecture, focusing on decoupling an AI Agent in a Go application."
---

Vibecoding—the process of rapidly generating features alongside an AI—is an incredible productivity boost. You can spin up entire architectures, agent loops, and database integrations in an afternoon. The momentum is addictive.

But momentum has a cost. Recently, while trying to write a simple unit test for an AI **Analyst** agent in my Go project, I hit a wall. To test the agent, I had to mock a Vector DB repository, an Embedding Generator, and a half-dozen other unrelated components. 

The AI had helped me build the feature fast, but it had built it as a **God Object**. The `Planner` struct knew *everything* about the system, tightly coupling my business logic to my infrastructure.

Here is how I recognized the smell, and more importantly, how I guided the AI to help me clean up the mess it made.

### The Smell of the God Object

The issue surfaced when I tried to verify my new "Pull" model (where the AI agent autonomously calls a tool to fetch recipes instead of having them pushed into its prompt). I wanted a deterministic, fast unit test for this multi-turn loop.

When I looked at the code, I realized every single agent—the **Analyst**, the **Chef**, and the **PlanReviewer**—were methods bound directly to the `*Planner` struct. 

```go
// The original "God Object" approach
type Planner struct {
	recipeRepo        *recipe.Repository
	vectorRepo        *llm.VectorRepository
	planRepo          *PlanRepository
	analystGenerator  llm.TextGenerator 
	chefGenerator     llm.TextGenerator
	reviewerGenerator llm.TextGenerator
    // ... several other dependencies
}

// The analyst logic was tightly bound to the Planner's heavy dependencies
func (p *Planner) runAnalyst(...) {
   // ...
   // This call implicitly relied on the vectorRepo and embedGen
   recipes, err := p.getRecipeCandidates(ctx, toolCall.Args["query"], exclude) 
}
```

This is a classic code smell. To test the `Analyst` logic (the "GPS"), I had to build the entire `Planner` (the "Car"). If I wanted to test how the **Chef** formatted a shopping list, I still needed a database connection just to instantiate the parent struct.

### Steering the AI: Architect over Doer

One of the traps of vibecoding is accepting the first working solution. The AI is eager to please; if you ask it to "make the agent call a tool," it will bolt that tool onto whatever struct happens to be nearby.

To fix this, I had to stop prompting for *features* and start prompting for *architecture*. I explicitly invoked a more rigorous **"Architect over Doer"** principle. I told the AI: 

> "Don't just bolt on features to existing structs to make them work. If a request introduces tight coupling or a code smell, pause and propose a structural refactor."

By calling out the tight coupling, I forced the AI to evaluate the code against solid software engineering principles rather than just checking if it compiled.

### The Decoupling Strategy: Dismantling the God Object

Together, we mapped out a comprehensive refactor to dismantle the God Object and isolate the agents into their own domain logic.

1. **The Abstraction Boundary:** We identified that the Analyst doesn't care about vector databases; it just needs a way to find recipes. We defined a narrow `RecipeSearcher` interface.
2. **The "Implicit" Advantage:** In Go, we don't need to define the interface where we implement it. We defined `RecipeSearcher` right inside the `Analyst` package.
3. **The Isolated Agent Structs:** We moved the logic for each agent into its own struct with only the dependencies it actually uses.

#### The "Interface vs. Concrete" Debate

During the refactor, a question arose: *"If there's only one implementation of the searcher, why not just pass the concrete struct?"*

The answer is **Testability**. Even if there is only one *production* implementation, there is always a second implementation: the **Mock**. By using an interface, I can test the Analyst's tool loop with a simple five-line mock, completely ignoring the complex vector embedding logic.

### The Refactored Architecture

Here is what the decoupled `Analyst` looks like now. It's clean, isolated, and "dumb" to the rest of the system:

```go
// internal/planner/analyst.go

// The narrow contract the Analyst needs
type RecipeSearcher interface {
	GetRecipeCandidates(ctx context.Context, query string, excludeIDs []string) ([]recipe.Recipe, error)
}

// The isolated agent
type Analyst struct {
	llm      llm.TextGenerator
	searcher RecipeSearcher
}

func NewAnalyst(llm llm.TextGenerator, searcher RecipeSearcher) *Analyst {
	return &Analyst{llm: llm, searcher: searcher}
}

func (a *Analyst) Run(ctx context.Context, ...) (AnalystResult, error) {
    // ... clean execution loop using a.searcher ...
}
```

### The Payoff: DRY Testing

With the agent isolated, writing the unit test became trivial. I didn't need to mock a database; I just needed a simple mock that satisfied the interface.

```go
type mockSearcher struct {
    recipes []recipe.Recipe
}
func (m *mockSearcher) GetRecipeCandidates(ctx context.Context, query string, excludeIDs []string) ([]recipe.Recipe, error) {
    return m.recipes, nil
}
```

### Reflections on Vibecoding

Vibecoding is not a silver bullet. It trades architectural rigor for initial velocity. 

I found that you *can* use AI to write clean, decoupled systems, but you have to actively manage the "vibe." You have to switch personas. When you are prototyping, let the AI bolt things together. But once the feature works, you must put on your **"Staff Engineer"** hat, review the shape of the code, and instruct the AI to pay down the technical debt it just created.

The speed of vibecoding isn't just about writing code fast; it's also about having an untiring pair-programmer ready to execute a massive refactor the moment you identify a code smell.

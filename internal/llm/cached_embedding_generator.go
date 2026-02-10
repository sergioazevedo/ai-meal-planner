package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CachedEmbeddingGenerator wraps an EmbeddingGenerator to cache results
// to a file, reducing API calls and improving test performance.
type CachedEmbeddingGenerator struct {
	realGen       EmbeddingGenerator
	cache         map[string][]float32
	cacheFilePath string
	mu            sync.Mutex
}

// NewCachedEmbeddingGenerator creates a new CachedEmbeddingGenerator.
// It attempts to load the cache from the specified file path.
func NewCachedEmbeddingGenerator(realGen EmbeddingGenerator, cacheFilePath string) (*CachedEmbeddingGenerator, error) {
	c := &CachedEmbeddingGenerator{
		realGen:       realGen,
		cache:         make(map[string][]float32),
		cacheFilePath: cacheFilePath,
	}

	// Ensure the directory for the cache file exists
	cacheDir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	// Try to load existing cache
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Cache file not found, starting with empty cache: %s
", cacheFilePath)
			return c, nil
		}
		return nil, fmt.Errorf("failed to read cache file %s: %w", cacheFilePath, err)
	}

	if err := json.Unmarshal(data, &c.cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data from %s: %w", cacheFilePath, err)
	}

	fmt.Printf("Loaded %d embeddings from cache: %s
", len(c.cache), cacheFilePath)
	return c, nil
}

// GenerateEmbedding checks the cache first. If the embedding is not found,
// it calls the real generator, stores the result in the cache, and returns it.
func (c *CachedEmbeddingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if embedding, ok := c.cache[text]; ok {
		// fmt.Printf("Cache HIT for text: "%s"
", text)
		return embedding, nil
	}

	// fmt.Printf("Cache MISS for text: "%s", calling real generator...
", text)
	embedding, err := c.realGen.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding using real generator: %w", err)
	}

	c.cache[text] = embedding
	return embedding, nil
}

// SaveCache persists the current in-memory cache to the file system.
func (c *CachedEmbeddingGenerator) SaveCache() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	if err := os.WriteFile(c.cacheFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file %s: %w", c.cacheFilePath, err)
	}

	fmt.Printf("Saved %d embeddings to cache: %s
", len(c.cache), c.cacheFilePath)
	return nil
}

package llm

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"slices"

	db "ai-meal-planner/internal/llm/vector_db"
)

type VectorRepository struct {
	queries *db.Queries
	db      *sql.DB
}

func NewVectorRepository(d *sql.DB) *VectorRepository {
	return &VectorRepository{
		queries: db.New(d),
		db:      d,
	}
}

// WithTx returns a new VectorRepository that uses the provided transaction.
func (r *VectorRepository) WithTx(tx *sql.Tx) *VectorRepository {
	return &VectorRepository{
		queries: db.New(tx),
		db:      r.db,
	}
}

func (r *VectorRepository) Save(ctx context.Context, recipeID string, embedding []float32) error {
	embeddingBytes, err := float32SliceToByteSlice(embedding)
	if err != nil {
		return fmt.Errorf("failed to convert float32 slice to byte slice: %w", err)
	}

	params := db.InsertEmbeddingParams{
		RecipeID:  recipeID,
		Embedding: embeddingBytes,
	}

	return r.queries.InsertEmbedding(ctx, params)
}

func (r *VectorRepository) Get(ctx context.Context, recipeID string) ([]float32, error) {
	dbEmbedding, err := r.queries.GetEmbeddingByRecipeID(ctx, recipeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Embedding not found
		}
		return nil, fmt.Errorf("failed to get embedding by recipe ID: %w", err)
	}

	embedding, err := byteSliceToFloat32Slice(dbEmbedding.Embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to convert byte slice to float32 slice: %w", err)
	}
	return embedding, nil
}

// FindSimilar searches for recipes with embeddings similar to the query.
// It retrieves all embeddings, calculates cosine similarity, and fetches the corresponding
// recipe data for the top N similar recipes.
func (r *VectorRepository) FindSimilar(ctx context.Context, queryEmbedding []float32, limit int, excludeIDs []string) ([]string, error) {
	allEmbeddings, err := r.queries.ListAllEmbeddings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all embeddings: %w", err)
	}

	// Create a map for efficient exclusion lookup
	excludeMap := make(map[string]struct{})
	for _, id := range excludeIDs {
		excludeMap[id] = struct{}{}
	}

	type scoredRecipe struct {
		RecipeID string
		Score    float64
	}

	scoredRecipes := []scoredRecipe{}

	for _, dbEmbed := range allEmbeddings {
		// Skip if ID is in the exclusion list
		if _, excluded := excludeMap[dbEmbed.RecipeID]; excluded {
			continue
		}

		embed, err := byteSliceToFloat32Slice(dbEmbed.Embedding)
		if err != nil {
			fmt.Printf("Warning: Failed to convert embedding for recipe ID %s: %v", dbEmbed.RecipeID, err)
			continue
		}

		score := cosineSimilarity(queryEmbedding, embed)
		scoredRecipes = append(scoredRecipes, struct {
			RecipeID string
			Score    float64
		}{
			RecipeID: dbEmbed.RecipeID,
			Score:    score,
		})
	}

	// Sort by score descending
	slices.SortFunc(scoredRecipes, func(i, j scoredRecipe) int {
		return int(i.Score*100) - int(j.Score*100)
	})

	// Take top K and fetch full recipe data
	if limit > len(scoredRecipes) {
		limit = len(scoredRecipes)
	}

	var result []string
	for i := 0; i < limit; i++ {
		result = append(result, scoredRecipes[i].RecipeID)
	}

	return result, nil
}

// float32SliceToByteSlice converts a slice of float32 to a byte slice.
func float32SliceToByteSlice(floats []float32) ([]byte, error) {
	if len(floats) == 0 {
		return nil, nil
	}
	buf := make([]byte, 4*len(floats)) // 4 bytes per float32
	for i, f := range floats {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(f))
	}
	return buf, nil
}

// byteSliceToFloat32Slice converts a byte slice to a slice of float32.
func byteSliceToFloat32Slice(bytes []byte) ([]float32, error) {
	if len(bytes) == 0 {
		return nil, nil
	}
	if len(bytes)%4 != 0 {
		return nil, fmt.Errorf("byte slice length is not a multiple of 4")
	}
	floats := make([]float32, len(bytes)/4)
	for i := 0; i < len(bytes)/4; i++ {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(bytes[i*4 : (i+1)*4]))
	}
	return floats, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

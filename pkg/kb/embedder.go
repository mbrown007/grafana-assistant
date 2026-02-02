package kb

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Embedder wraps the OpenAI embeddings API.
type Embedder struct {
	client *openai.Client
	model  openai.EmbeddingModel
}

// NewEmbedder creates an embedder using the given API key and model name.
// Model should be an OpenAI embedding model name (e.g. "text-embedding-3-small").
func NewEmbedder(apiKey, model string) *Embedder {
	return &Embedder{
		client: openai.NewClient(apiKey),
		model:  openai.EmbeddingModel(model),
	}
}

// Embed returns one embedding per input text.
func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: e.model,
	})
	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	result := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		result[i] = d.Embedding
	}
	return result, nil
}

// EmbedSingle returns the embedding for a single text (convenience for query-time).
func (e *Embedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

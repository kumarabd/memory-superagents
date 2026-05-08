package ai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	c *openai.Client
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		panic("OPENAI_API_KEY must not be empty")
	}
	return &Client{c: openai.NewClient(apiKey)}
}

// Embed returns the text-embedding-ada-002 embedding for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.c.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}
	emb := resp.Data[0].Embedding
	result := make([]float32, len(emb))
	for i, v := range emb {
		result[i] = float32(v)
	}
	return result, nil
}

// EmbedBatch embeds multiple texts in a single API call.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.c.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, fmt.Errorf("batch embedding failed: %w", err)
	}
	results := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			results[i][j] = float32(v)
		}
	}
	return results, nil
}

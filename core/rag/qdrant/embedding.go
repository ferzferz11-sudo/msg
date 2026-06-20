package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"LavenderMessenger/core/rag"
)

// OpenAIEmbeddingService implements rag.EmbeddingService using OpenAI API.
// Uses text-embedding-3-small (1536 dim, $0.00002/1K tokens).
type OpenAIEmbeddingService struct {
	apiKey  string
	model   string
	dim     int
	http    *http.Client
}

func NewOpenAIEmbeddingService(dim int) *OpenAIEmbeddingService {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil
	}

	model := "text-embedding-3-small"
	if d := os.Getenv("EMBEDDING_MODEL"); d != "" {
		model = d
	}

	return &OpenAIEmbeddingService{
		apiKey: apiKey,
		model:  model,
		dim:    dim,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *OpenAIEmbeddingService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return s.embed(ctx, text)
}

func (s *OpenAIEmbeddingService) EmbedImage(ctx context.Context, imageData []byte) ([]float32, error) {
	return nil, fmt.Errorf("image embedding not supported by OpenAI text embeddings")
}

func (s *OpenAIEmbeddingService) EmbedMultimodal(ctx context.Context, text string, images [][]byte) (*rag.MultimodalEmbedding, error) {
	textVec, err := s.EmbedText(ctx, text)
	if err != nil {
		return nil, err
	}
	return &rag.MultimodalEmbedding{
		TextEmbedding:  textVec,
		JointEmbedding: textVec,
	}, nil
}

func (s *OpenAIEmbeddingService) embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]any{
		"model": s.model,
		"input": text,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embeddings %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return result.Data[0].Embedding, nil
}

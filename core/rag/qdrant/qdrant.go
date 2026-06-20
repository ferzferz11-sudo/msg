package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"LavenderMessenger/core/rag"
)

// Client implements rag.VectorSearch using Qdrant REST API.
// Falls back gracefully if Qdrant is unavailable.
type Client struct {
	baseURL    string
	collection string
	dim        int
	http       *http.Client
	mu         sync.RWMutex
	available  bool
}

// NewClient creates a Qdrant client. Returns nil if QDRANT_URL is not set.
func NewClient(collection string, dim int) *Client {
	addr := os.Getenv("QDRANT_URL")
	if addr == "" {
		return nil
	}

	c := &Client{
		baseURL:    addr,
		collection: collection,
		dim:        dim,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.ensureCollection(ctx); err != nil {
		log.Printf("[Qdrant] not available, RAG will use in-memory: %v", err)
		return c
	}

	c.mu.Lock()
	c.available = true
	c.mu.Unlock()
	log.Printf("[Qdrant] connected at %s, collection=%s", addr, collection)
	return c
}

func (c *Client) IsAvailable() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.available
}

func (c *Client) ensureCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     c.dim,
			"distance": "Cosine",
		},
	}
	return c.do(ctx, "PUT", url, body, nil)
}

func (c *Client) Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]rag.VectorResult, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("qdrant not available")
	}
	if topK <= 0 {
		topK = 5
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, c.collection)
	payload := map[string]any{
		"vector":       embedding,
		"limit":        topK,
		"with_payload": true,
	}
	if len(filters) > 0 {
		payload["filter"] = buildFilter(filters)
	}

	var result searchResponse
	if err := c.do(ctx, "POST", url, payload, &result); err != nil {
		return nil, err
	}

	out := make([]rag.VectorResult, 0, len(result.Result))
	for _, r := range result.Result {
		vr := rag.VectorResult{
			ID:    fmt.Sprintf("%v", r.ID),
			Score: r.Score,
		}
		if p, ok := r.Payload.(map[string]any); ok {
			vr.Metadata = p
			if content, ok := p["content"].(string); ok {
				vr.Content = content
			}
		}
		out = append(out, vr)
	}
	return out, nil
}

func (c *Client) Upsert(ctx context.Context, id string, embedding []float32, metadata map[string]any) error {
	if !c.IsAvailable() {
		return fmt.Errorf("qdrant not available")
	}

	url := fmt.Sprintf("%s/collections/%s/points", c.baseURL, c.collection)
	body := map[string]any{
		"points": []any{map[string]any{
			"id":      id,
			"vector":  embedding,
			"payload": metadata,
		}},
	}
	return c.do(ctx, "PUT", url, body, nil)
}

func (c *Client) Delete(ctx context.Context, id string) error {
	if !c.IsAvailable() {
		return fmt.Errorf("qdrant not available")
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", c.baseURL, c.collection)
	body := map[string]any{"points": []string{id}}
	return c.do(ctx, "POST", url, body, nil)
}

func (c *Client) do(ctx context.Context, method, url string, payload any, result any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant %s: %d %s", url, resp.StatusCode, string(body))
	}

	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(result)
	}
	return nil
}

func buildFilter(filters map[string]any) map[string]any {
	must := make([]any, 0, len(filters))
	for k, v := range filters {
		must = append(must, map[string]any{
			"key":   k,
			"match": map[string]any{"value": v},
		})
	}
	return map[string]any{"must": must}
}

type searchResponse struct {
	Result []searchResult `json:"result"`
}

type searchResult struct {
	ID      any     `json:"id"`
	Score   float32 `json:"score"`
	Payload any     `json:"payload"`
}

package main

// ai_provider_reve.go — Reve image generation provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type reveProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

func newReveProvider(config map[string]any, apiKey string) (AgentProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("REVE_API_KEY")
	}
	baseURL, _ := config["base_url"].(string)
	if baseURL == "" {
		baseURL = os.Getenv("REVE_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.reve.com/v1/image"
	}
	return &reveProvider{
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: baseURL,
	}, nil
}

func (p *reveProvider) StreamChat(ctx context.Context, messages []AIMessageInput, tools []ToolDefInput) (<-chan StreamChunk, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Reve API key not configured")
	}

	prompt := ""
	if len(messages) > 0 {
		prompt = messages[len(messages)-1].Content
	}
	if prompt == "" {
		return nil, fmt.Errorf("empty prompt")
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)

		imageURL, err := p.generateImage(ctx, prompt)
		if err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}

		ch <- StreamChunk{
			Content:   "Image generated",
			ImageURL:  imageURL,
			Finished:  true,
			Done:      true,
		}
	}()

	return ch, nil
}

func (p *reveProvider) generateImage(ctx context.Context, prompt string) (string, error) {
	payload := map[string]any{
		"prompt":              prompt,
		"test_time_scaling":   1,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Reve request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Reve returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Reve response: %w", err)
	}

	if result.Image == "" {
		return "", fmt.Errorf("Reve returned empty image")
	}

	imageBytes, err := base64.StdEncoding.DecodeString(result.Image)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	uploadedURL, err := p.uploadImage(ctx, imageBytes)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	return uploadedURL, nil
}

func (p *reveProvider) uploadImage(ctx context.Context, imageBytes []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("image", "reve.png")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", err
	}
	writer.Close()

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}
	uploadURL := fmt.Sprintf("http://localhost:%s/upload-image", httpPort)

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer system")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload returned %d: %s", resp.StatusCode, string(body))
	}

	var uploadResult struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResult); err != nil {
		return "", err
	}

	return uploadResult.URL, nil
}

func (p *reveProvider) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		SupportsImages:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
		MaxTokens:         0,
	}
}

func (p *reveProvider) HealthCheck(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("Reve API key not configured")
	}
	return nil
}

func (p *reveProvider) Close() error { return nil }

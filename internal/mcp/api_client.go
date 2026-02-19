package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type kaizenAPIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newKaizenAPIClient() *kaizenAPIClient {
	baseURL := strings.TrimRight(getEnv("KAIZEN_API_BASE_URL", "http://localhost:8080"), "/")
	return &kaizenAPIClient{
		baseURL: baseURL,
		apiKey:  os.Getenv("KAIZEN_API_KEY"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *kaizenAPIClient) call(ctx context.Context, method, path string, payload interface{}) (map[string]interface{}, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("KAIZEN_API_KEY is not set")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request payload: %w", err)
		}
		body = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", serverName, serverVersion))
	if payload != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var decoded map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	} else {
		decoded = map[string]interface{}{}
	}

	if resp.StatusCode >= 400 {
		msg := "Kaizen API request failed"
		if v, ok := decoded["error"].(string); ok && v != "" {
			msg = v
		}
		return nil, fmt.Errorf("%s (status=%d)", msg, resp.StatusCode)
	}

	return decoded, nil
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

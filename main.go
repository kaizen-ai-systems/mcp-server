// Package main provides a minimal MCP server for Kaizen tools over stdio.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	mcpServerName    = "kaizen-mcp"
	mcpServerVersion = "1.0.0"
	mcpProtocol      = "2024-11-05"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type toolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type toolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

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
	req.Header.Set("Content-Type", "application/json")

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

type mcpServer struct {
	reader *bufio.Reader
	writer *bufio.Writer
	logger *slog.Logger
	client *kaizenAPIClient
}

func newMCPServer() *mcpServer {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &mcpServer{
		reader: bufio.NewReader(os.Stdin),
		writer: bufio.NewWriter(os.Stdout),
		logger: logger,
		client: newKaizenAPIClient(),
	}
}

func (s *mcpServer) serve() error {
	for {
		payload, err := readMessage(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to read message: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			s.logger.Warn("dropping invalid json-rpc payload", "error", err)
			continue
		}

		if req.Method == "notifications/initialized" || req.Method == "initialized" {
			continue
		}

		var (
			result interface{}
			rpcErr *jsonRPCError
		)

		switch req.Method {
		case "initialize":
			result = map[string]interface{}{
				"protocolVersion": mcpProtocol,
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]string{
					"name":    mcpServerName,
					"version": mcpServerVersion,
				},
			}
		case "ping":
			result = map[string]interface{}{}
		case "tools/list":
			result = map[string]interface{}{"tools": toolDefinitions()}
		case "tools/call":
			result, rpcErr = s.handleToolCall(req.Params)
		default:
			rpcErr = &jsonRPCError{
				Code:    -32601,
				Message: "method not found",
				Data:    req.Method,
			}
		}

		if len(req.ID) == 0 {
			continue
		}

		var id interface{}
		if err := json.Unmarshal(req.ID, &id); err != nil {
			id = string(req.ID)
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  result,
			Error:   rpcErr,
		}
		if err := writeMessage(s.writer, resp); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}
}

// MCP clients use Content-Length framing over stdio, but we also accept
// line-delimited JSON for local smoke tests.
func readMessage(reader *bufio.Reader) ([]byte, error) {
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			trimmed := strings.TrimSpace(firstLine)
			if trimmed == "" {
				return nil, io.EOF
			}
			if strings.HasPrefix(trimmed, "{") {
				return []byte(trimmed), nil
			}
		}
		return nil, err
	}

	trimmed := strings.TrimSpace(firstLine)
	if trimmed == "" {
		return nil, fmt.Errorf("received empty message")
	}
	if strings.HasPrefix(trimmed, "{") {
		return []byte(trimmed), nil
	}

	headers := []string{strings.TrimRight(firstLine, "\r\n")}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		clean := strings.TrimRight(line, "\r\n")
		if clean == "" {
			break
		}
		headers = append(headers, clean)
	}

	length, err := parseContentLength(headers)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}
	return payload, nil
}

func parseContentLength(headers []string) (int, error) {
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			continue
		}
		rawLen := strings.TrimSpace(parts[1])
		length, err := strconv.Atoi(rawLen)
		if err != nil || length <= 0 {
			return 0, fmt.Errorf("invalid Content-Length value: %q", rawLen)
		}
		return length, nil
	}
	return 0, fmt.Errorf("missing Content-Length header")
}

func writeMessage(writer *bufio.Writer, response jsonRPCResponse) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return writer.Flush()
}

func (s *mcpServer) handleToolCall(raw json.RawMessage) (interface{}, *jsonRPCError) {
	var params toolsCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, &jsonRPCError{
			Code:    -32602,
			Message: "invalid tool call params",
			Data:    err.Error(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var (
		data map[string]interface{}
		err  error
	)

	switch params.Name {
	case "akuma.query":
		data, err = s.callAkumaQuery(ctx, params.Arguments)
	case "akuma.explain":
		data, err = s.callAkumaExplain(ctx, params.Arguments)
	case "enzan.summary":
		data, err = s.callEnzanSummary(ctx, params.Arguments)
	case "enzan.burn":
		data, err = s.client.call(ctx, http.MethodGet, "/v1/enzan/burn", nil)
	case "sozo.generate":
		data, err = s.callSozoGenerate(ctx, params.Arguments)
	case "sozo.schemas":
		data, err = s.client.call(ctx, http.MethodGet, "/v1/sozo/schemas", nil)
	default:
		return nil, &jsonRPCError{
			Code:    -32602,
			Message: "unknown tool",
			Data:    params.Name,
		}
	}

	if err != nil {
		return map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": err.Error()},
			},
			"isError": true,
		}, nil
	}

	pretty, _ := json.MarshalIndent(data, "", "  ")
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(pretty)},
		},
		"structuredContent": data,
	}, nil
}

func (s *mcpServer) callAkumaQuery(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	dialect, _ := args["dialect"].(string)
	prompt, _ := args["prompt"].(string)
	if strings.TrimSpace(dialect) == "" {
		return nil, fmt.Errorf("dialect is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	payload := map[string]interface{}{
		"dialect": dialect,
		"prompt":  prompt,
	}
	if v, ok := args["mode"]; ok {
		payload["mode"] = v
	}
	if v, ok := args["maxRows"]; ok {
		payload["maxRows"] = v
	}
	if v, ok := args["guardrails"]; ok {
		payload["guardrails"] = v
	}

	return s.client.call(ctx, http.MethodPost, "/v1/akuma/query", payload)
}

func (s *mcpServer) callAkumaExplain(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	sql, _ := args["sql"].(string)
	if strings.TrimSpace(sql) == "" {
		return nil, fmt.Errorf("sql is required")
	}
	return s.client.call(ctx, http.MethodPost, "/v1/akuma/explain", map[string]interface{}{"sql": sql})
}

func (s *mcpServer) callEnzanSummary(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"window": "24h",
	}
	if v, ok := args["window"]; ok {
		payload["window"] = v
	}
	if v, ok := args["groupBy"]; ok {
		payload["groupBy"] = v
	}
	return s.client.call(ctx, http.MethodPost, "/v1/enzan/summary", payload)
}

func (s *mcpServer) callSozoGenerate(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := args["records"]; !ok {
		return nil, fmt.Errorf("records is required")
	}
	if _, hasSchema := args["schema"]; !hasSchema {
		if _, hasSchemaName := args["schemaName"]; !hasSchemaName {
			return nil, fmt.Errorf("schema or schemaName is required")
		}
	}

	payload := map[string]interface{}{
		"records": args["records"],
	}
	for _, key := range []string{"schema", "schemaName", "correlations", "seed"} {
		if v, ok := args[key]; ok {
			payload[key] = v
		}
	}
	return s.client.call(ctx, http.MethodPost, "/v1/sozo/generate", payload)
}

func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "akuma.query",
			Description: "Translate natural language into SQL (optionally returning rows or explanation).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dialect":    map[string]interface{}{"type": "string", "enum": []string{"postgres", "mysql", "snowflake", "bigquery"}},
					"prompt":     map[string]interface{}{"type": "string"},
					"mode":       map[string]interface{}{"type": "string", "enum": []string{"sql-only", "sql-and-results", "explain"}},
					"maxRows":    map[string]interface{}{"type": "number"},
					"guardrails": map[string]interface{}{"type": "object"},
				},
				"required":             []string{"dialect", "prompt"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "akuma.explain",
			Description: "Explain a SQL query in plain English.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"sql"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.summary",
			Description: "Summarize GPU spend and usage for a time window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window":  map[string]interface{}{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}},
					"groupBy": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.burn",
			Description: "Get current burn rate in USD/hour.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "sozo.generate",
			Description: "Generate synthetic tabular data from a schema or named preset.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"records":      map[string]interface{}{"type": "number"},
					"schemaName":   map[string]interface{}{"type": "string"},
					"schema":       map[string]interface{}{"type": "object"},
					"correlations": map[string]interface{}{"type": "object"},
					"seed":         map[string]interface{}{"type": "number"},
				},
				"required": []string{"records"},
			},
		},
		{
			Name:        "sozo.schemas",
			Description: "List built-in Sozo schema presets.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
	}
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func main() {
	server := newMCPServer()
	server.logger.Info("starting mcp server", "name", mcpServerName, "api_base_url", server.client.baseURL)
	if err := server.serve(); err != nil {
		server.logger.Error("mcp server stopped with error", "error", err)
		os.Exit(1)
	}
}

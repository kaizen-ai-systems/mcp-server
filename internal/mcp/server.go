package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Server struct {
	reader *bufio.Reader
	writer *bufio.Writer
	logger *slog.Logger
	client *kaizenAPIClient
}

func NewServer() *Server {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &Server{
		reader: bufio.NewReader(os.Stdin),
		writer: bufio.NewWriter(os.Stdout),
		logger: logger,
		client: newKaizenAPIClient(),
	}
}

func (s *Server) Serve() error {
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
				"protocolVersion": protocol,
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]string{
					"name":    serverName,
					"version": serverVersion,
				},
			}
		case "ping":
			result = map[string]interface{}{}
		case "tools/list":
			result = map[string]interface{}{"tools": toolDefinitions()}
		case "tools/call":
			result, rpcErr = s.handleToolCall(req.Params)
		default:
			rpcErr = &jsonRPCError{Code: -32601, Message: "method not found", Data: req.Method}
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

func (s *Server) handleToolCall(raw json.RawMessage) (interface{}, *jsonRPCError) {
	var params toolsCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid tool call params", Data: err.Error()}
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
	case "akuma.schema":
		data, err = s.callAkumaSchema(ctx, params.Arguments)
	case "enzan.summary":
		data, err = s.callEnzanSummary(ctx, params.Arguments)
	case "enzan.burn":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/burn", nil)
	case "sozo.generate":
		data, err = s.callSozoGenerate(ctx, params.Arguments)
	case "sozo.schemas":
		data, err = s.client.call(ctx, "GET", "/v1/sozo/schemas", nil)
	default:
		return nil, &jsonRPCError{Code: -32602, Message: "unknown tool", Data: params.Name}
	}

	if err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}

	pretty, _ := json.MarshalIndent(data, "", "  ")
	return map[string]interface{}{
		"content":           []map[string]string{{"type": "text", "text": string(pretty)}},
		"structuredContent": data,
	}, nil
}

func (s *Server) callAkumaQuery(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
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

	return s.client.call(ctx, "POST", "/v1/akuma/query", payload)
}

func (s *Server) callAkumaExplain(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	sql, _ := args["sql"].(string)
	if strings.TrimSpace(sql) == "" {
		return nil, fmt.Errorf("sql is required")
	}
	return s.client.call(ctx, "POST", "/v1/akuma/explain", map[string]interface{}{"sql": sql})
}

func (s *Server) callAkumaSchema(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	tables, ok := args["tables"]
	if !ok {
		return nil, fmt.Errorf("tables is required")
	}

	payload := map[string]interface{}{
		"tables": tables,
	}
	if version, ok := args["version"]; ok {
		payload["version"] = version
	}
	return s.client.call(ctx, "POST", "/v1/akuma/schema", payload)
}

func (s *Server) callEnzanSummary(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"window": "24h",
	}
	if v, ok := args["window"]; ok {
		payload["window"] = v
	}
	if v, ok := args["groupBy"]; ok {
		payload["groupBy"] = v
	}
	return s.client.call(ctx, "POST", "/v1/enzan/summary", payload)
}

func (s *Server) callSozoGenerate(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
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
	return s.client.call(ctx, "POST", "/v1/sozo/generate", payload)
}

func (s *Server) LogStartup() {
	s.logger.Info("starting mcp server", "name", serverName, "api_base_url", s.client.baseURL)
}

func (s *Server) LogFatal(err error) {
	s.logger.Error("mcp server stopped with error", "error", err)
}

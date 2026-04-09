package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
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
	case "enzan.costs_by_model":
		data, err = s.callEnzanCostsByModel(ctx, params.Arguments)
	case "enzan.pricing_models":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/pricing/models", nil)
	case "enzan.set_model_pricing":
		data, err = s.callEnzanSetModelPricing(ctx, params.Arguments)
	case "enzan.pricing_gpus":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/pricing/gpus", nil)
	case "enzan.set_gpu_pricing":
		data, err = s.callEnzanSetGPUPricing(ctx, params.Arguments)
	case "enzan.optimize":
		data, err = s.callEnzanOptimize(ctx, params.Arguments)
	case "enzan.alerts":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/alerts", nil)
	case "enzan.create_alert":
		data, err = s.callEnzanCreateAlert(ctx, params.Arguments)
	case "enzan.update_alert":
		data, err = s.callEnzanUpdateAlert(ctx, params.Arguments)
	case "enzan.delete_alert":
		data, err = s.callEnzanDeleteAlert(ctx, params.Arguments)
	case "enzan.alert_events":
		data, err = s.callEnzanAlertEvents(ctx, params.Arguments)
	case "enzan.alert_deliveries":
		data, err = s.callEnzanAlertDeliveries(ctx, params.Arguments)
	case "enzan.alert_endpoints":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/alerts/endpoints", nil)
	case "enzan.create_alert_endpoint":
		data, err = s.callEnzanCreateAlertEndpoint(ctx, params.Arguments)
	case "enzan.update_alert_endpoint":
		data, err = s.callEnzanUpdateAlertEndpoint(ctx, params.Arguments)
	case "enzan.delete_alert_endpoint":
		data, err = s.callEnzanDeleteAlertEndpoint(ctx, params.Arguments)
	case "enzan.chat":
		data, err = s.callEnzanChat(ctx, params.Arguments)
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
	if v, ok := args["sourceId"]; ok {
		payload["sourceId"] = v
	}
	if v, ok := args["guardrails"]; ok {
		payload["guardrails"] = v
	}

	return s.client.call(ctx, "POST", "/v1/akuma/query", payload)
}

func (s *Server) callEnzanCreateAlertEndpoint(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	targetURL, _ := args["targetUrl"].(string)
	if strings.TrimSpace(targetURL) == "" {
		return nil, fmt.Errorf("targetUrl is required")
	}
	payload := map[string]interface{}{
		"targetUrl": targetURL,
	}
	if signingSecret, ok := args["signingSecret"]; ok {
		payload["signingSecret"] = signingSecret
	}
	return s.client.call(ctx, "POST", "/v1/enzan/alerts/endpoints", payload)
}

func (s *Server) callEnzanCreateAlert(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["name"].(string)
	alertType, _ := args["type"].(string)
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(alertType) == "" {
		return nil, fmt.Errorf("type is required")
	}
	payload := map[string]interface{}{
		"name": name,
		"type": alertType,
	}
	if id, ok := args["id"]; ok {
		payload["id"] = id
	}
	if threshold, ok := args["threshold"]; ok {
		payload["threshold"] = threshold
	}
	if window, ok := args["window"]; ok {
		payload["window"] = window
	}
	if labels, ok := args["labels"]; ok {
		payload["labels"] = labels
	}
	if enabled, ok := args["enabled"]; ok {
		payload["enabled"] = enabled
	}
	return s.client.call(ctx, "POST", "/v1/enzan/alerts", payload)
}

func (s *Server) callEnzanUpdateAlert(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id is required")
	}
	payload := map[string]interface{}{}
	if name, ok := args["name"]; ok {
		payload["name"] = name
	}
	if threshold, ok := args["threshold"]; ok {
		payload["threshold"] = threshold
	}
	if window, ok := args["window"]; ok {
		payload["window"] = window
	}
	if labels, ok := args["labels"]; ok {
		payload["labels"] = labels
	}
	if enabled, ok := args["enabled"]; ok {
		payload["enabled"] = enabled
	}
	return s.client.call(ctx, "PATCH", "/v1/enzan/alerts/"+url.PathEscape(id), payload)
}

func (s *Server) callEnzanAlertEvents(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	path := "/v1/enzan/alerts/events"
	if limit, ok := numericToolArg(args, "limit"); ok && limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	return s.client.call(ctx, "GET", path, nil)
}

func (s *Server) callEnzanAlertDeliveries(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	path := "/v1/enzan/alerts/deliveries"
	if limit, ok := numericToolArg(args, "limit"); ok && limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	return s.client.call(ctx, "GET", path, nil)
}

func (s *Server) callEnzanDeleteAlert(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.client.call(ctx, "DELETE", "/v1/enzan/alerts/"+url.PathEscape(id), nil)
}

func (s *Server) callEnzanUpdateAlertEndpoint(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id is required")
	}
	payload := map[string]interface{}{}
	if targetURL, ok := args["targetUrl"]; ok {
		payload["targetUrl"] = targetURL
	}
	if signingSecret, ok := args["signingSecret"]; ok {
		payload["signingSecret"] = signingSecret
	}
	if enabled, ok := args["enabled"]; ok {
		payload["enabled"] = enabled
	}
	return s.client.call(ctx, "PATCH", "/v1/enzan/alerts/endpoints/"+url.PathEscape(id), payload)
}

func (s *Server) callEnzanDeleteAlertEndpoint(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.client.call(ctx, "DELETE", "/v1/enzan/alerts/endpoints/"+url.PathEscape(id), nil)
}

func numericToolArg(args map[string]interface{}, key string) (int, bool) {
	switch value := args[key].(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	case int64:
		return int(value), true
	default:
		return 0, false
	}
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
	dialect, _ := args["dialect"].(string)
	if strings.TrimSpace(dialect) == "" {
		return nil, fmt.Errorf("dialect is required")
	}

	payload := map[string]interface{}{
		"dialect": dialect,
		"tables":  tables,
	}
	if sourceID, ok := args["sourceId"]; ok {
		payload["sourceId"] = sourceID
	}
	if name, ok := args["name"]; ok {
		payload["name"] = name
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

func (s *Server) callEnzanCostsByModel(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"window": "30d",
	}
	if v, ok := args["window"]; ok {
		payload["window"] = v
	}
	return s.client.call(ctx, "POST", "/v1/enzan/costs/by-model", payload)
}

func (s *Server) callEnzanOptimize(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"window": "30d",
	}
	if v, ok := args["window"]; ok {
		payload["window"] = v
	}
	return s.client.call(ctx, "POST", "/v1/enzan/optimize", payload)
}

func (s *Server) callEnzanChat(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	if v, ok := args["message"]; ok {
		payload["message"] = v
	}
	if v, ok := args["conversationId"]; ok {
		payload["conversationId"] = v
	}
	if v, ok := args["window"]; ok {
		payload["window"] = v
	}
	return s.client.call(ctx, "POST", "/v1/enzan/chat", payload)
}

func (s *Server) callEnzanSetModelPricing(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	provider, _ := args["provider"].(string)
	model, _ := args["model"].(string)
	if strings.TrimSpace(provider) == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if _, ok := args["input_cost_per_1k_tokens_usd"]; !ok {
		return nil, fmt.Errorf("input_cost_per_1k_tokens_usd is required")
	}
	if _, ok := args["output_cost_per_1k_tokens_usd"]; !ok {
		return nil, fmt.Errorf("output_cost_per_1k_tokens_usd is required")
	}
	payload := map[string]interface{}{
		"provider":                      provider,
		"model":                         model,
		"input_cost_per_1k_tokens_usd":  args["input_cost_per_1k_tokens_usd"],
		"output_cost_per_1k_tokens_usd": args["output_cost_per_1k_tokens_usd"],
	}
	for _, key := range []string{"display_name", "currency", "active"} {
		if v, ok := args[key]; ok {
			payload[key] = v
		}
	}
	return s.client.call(ctx, "POST", "/v1/enzan/pricing/models", payload)
}

func (s *Server) callEnzanSetGPUPricing(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	provider, _ := args["provider"].(string)
	gpuType, _ := args["gpu_type"].(string)
	if strings.TrimSpace(provider) == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(gpuType) == "" {
		return nil, fmt.Errorf("gpu_type is required")
	}
	if _, ok := args["hourly_rate_usd"]; !ok {
		return nil, fmt.Errorf("hourly_rate_usd is required")
	}
	payload := map[string]interface{}{
		"provider":        provider,
		"gpu_type":        gpuType,
		"hourly_rate_usd": args["hourly_rate_usd"],
	}
	for _, key := range []string{"display_name", "currency", "active"} {
		if v, ok := args[key]; ok {
			payload[key] = v
		}
	}
	return s.client.call(ctx, "POST", "/v1/enzan/pricing/gpus", payload)
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

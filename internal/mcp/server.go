package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	case "akuma.query_interactive":
		data, err = s.callAkumaQueryInteractive(ctx, params.Arguments)
	case "akuma.explain":
		data, err = s.callAkumaExplain(ctx, params.Arguments)
	case "akuma.schema":
		data, err = s.callAkumaSchema(ctx, params.Arguments)
	case "enzan.summary":
		data, err = s.callEnzanSummary(ctx, params.Arguments)
	case "enzan.costs_by_model":
		data, err = s.callEnzanCostsByModel(ctx, params.Arguments)
	case "enzan.routing":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/routing", nil)
	case "enzan.set_routing":
		data, err = s.callEnzanSetRouting(ctx, params.Arguments)
	case "enzan.routing_savings":
		data, err = s.callEnzanRoutingSavings(ctx, params.Arguments)
	case "enzan.pricing_models":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/pricing/models", nil)
	case "enzan.set_model_pricing":
		data, err = s.callEnzanSetModelPricing(ctx, params.Arguments)
	case "enzan.pricing_gpus":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/pricing/gpus", nil)
	case "enzan.set_gpu_pricing":
		data, err = s.callEnzanSetGPUPricing(ctx, params.Arguments)
	case "enzan.pricing_refresh_trigger":
		// Preserve 429 {status:"dropped",triggeredBy:...} body so MCP
		// callers can branch on the typed shape, matching the SDK contract.
		data, err = s.callPreservingTypedBody(ctx, "POST", "/v1/enzan/pricing/refresh", nil, []int{http.StatusTooManyRequests})
	case "enzan.pricing_refresh_log":
		data, err = s.callEnzanPricingRefreshLog(ctx, params.Arguments)
	case "enzan.pricing_providers":
		data, err = s.client.call(ctx, "GET", "/v1/enzan/pricing/providers", nil)
	case "enzan.pricing_offers_upsert":
		data, err = s.callEnzanPricingOffersUpsert(ctx, params.Arguments)
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
		// typedBodyError carries a meaningful response body alongside a
		// transport failure status or semantic failure state. Thread BOTH
		// signals: isError=true so generic MCP clients see the failure,
		// AND structuredContent with the typed body so callers that want
		// to branch on the body shape can read it directly.
		var typedErr *typedBodyError
		if errors.As(err, &typedErr) {
			pretty, _ := json.MarshalIndent(typedErr.Body, "", "  ")
			return map[string]interface{}{
				"content":           []map[string]string{{"type": "text", "text": fmt.Sprintf("%s:\n%s", typedErr.Error(), pretty)}},
				"structuredContent": typedErr.Body,
				"isError":           true,
			}, nil
		}
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
	payload, err := buildAkumaQueryPayload(args)
	if err != nil {
		return nil, err
	}
	return s.client.call(ctx, http.MethodPost, "/v1/akuma/query", payload)
}

func (s *Server) callAkumaQueryInteractive(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	payload, err := buildAkumaQueryPayload(args)
	if err != nil {
		return nil, err
	}
	// Interactive keeps its own call path because it preserves typed non-2xx
	// bodies and converts HTTP 200 non-completed envelopes into MCP tool errors.
	data, err := s.callPreservingTypedBody(ctx, http.MethodPost, "/v1/akuma/queries/interactive", payload, []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusMethodNotAllowed,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusInternalServerError,
	})
	if err != nil {
		return nil, err
	}
	if err := validateAkumaInteractiveResponse(data); err != nil {
		return nil, err
	}
	if data["status"] != "completed" {
		return nil, &typedBodyError{Status: http.StatusOK, Body: data, Msg: fmt.Sprintf("interactive query %s", data["status"])}
	}
	return data, nil
}

func buildAkumaQueryPayload(args map[string]interface{}) (map[string]interface{}, error) {
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

	return payload, nil
}

func validateAkumaInteractiveResponse(data map[string]interface{}) error {
	status, ok := data["status"].(string)
	if !ok || strings.TrimSpace(status) == "" {
		return fmt.Errorf("interactive query response missing status")
	}

	result, hasResult := data["result"]
	var resultMap map[string]interface{}
	if hasResult {
		var ok bool
		resultMap, ok = result.(map[string]interface{})
		if !ok {
			return fmt.Errorf("interactive query response result must be an object")
		}
	}
	if (status == "completed" || status == "rejected") && !hasResult {
		return fmt.Errorf("interactive query response missing result")
	}
	if status == "rejected" {
		errorText, _ := resultMap["error"].(string)
		if strings.TrimSpace(errorText) == "" {
			return fmt.Errorf("interactive query rejected response missing error")
		}
	}
	if status == "completed" {
		errorText, _ := resultMap["error"].(string)
		if strings.TrimSpace(errorText) != "" {
			return fmt.Errorf("interactive query completed response must not include error")
		}
	}

	return nil
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

func (s *Server) callEnzanSetRouting(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	enabled, ok := args["enabled"]
	if !ok {
		return nil, fmt.Errorf("enabled is required")
	}
	payload := map[string]interface{}{
		"enabled": enabled,
	}
	if value, ok := args["simple_model"]; ok {
		payload["simple_model"] = value
	}
	if value, ok := args["moderate_model"]; ok {
		payload["moderate_model"] = value
	}
	if value, ok := args["complex_model"]; ok {
		payload["complex_model"] = value
	}
	return s.client.call(ctx, "POST", "/v1/enzan/routing", payload)
}

func (s *Server) callEnzanRoutingSavings(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	path := "/v1/enzan/routing/savings"
	if window, ok := args["window"].(string); ok && strings.TrimSpace(window) != "" {
		path += "?window=" + url.QueryEscape(window)
	}
	return s.client.call(ctx, "GET", path, nil)
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

// callPreservingTypedBody runs an api call. For the listed status codes
// (429 dropped / 409 stale on the 8.2-public surface), it returns the
// typed response body wrapped in a *typedBodyError so handleToolCall can
// surface BOTH (a) `isError: true` — generic MCP clients that branch
// only on tool failure correctly see a dropped/stale outcome — AND
// (b) the typed body in `structuredContent` for callers that want to
// branch on the body shape. Matches the SDK contract that exposes the
// same bodies via err.data.
func (s *Server) callPreservingTypedBody(ctx context.Context, method, path string, payload interface{}, preserveStatuses []int) (map[string]interface{}, error) {
	data, err := s.client.call(ctx, method, path, payload)
	if err != nil {
		var apiErr *apiCallError
		if errors.As(err, &apiErr) {
			for _, code := range preserveStatuses {
				if apiErr.Status == code {
					return nil, &typedBodyError{Status: apiErr.Status, Body: apiErr.Body, Msg: apiErr.Msg}
				}
			}
		}
	}
	return data, err
}

// typedBodyError signals that the underlying API call failed or produced a
// semantic failure envelope AND the body should be surfaced to the MCP client
// as structuredContent alongside `isError: true`. handleToolCall recognises
// this concrete type and threads both signals into the tool result.
type typedBodyError struct {
	Status int
	Body   map[string]interface{}
	Msg    string
}

func (e *typedBodyError) Error() string { return e.Msg }

func (s *Server) callEnzanPricingRefreshLog(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	path := "/v1/enzan/pricing/refresh/log"
	// Forward `limit` verbatim (including 0 and negative) so the server's
	// "limit must be a positive integer" 400 path stays observable from
	// MCP. Matches the SDK contract — server is the clamp/validation
	// authority, clients must not silently drop user-provided values.
	if limit, ok := numericToolArg(args, "limit"); ok {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	return s.client.call(ctx, "GET", path, nil)
}

func (s *Server) callEnzanPricingOffersUpsert(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	// Resolve each branch into one of three states: absent (key missing or
	// null), valid object, or present-but-malformed. Both "valid" and
	// "present-but-malformed" count as "branch selected" for the
	// exactly-one check — otherwise `{"gpu": valid, "llm": "oops"}` would
	// silently slip through as a GPU-only call. Malformed branches still
	// fail the request at the tool boundary, even when the other branch
	// looks valid.
	gpu, gpuState := classifyOfferBranch(args, "gpu")
	llm, llmState := classifyOfferBranch(args, "llm")
	gpuPresent := gpuState != offerBranchAbsent
	llmPresent := llmState != offerBranchAbsent
	if gpuPresent == llmPresent {
		return nil, fmt.Errorf("exactly one of gpu or llm must be set")
	}
	if gpuPresent && gpuState == offerBranchMalformed {
		return nil, fmt.Errorf("gpu must be an object")
	}
	if llmPresent && llmState == offerBranchMalformed {
		return nil, fmt.Errorf("llm must be an object")
	}
	payload := map[string]interface{}{}
	if gpuPresent {
		payload["gpu"] = gpu
	} else {
		payload["llm"] = llm
	}
	// Preserve 409 {status:"stale"} body so MCP callers can branch on the
	// typed stale shape, matching the SDK contract.
	return s.callPreservingTypedBody(ctx, "POST", "/v1/enzan/pricing/offers", payload, []int{http.StatusConflict})
}

type offerBranchState int

const (
	offerBranchAbsent offerBranchState = iota
	offerBranchValid
	offerBranchMalformed
)

// classifyOfferBranch returns (object, state) for an offer branch under
// `args[key]`. State:
//   - absent: key missing or null
//   - valid: key is a JSON object
//   - malformed: key is present but not null and not an object (e.g.,
//     a number or string). The exactly-one-of check counts both "valid"
//     and "malformed" as "present" so a caller cannot smuggle a second
//     branch as a non-object value.
func classifyOfferBranch(args map[string]interface{}, key string) (map[string]interface{}, offerBranchState) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, offerBranchAbsent
	}
	asMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, offerBranchMalformed
	}
	return asMap, offerBranchValid
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

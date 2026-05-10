package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolDefinitionsIncludesAkumaSchema(t *testing.T) {
	tools := toolDefinitions()
	found := false
	for _, tool := range tools {
		if tool.Name == "akuma.schema" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected akuma.schema tool in tools/list response")
	}
}

func TestToolDefinitionsIncludesAkumaQueryInteractive(t *testing.T) {
	tools := toolDefinitions()
	found := false
	for _, tool := range tools {
		if tool.Name == "akuma.query_interactive" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected akuma.query_interactive tool in tools/list response")
	}
}

func TestToolDefinitionsIncludesEnzanCostsByModel(t *testing.T) {
	tools := toolDefinitions()
	found := false
	for _, tool := range tools {
		if tool.Name == "enzan.costs_by_model" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected enzan.costs_by_model tool in tools/list response")
	}
}

func TestToolDefinitionsIncludeEnzanPricingTools(t *testing.T) {
	tools := toolDefinitions()
	required := map[string]bool{
		"enzan.pricing_models":          false,
		"enzan.set_model_pricing":       false,
		"enzan.pricing_gpus":            false,
		"enzan.set_gpu_pricing":         false,
		"enzan.pricing_refresh_trigger": false,
		"enzan.pricing_refresh_log":     false,
		"enzan.pricing_providers":       false,
		"enzan.pricing_offers_upsert":   false,
		"enzan.routing":                 false,
		"enzan.set_routing":             false,
		"enzan.routing_savings":         false,
		"enzan.alerts":                  false,
		"enzan.create_alert":            false,
		"enzan.update_alert":            false,
		"enzan.delete_alert":            false,
		"enzan.alert_events":            false,
		"enzan.alert_deliveries":        false,
		"enzan.alert_endpoints":         false,
		"enzan.create_alert_endpoint":   false,
		"enzan.update_alert_endpoint":   false,
		"enzan.delete_alert_endpoint":   false,
	}
	for _, tool := range tools {
		if _, ok := required[tool.Name]; ok {
			required[tool.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("expected %s tool in tools/list response", name)
		}
	}
}

func TestHandleToolCallUnknownTool(t *testing.T) {
	s := &Server{}
	raw, err := json.Marshal(toolsCallParams{
		Name:      "nope.tool",
		Arguments: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}
	if rpcErr == nil {
		t.Fatalf("expected rpc error")
	}
	if rpcErr.Code != -32602 {
		t.Fatalf("expected -32602, got %d", rpcErr.Code)
	}
}

func TestHandleToolCallMissingPromptReturnsToolError(t *testing.T) {
	s := &Server{}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}

	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected isError=true, got %#v", response["isError"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveDispatchesToInteractiveEndpoint(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/akuma/queries/interactive" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		if payload["sourceId"] != "src_123" {
			t.Errorf("expected sourceId to round-trip, got %#v", payload["sourceId"])
			http.Error(w, "bad source id", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"status":"completed","result":{"sql":"select 1"}}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect":  "postgres",
			"prompt":   "show one row",
			"sourceId": "src_123",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	content, ok := response["structuredContent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content, got %#v", response["structuredContent"])
	}
	if content["status"] != "completed" {
		t.Fatalf("unexpected status: %#v", content["status"])
	}
	resultContent, ok := content["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested result content, got %#v", content["result"])
	}
	if resultContent["sql"] != "select 1" {
		t.Fatalf("unexpected sql: %#v", resultContent["sql"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveRejectedEnvelope(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/akuma/queries/interactive" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"rejected","result":{"sql":"select * from users","error":"unsafe query"}}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "unsafe query",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	content, ok := response["structuredContent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content, got %#v", response["structuredContent"])
	}
	if content["status"] != "rejected" {
		t.Fatalf("unexpected status: %#v", content["status"])
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected rejected envelope to set isError=true, got %#v", response["isError"])
	}
	textContent, ok := response["content"].([]map[string]string)
	if !ok || len(textContent) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if !strings.HasPrefix(textContent[0]["text"], "interactive query rejected:\n") {
		t.Fatalf("expected semantic error text, got %#v", textContent[0]["text"])
	}
	resultContent, ok := content["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested result content, got %#v", content["result"])
	}
	if resultContent["error"] != "unsafe query" {
		t.Fatalf("unexpected error: %#v", resultContent["error"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveRejectsRejectedEnvelopeWithoutError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"rejected","result":{"sql":"select * from users"}}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "unsafe query",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected isError=true, got %#v", response["isError"])
	}
	content, ok := response["content"].([]map[string]string)
	if !ok || len(content) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if content[0]["text"] != "interactive query rejected response missing error" {
		t.Fatalf("unexpected tool error: %#v", content[0]["text"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveRejectsCompletedEnvelopeWithError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"completed","result":{"sql":"select *","error":"unsafe query"}}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "unsafe query",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected isError=true, got %#v", response["isError"])
	}
	content, ok := response["content"].([]map[string]string)
	if !ok || len(content) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if content[0]["text"] != "interactive query completed response must not include error" {
		t.Fatalf("unexpected tool error: %#v", content[0]["text"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveMalformedEnvelopeReturnsToolError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"completed"}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "show one row",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected isError=true, got %#v", response["isError"])
	}
	content, ok := response["content"].([]map[string]string)
	if !ok || len(content) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if content[0]["text"] != "interactive query response missing result" {
		t.Fatalf("unexpected tool error: %#v", content[0]["text"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveRejectsNullResult(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"needs_clarification","result":null}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "show one row",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected isError=true, got %#v", response["isError"])
	}
	content, ok := response["content"].([]map[string]string)
	if !ok || len(content) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if content[0]["text"] != "interactive query response result must be an object" {
		t.Fatalf("unexpected tool error: %#v", content[0]["text"])
	}
}

func TestHandleToolCallAkumaQueryInteractiveAllowsFutureStatusWithoutResult(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"needs_clarification"}`))
	}))
	defer api.Close()

	s := &Server{client: &kaizenAPIClient{
		baseURL:    api.URL,
		apiKey:     "test",
		httpClient: api.Client(),
	}}
	raw, err := json.Marshal(toolsCallParams{
		Name: "akuma.query_interactive",
		Arguments: map[string]interface{}{
			"dialect": "postgres",
			"prompt":  "show one row",
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	result, rpcErr := s.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("expected no rpc error, got %+v", rpcErr)
	}
	response, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", result)
	}
	content, ok := response["structuredContent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content, got %#v", response["structuredContent"])
	}
	if content["status"] != "needs_clarification" {
		t.Fatalf("unexpected status: %#v", content["status"])
	}
	if isError, ok := response["isError"].(bool); !ok || !isError {
		t.Fatalf("expected future non-completed envelope to set isError=true, got %#v", response["isError"])
	}
	textContent, ok := response["content"].([]map[string]string)
	if !ok || len(textContent) != 1 {
		t.Fatalf("expected one text content item, got %#v", response["content"])
	}
	if !strings.HasPrefix(textContent[0]["text"], "interactive query needs_clarification:\n") {
		t.Fatalf("expected semantic error text, got %#v", textContent[0]["text"])
	}
	if _, ok := content["result"]; ok {
		t.Fatalf("future status without result should pass through without result, got %#v", content)
	}
}

func TestHandleToolCallAkumaQueryInteractiveNonOKReturnsToolError(t *testing.T) {
	for _, status := range []int{
		http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"bad sql","sql":"select *","warnings":["blocked"]}`))
			}))
			defer api.Close()

			s := &Server{client: &kaizenAPIClient{
				baseURL:    api.URL,
				apiKey:     "test",
				httpClient: api.Client(),
			}}
			raw, err := json.Marshal(toolsCallParams{
				Name: "akuma.query_interactive",
				Arguments: map[string]interface{}{
					"dialect": "postgres",
					"prompt":  "show one row",
				},
			})
			if err != nil {
				t.Fatalf("marshal params: %v", err)
			}

			result, rpcErr := s.handleToolCall(raw)
			if rpcErr != nil {
				t.Fatalf("expected no rpc error, got %+v", rpcErr)
			}
			response, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("expected result map, got %T", result)
			}
			if isError, ok := response["isError"].(bool); !ok || !isError {
				t.Fatalf("expected isError=true, got %#v", response["isError"])
			}
			structured, ok := response["structuredContent"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected structured error content, got %#v", response["structuredContent"])
			}
			if structured["sql"] != "select *" {
				t.Fatalf("expected structured sql payload, got %#v", structured["sql"])
			}
			warnings, ok := structured["warnings"].([]interface{})
			if !ok || len(warnings) != 1 || warnings[0] != "blocked" {
				t.Fatalf("expected structured warnings, got %#v", structured["warnings"])
			}
			content, ok := response["content"].([]map[string]string)
			if !ok || len(content) != 1 {
				t.Fatalf("expected one text content item, got %#v", response["content"])
			}
			if !strings.Contains(content[0]["text"], `"sql": "select *"`) {
				t.Fatalf("expected structured body in tool text, got %#v", content[0]["text"])
			}
		})
	}
}

// --- enzan.pricing_* dispatch tests ---
//
// These cover the 4 new MCP tools added in 8.2-public so the route
// strings, limit forwarding, and exactly-one-of gpu/llm validation can't
// regress unnoticed. Server is driven by httptest so the dispatcher
// hits a real HTTP path.

type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func newPricingTestServer(t *testing.T, captured *[]capturedRequest, responses map[string]string) (*Server, func()) {
	t.Helper()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*captured = append(*captured, capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Body:   string(body),
		})
		respBody, ok := responses[r.Method+" "+r.URL.Path]
		if !ok {
			respBody = "{}"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	srv := &Server{
		client: &kaizenAPIClient{
			baseURL:    hs.URL,
			apiKey:     "test-key",
			httpClient: hs.Client(),
		},
	}
	return srv, hs.Close
}

func TestHandleToolCallEnzanPricingRefreshTriggerHitsCorrectRoute(t *testing.T) {
	var captured []capturedRequest
	s, cleanup := newPricingTestServer(t, &captured, map[string]string{
		"POST /v1/enzan/pricing/refresh": `{"status":"queued","triggeredBy":"u1"}`,
	})
	defer cleanup()

	raw, _ := json.Marshal(toolsCallParams{Name: "enzan.pricing_refresh_trigger", Arguments: map[string]interface{}{}})
	if _, rpcErr := s.handleToolCall(raw); rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	if len(captured) != 1 || captured[0].Method != http.MethodPost || captured[0].Path != "/v1/enzan/pricing/refresh" {
		t.Fatalf("unexpected captured request: %+v", captured)
	}
}

func TestHandleToolCallEnzanPricingRefreshLogForwardsLimitVerbatim(t *testing.T) {
	cases := []struct {
		name      string
		args      map[string]interface{}
		wantQuery string
	}{
		{"default omits limit", map[string]interface{}{}, ""},
		{"forwards positive limit", map[string]interface{}{"limit": 25.0}, "limit=25"},
		{"forwards zero so server can 400", map[string]interface{}{"limit": 0.0}, "limit=0"},
		{"forwards negative value so server can 400", map[string]interface{}{"limit": -1.0}, "limit=-1"},
		{"forwards above-cap value so server can clamp", map[string]interface{}{"limit": 500.0}, "limit=500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured []capturedRequest
			s, cleanup := newPricingTestServer(t, &captured, map[string]string{
				"GET /v1/enzan/pricing/refresh/log": `{"entries":[]}`,
			})
			defer cleanup()

			raw, _ := json.Marshal(toolsCallParams{Name: "enzan.pricing_refresh_log", Arguments: tc.args})
			if _, rpcErr := s.handleToolCall(raw); rpcErr != nil {
				t.Fatalf("rpc error: %+v", rpcErr)
			}
			if len(captured) != 1 {
				t.Fatalf("expected 1 request, got %d (%+v)", len(captured), captured)
			}
			if captured[0].Query != tc.wantQuery {
				t.Fatalf("expected query %q, got %q", tc.wantQuery, captured[0].Query)
			}
		})
	}
}

func TestHandleToolCallEnzanPricingProvidersHitsCorrectRoute(t *testing.T) {
	var captured []capturedRequest
	s, cleanup := newPricingTestServer(t, &captured, map[string]string{
		"GET /v1/enzan/pricing/providers": `{"providers":[]}`,
	})
	defer cleanup()

	raw, _ := json.Marshal(toolsCallParams{Name: "enzan.pricing_providers", Arguments: map[string]interface{}{}})
	if _, rpcErr := s.handleToolCall(raw); rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	if len(captured) != 1 || captured[0].Method != http.MethodGet || captured[0].Path != "/v1/enzan/pricing/providers" {
		t.Fatalf("unexpected captured request: %+v", captured)
	}
}

func TestHandleToolCallEnzanPricingRefreshTriggerPreserves429DroppedBody(t *testing.T) {
	// MCP must surface the {status:"dropped",triggeredBy:...} body as tool
	// data so callers can branch on the typed shape, matching the SDK
	// contract. Without callPreservingTypedBody, this would become a
	// generic "tool error" with the body lost.
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":"dropped","triggeredBy":"u1"}`))
	}))
	defer hs.Close()

	srv := &Server{client: &kaizenAPIClient{baseURL: hs.URL, apiKey: "k", httpClient: hs.Client()}}
	raw, _ := json.Marshal(toolsCallParams{Name: "enzan.pricing_refresh_trigger", Arguments: map[string]interface{}{}})
	result, rpcErr := srv.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	resp, _ := result.(map[string]interface{})
	// Both signals must be present: isError=true so generic MCP clients
	// branching on tool failure correctly see the dropped outcome, AND
	// structuredContent carrying the typed body so callers that want to
	// branch on {status:"dropped", triggeredBy} can read it.
	if resp["isError"] != true {
		t.Fatalf("expected isError=true on 429 dropped, got %+v", resp)
	}
	structured, _ := resp["structuredContent"].(map[string]interface{})
	if got, _ := structured["status"].(string); got != "dropped" {
		t.Fatalf("expected structuredContent.status=\"dropped\", got %q (%+v)", got, structured)
	}
	if got, _ := structured["triggeredBy"].(string); got != "u1" {
		t.Fatalf("expected structuredContent.triggeredBy=\"u1\", got %q", got)
	}
}

func TestHandleToolCallEnzanPricingOffersUpsertPreserves409StaleBody(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"stale"}`))
	}))
	defer hs.Close()

	srv := &Server{client: &kaizenAPIClient{baseURL: hs.URL, apiKey: "k", httpClient: hs.Client()}}
	raw, _ := json.Marshal(toolsCallParams{
		Name: "enzan.pricing_offers_upsert",
		Arguments: map[string]interface{}{
			"gpu": map[string]interface{}{"provider": "p", "gpuType": "g", "displayName": "d", "hourlyRateUSD": 1.0},
		},
	})
	result, rpcErr := srv.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	resp, _ := result.(map[string]interface{})
	if resp["isError"] != true {
		t.Fatalf("expected isError=true on 409 stale, got %+v", resp)
	}
	structured, _ := resp["structuredContent"].(map[string]interface{})
	if got, _ := structured["status"].(string); got != "stale" {
		t.Fatalf("expected structuredContent.status=\"stale\", got %q (%+v)", got, structured)
	}
}

func TestHandleToolCallEnzanPricingOffersUpsertRejectsNullOrNonObjectBranches(t *testing.T) {
	// Schema-bypassing MCP callers could send {"gpu": null} or {"llm": 1};
	// both must be rejected at the tool boundary, not forwarded to the API.
	// Crucially, {"gpu": valid, "llm": "oops"} must also be rejected — both
	// branches are "present" so exactly-one fails, even though llm is not
	// a valid object. Anything less would let callers smuggle a second
	// branch as a non-object value.
	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{"gpu null only", map[string]interface{}{"gpu": nil}},
		{"llm null only", map[string]interface{}{"llm": nil}},
		{"both null", map[string]interface{}{"gpu": nil, "llm": nil}},
		{"gpu non-object", map[string]interface{}{"gpu": 1}},
		{"llm non-object", map[string]interface{}{"llm": "string-not-object"}},
		{"valid gpu plus llm non-object",
			map[string]interface{}{"gpu": map[string]interface{}{"provider": "p", "gpuType": "g", "displayName": "d", "hourlyRateUSD": 1.0}, "llm": "x"}},
		{"valid llm plus gpu non-object",
			map[string]interface{}{"gpu": 0, "llm": map[string]interface{}{"provider": "p", "model": "m", "displayName": "d", "inputCostPer1KTokensUSD": 0, "outputCostPer1KTokensUSD": 0}}},
	}
	var captured []capturedRequest
	srv, cleanup := newPricingTestServer(t, &captured, map[string]string{})
	defer cleanup()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(captured)
			raw, _ := json.Marshal(toolsCallParams{Name: "enzan.pricing_offers_upsert", Arguments: tc.args})
			result, rpcErr := srv.handleToolCall(raw)
			if rpcErr != nil {
				t.Fatalf("rpc error: %+v", rpcErr)
			}
			resp, _ := result.(map[string]interface{})
			if resp["isError"] != true {
				t.Fatalf("expected isError=true for %q, got %+v", tc.name, resp)
			}
			if len(captured) != before {
				t.Fatalf("expected no HTTP request for %q, got %+v", tc.name, captured[before:])
			}
		})
	}
}

func TestHandleToolCallEnzanPricingOffersUpsertLLMBranchHitsServer(t *testing.T) {
	// Symmetric coverage: GPU branch is exercised in another test; this
	// verifies the LLM branch's wire body has only the llm key.
	var captured []capturedRequest
	srv, cleanup := newPricingTestServer(t, &captured, map[string]string{
		"POST /v1/enzan/pricing/offers": `{"status":"upserted","llm":{"id":"x"}}`,
	})
	defer cleanup()

	raw, _ := json.Marshal(toolsCallParams{
		Name: "enzan.pricing_offers_upsert",
		Arguments: map[string]interface{}{
			"llm": map[string]interface{}{"provider": "p", "model": "m", "displayName": "d", "inputCostPer1KTokensUSD": 0.001, "outputCostPer1KTokensUSD": 0.002},
		},
	})
	if _, rpcErr := srv.handleToolCall(raw); rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	if len(captured) != 1 || captured[0].Path != "/v1/enzan/pricing/offers" {
		t.Fatalf("expected single POST to /v1/enzan/pricing/offers, got %+v", captured)
	}
	if !strings.Contains(captured[0].Body, `"llm"`) || strings.Contains(captured[0].Body, `"gpu"`) {
		t.Fatalf("expected LLM-only request body, got %s", captured[0].Body)
	}
}

func TestCallPreservingTypedBodyPassesThroughNonAPICallErrors(t *testing.T) {
	// Network/transport errors (no apiCallError, just a wrapped fmt.Errorf
	// from the http client) must bubble up unchanged — only typed
	// non-2xx bodies in preserveStatuses get rewritten to typedBodyError.
	// Without this test, a future refactor could accidentally swallow
	// transport errors as success.
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't even respond — close the connection mid-flight.
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer hs.Close()

	srv := &Server{client: &kaizenAPIClient{baseURL: hs.URL, apiKey: "k", httpClient: hs.Client()}}
	_, err := srv.callPreservingTypedBody(context.Background(), "POST", "/v1/enzan/pricing/refresh", nil, []int{http.StatusTooManyRequests})
	if err == nil {
		t.Fatalf("expected transport error, got nil")
	}
	var typedErr *typedBodyError
	if errors.As(err, &typedErr) {
		t.Fatalf("transport error should not be rewritten as typedBodyError, got %+v", typedErr)
	}
}

func TestHandleToolCallEnzanPricingOffersUpsertEnforcesExactlyOne(t *testing.T) {
	// Both gpu and llm set: server must NOT be hit (validation runs in MCP).
	var capturedBoth []capturedRequest
	sBoth, cleanupBoth := newPricingTestServer(t, &capturedBoth, map[string]string{})
	defer cleanupBoth()

	raw, _ := json.Marshal(toolsCallParams{
		Name: "enzan.pricing_offers_upsert",
		Arguments: map[string]interface{}{
			"gpu": map[string]interface{}{"provider": "p", "gpuType": "g", "displayName": "d", "hourlyRateUSD": 1.0},
			"llm": map[string]interface{}{"provider": "p", "model": "m", "displayName": "d", "inputCostPer1KTokensUSD": 0.0, "outputCostPer1KTokensUSD": 0.0},
		},
	})
	result, rpcErr := sBoth.handleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	resp, ok := result.(map[string]interface{})
	if !ok || resp["isError"] != true {
		t.Fatalf("expected tool error for both gpu+llm, got %+v", result)
	}
	errText, _ := resp["content"].([]map[string]string)
	if len(errText) > 0 && !strings.Contains(errText[0]["text"], "exactly one") {
		t.Fatalf("expected exactly-one-of error message, got %+v", errText)
	}
	if len(capturedBoth) != 0 {
		t.Fatalf("expected no HTTP request when validation rejects, got %+v", capturedBoth)
	}

	// Neither gpu nor llm set: same outcome.
	rawNone, _ := json.Marshal(toolsCallParams{
		Name:      "enzan.pricing_offers_upsert",
		Arguments: map[string]interface{}{},
	})
	resultNone, rpcErrNone := sBoth.handleToolCall(rawNone)
	if rpcErrNone != nil {
		t.Fatalf("rpc error: %+v", rpcErrNone)
	}
	respNone, _ := resultNone.(map[string]interface{})
	if respNone["isError"] != true {
		t.Fatalf("expected tool error for neither gpu nor llm, got %+v", resultNone)
	}

	// Only gpu set: must reach POST /v1/enzan/pricing/offers with gpu key.
	var capturedGPU []capturedRequest
	sGPU, cleanupGPU := newPricingTestServer(t, &capturedGPU, map[string]string{
		"POST /v1/enzan/pricing/offers": `{"status":"upserted","gpu":{"id":"x"}}`,
	})
	defer cleanupGPU()
	rawGPU, _ := json.Marshal(toolsCallParams{
		Name: "enzan.pricing_offers_upsert",
		Arguments: map[string]interface{}{
			"gpu": map[string]interface{}{"provider": "p", "gpuType": "g", "displayName": "d", "hourlyRateUSD": 1.0},
		},
	})
	if _, rpcErr := sGPU.handleToolCall(rawGPU); rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	if len(capturedGPU) != 1 || capturedGPU[0].Path != "/v1/enzan/pricing/offers" {
		t.Fatalf("expected single POST to /v1/enzan/pricing/offers, got %+v", capturedGPU)
	}
	if !strings.Contains(capturedGPU[0].Body, `"gpu"`) || strings.Contains(capturedGPU[0].Body, `"llm"`) {
		t.Fatalf("expected request body to contain gpu but not llm, got %s", capturedGPU[0].Body)
	}
}

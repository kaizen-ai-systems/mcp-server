package mcp

import (
	"encoding/json"
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
		"enzan.pricing_models":        false,
		"enzan.set_model_pricing":     false,
		"enzan.pricing_gpus":          false,
		"enzan.set_gpu_pricing":       false,
		"enzan.routing":               false,
		"enzan.set_routing":           false,
		"enzan.routing_savings":       false,
		"enzan.alerts":                false,
		"enzan.create_alert":          false,
		"enzan.update_alert":          false,
		"enzan.delete_alert":          false,
		"enzan.alert_events":          false,
		"enzan.alert_deliveries":      false,
		"enzan.alert_endpoints":       false,
		"enzan.create_alert_endpoint": false,
		"enzan.update_alert_endpoint": false,
		"enzan.delete_alert_endpoint": false,
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

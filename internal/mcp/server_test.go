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

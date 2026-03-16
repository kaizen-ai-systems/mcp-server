package mcp

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
					"sourceId":   map[string]interface{}{"type": "string"},
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
			Name:        "akuma.schema",
			Description: "Set Akuma schema context used for query generation.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sourceId": map[string]interface{}{"type": "string"},
					"name":     map[string]interface{}{"type": "string"},
					"dialect":  map[string]interface{}{"type": "string", "enum": []string{"postgres", "mysql", "snowflake", "bigquery"}},
					"version":  map[string]interface{}{"type": "string"},
					"tables":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
				},
				"required":             []string{"dialect", "tables"},
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
			Name:        "enzan.costs_by_model",
			Description: "Break down Akuma API spend by model for a time window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window": map[string]interface{}{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}},
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
				"required":             []string{"records"},
				"additionalProperties": false,
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

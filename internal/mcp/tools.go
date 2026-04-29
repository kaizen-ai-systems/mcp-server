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
			Name:        "enzan.routing",
			Description: "Get the current Enzan smart-routing config.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.set_routing",
			Description: "Upsert the current Enzan smart-routing config.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enabled":        map[string]interface{}{"type": "boolean"},
					"simple_model":   map[string]interface{}{"type": "string"},
					"moderate_model": map[string]interface{}{"type": "string"},
					"complex_model":  map[string]interface{}{"type": "string"},
				},
				"required":             []string{"enabled"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.routing_savings",
			Description: "Get realized Enzan smart-routing savings for a time window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window": map[string]interface{}{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_models",
			Description: "List configured LLM pricing entries.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.set_model_pricing",
			Description: "Upsert one LLM pricing entry (admin API key required).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider":                      map[string]interface{}{"type": "string"},
					"model":                         map[string]interface{}{"type": "string"},
					"display_name":                  map[string]interface{}{"type": "string"},
					"input_cost_per_1k_tokens_usd":  map[string]interface{}{"type": "number"},
					"output_cost_per_1k_tokens_usd": map[string]interface{}{"type": "number"},
					"currency":                      map[string]interface{}{"type": "string"},
					"active":                        map[string]interface{}{"type": "boolean"},
				},
				"required":             []string{"provider", "model", "input_cost_per_1k_tokens_usd", "output_cost_per_1k_tokens_usd"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_gpus",
			Description: "List configured GPU pricing entries.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.set_gpu_pricing",
			Description: "Upsert one GPU pricing entry (admin API key required).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider":        map[string]interface{}{"type": "string"},
					"gpu_type":        map[string]interface{}{"type": "string"},
					"display_name":    map[string]interface{}{"type": "string"},
					"hourly_rate_usd": map[string]interface{}{"type": "number"},
					"currency":        map[string]interface{}{"type": "string"},
					"active":          map[string]interface{}{"type": "boolean"},
				},
				"required":             []string{"provider", "gpu_type", "hourly_rate_usd"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_refresh_trigger",
			Description: "Trigger an on-demand live-pricing refresh sweep (admin enzan_pricing_admin required). Fire-and-forget; poll enzan.pricing_refresh_log for status.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_refresh_log",
			Description: "List recent live-pricing refresh-log entries (admin enzan_pricing_admin required). Default 50; server clamps to 1..200 and rejects non-positive values with 400. Limit is forwarded verbatim so the server remains the clamp/validation authority.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "integer"},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_providers",
			Description: "List registered live-pricing sources with adapter availability hints (admin enzan_pricing_admin required).",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.pricing_offers_upsert",
			Description: "Upsert one manual (admin-authored) live-pricing offer; exactly one of gpu or llm must be set (admin enzan_pricing_admin required).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"gpu": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"provider":          map[string]interface{}{"type": "string"},
							"gpuType":           map[string]interface{}{"type": "string"},
							"displayName":       map[string]interface{}{"type": "string"},
							"region":            map[string]interface{}{"type": "string"},
							"deploymentClass":   map[string]interface{}{"type": "string", "enum": []string{"on_demand", "reserved", "spot", "committed_monthly"}},
							"commitmentTerm":    map[string]interface{}{"type": "string"},
							"clusterSizeMin":    map[string]interface{}{"type": "integer"},
							"clusterSizeMax":    map[string]interface{}{"type": "integer"},
							"interconnectClass": map[string]interface{}{"type": "string", "enum": []string{"standard", "high_speed", "infiniband", "nvlink", "unknown"}},
							"trainingReady":     map[string]interface{}{"type": "boolean"},
							"hourlyRateUSD":     map[string]interface{}{"type": "number", "minimum": 0},
							"currency":          map[string]interface{}{"type": "string"},
							"currencyFxNote":    map[string]interface{}{"type": "string"},
							"sourceUrl":         map[string]interface{}{"type": "string"},
						},
						"required":             []string{"provider", "gpuType", "displayName", "hourlyRateUSD"},
						"additionalProperties": false,
					},
					"llm": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"provider":                 map[string]interface{}{"type": "string"},
							"model":                    map[string]interface{}{"type": "string"},
							"displayName":              map[string]interface{}{"type": "string"},
							"region":                   map[string]interface{}{"type": "string"},
							"commitmentTerm":           map[string]interface{}{"type": "string"},
							"inputCostPer1KTokensUSD":  map[string]interface{}{"type": "number", "minimum": 0},
							"outputCostPer1KTokensUSD": map[string]interface{}{"type": "number", "minimum": 0},
							"currency":                 map[string]interface{}{"type": "string"},
							"currencyFxNote":           map[string]interface{}{"type": "string"},
							"sourceUrl":                map[string]interface{}{"type": "string"},
						},
						"required":             []string{"provider", "model", "displayName", "inputCostPer1KTokensUSD", "outputCostPer1KTokensUSD"},
						"additionalProperties": false,
					},
				},
				// JSON-Schema-level "exactly one of gpu or llm" enforcement so
				// generated clients reject invalid payloads at the contract
				// layer rather than the runtime 400 path.
				"oneOf": []map[string]interface{}{
					{"required": []string{"gpu"}, "not": map[string]interface{}{"required": []string{"llm"}}},
					{"required": []string{"llm"}, "not": map[string]interface{}{"required": []string{"gpu"}}},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.optimize",
			Description: "Generate cost optimization recommendations for a time window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window": map[string]interface{}{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.alerts",
			Description: "List configured Enzan alert rules.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.create_alert",
			Description: "Create one Enzan alert rule.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":        map[string]interface{}{"type": "string"},
					"name":      map[string]interface{}{"type": "string"},
					"type":      map[string]interface{}{"type": "string", "enum": []string{"cost_threshold", "cost_anomaly", "budget_exceeded", "optimization_available", "pricing_change", "daily_summary"}},
					"threshold": map[string]interface{}{"type": "number"},
					"window":    map[string]interface{}{"type": "string"},
					"labels": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{"type": "string"},
					},
					"enabled": map[string]interface{}{"type": "boolean"},
				},
				"required":             []string{"name", "type"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.update_alert",
			Description: "Update one Enzan alert rule by id.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":        map[string]interface{}{"type": "string"},
					"name":      map[string]interface{}{"type": "string"},
					"threshold": map[string]interface{}{"type": "number"},
					"window":    map[string]interface{}{"type": "string"},
					"labels": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{"type": "string"},
					},
					"enabled": map[string]interface{}{"type": "boolean"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.delete_alert",
			Description: "Delete one Enzan alert rule by id.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.alert_events",
			Description: "List recent Enzan alert events.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.alert_deliveries",
			Description: "List recent Enzan alert deliveries.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.alert_endpoints",
			Description: "List configured Enzan alert delivery webhook endpoints.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.create_alert_endpoint",
			Description: "Create one Enzan alert delivery webhook endpoint.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"targetUrl":     map[string]interface{}{"type": "string"},
					"signingSecret": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"targetUrl"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.update_alert_endpoint",
			Description: "Update one Enzan alert delivery webhook endpoint by id.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":            map[string]interface{}{"type": "string"},
					"targetUrl":     map[string]interface{}{"type": "string"},
					"signingSecret": map[string]interface{}{"type": "string"},
					"enabled":       map[string]interface{}{"type": "boolean"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.delete_alert_endpoint",
			Description: "Delete one Enzan alert delivery webhook endpoint by id.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "enzan.chat",
			Description: "Ask a question about your GPU and API costs. Supports multi-turn conversations with optional time window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message":        map[string]interface{}{"type": "string", "description": "Your question about costs"},
					"conversationId": map[string]interface{}{"type": "string", "description": "Optional conversation ID for follow-ups"},
					"window":         map[string]interface{}{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}, "description": "Optional time window; inferred from message if omitted"},
				},
				"required":             []string{"message"},
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

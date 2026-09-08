# Kaizen MCP Server

Model Context Protocol (MCP) server for Kaizen APIs over stdio.

## Available tools

- `akuma.query`
- `akuma.query_interactive`
- `akuma.explain`
- `akuma.schema`
- `enzan.summary`
- `enzan.costs_by_model`
- `enzan.optimize`
- `enzan.routing`
- `enzan.set_routing`
- `enzan.routing_savings`
- `enzan.alerts`
- `enzan.create_alert`
- `enzan.update_alert`
- `enzan.delete_alert`
- `enzan.alert_events`
- `enzan.alert_deliveries`
- `enzan.alert_endpoints`
- `enzan.create_alert_endpoint`
- `enzan.update_alert_endpoint`
- `enzan.delete_alert_endpoint`
- `enzan.chat`
- `enzan.pricing_models`
- `enzan.set_model_pricing`
- `enzan.pricing_gpus`
- `enzan.pricing_search`
- `enzan.set_gpu_pricing`
- `enzan.pricing_refresh_trigger`
- `enzan.pricing_refresh_log`
- `enzan.pricing_providers`
- `enzan.pricing_offers_upsert`
- `enzan.burn`
- `sozo.generate`
- `sozo.schemas`

`akuma.query_interactive` returns HTTP 200 interactive envelopes as structured tool content. The known statuses are `completed`, `needs_clarification`, and `rejected`; future statuses pass through as semantic tool errors with the full envelope mirrored into `structuredContent`. `rejected` envelopes must include a non-empty `result.error`, and `completed` envelopes must not carry `result.error`. Typed non-2xx Akuma bodies are also MCP tool errors with decoded `structuredContent` so clients can inspect fields such as `sql`, `warnings`, and `tables`.

`completed` envelopes that carry executed SQL also include a `provenance` object: structured, server-derived facts about how the answer was produced — `provenanceFidelity` (`full` when parser-backed extraction succeeded for `postgres`/`mysql`, else `limited`), plus `tables`, `joins`, `filters`, `projections`, `aggregations`, and enumerated `warnings` (categories such as `parser_extraction_failed`). Hosts must not present `limited` provenance as equivalent to `full`.

`needs_clarification` envelopes carry a `clarification` object instead of `result`. Each clarification includes a server-issued `clarificationToken`, a short `question`, an `expiresAt` timestamp, and a list of `options` (each with `id`, `label`, and optional `description`). The MCP client surfaces these via `structuredContent` with `isError: true` so the host LLM can present the choices and obtain the user's selection.

To consume a clarification, call `akuma.query_interactive` again with `clarificationToken`, `optionId`, and `idempotencyKey` arguments (no `dialect`/`prompt` needed; the token identifies the pending clarification). The MCP forwards `idempotencyKey` to the API as the `Idempotency-Key` header. Same token + same `optionId` + same `idempotencyKey` replays the persisted result; a different `idempotencyKey`, or a different `optionId` with the same key, is rejected (409). A change to the source schema after issuance is rejected with 409 and the user must restart the query flow. Unconsumed tokens expire at their `expiresAt` timestamp (410); after a consume, the result is retained for replay for 24 hours, after which replays return 404.

The `akuma.query` tool remains supported for existing clients with its flat success response and text-only MCP error surface. Use `akuma.query_interactive` for new MCP integrations that need interactive statuses, the clarification flow, or typed non-2xx Akuma error bodies in `structuredContent`.

## Required environment variables

```bash
export KAIZEN_API_BASE_URL=https://api.kaizenaisystems.com
export KAIZEN_API_KEY=your-platform-key
```

## Run (from source)

```bash
cd cmd/mcp
go run .
```

## Install (public repo)

```bash
go install github.com/kaizen-ai-systems/mcp-server@latest
```

Then run:

```bash
mcp-server
```

## Claude Desktop config

```json
{
  "mcpServers": {
    "kaizen": {
      "command": "mcp-server",
      "env": {
        "KAIZEN_API_BASE_URL": "https://api.kaizenaisystems.com",
        "KAIZEN_API_KEY": "your-platform-key"
      }
    }
  }
}
```

## Cursor config

```json
{
  "mcpServers": {
    "kaizen": {
      "command": "mcp-server",
      "env": {
        "KAIZEN_API_BASE_URL": "https://api.kaizenaisystems.com",
        "KAIZEN_API_KEY": "your-platform-key"
      }
    }
  }
}
```

## Local validation

```bash
cd cmd/mcp
go test ./...
```

## Protocol details

- Transport: stdio
- Framing: `Content-Length` JSON-RPC messages (line-delimited JSON accepted for smoke tests)
- Protocol version: `2024-11-05`

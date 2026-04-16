# Kaizen MCP Server

Model Context Protocol (MCP) server for Kaizen APIs over stdio.

This package is the MCP source of truth in the monorepo and is snapshot-published to the public repo via the manifest-driven public publish workflow:

- <https://github.com/kaizen-ai-systems/mcp-server>

## Available tools

- `akuma.query`
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
- `enzan.set_gpu_pricing`
- `enzan.burn`
- `sozo.generate`
- `sozo.schemas`

## Required environment variables

```bash
export KAIZEN_API_BASE_URL=https://api.kaizenaisystems.com
export KAIZEN_API_KEY=your-platform-key
```

## Run (monorepo)

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

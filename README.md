# Kaizen MCP Server

Model Context Protocol (MCP) server for Kaizen APIs over stdio.

This package is the MCP source of truth in the monorepo and is subtree-published to:

- <https://github.com/kaizen-ai-systems/mcp-server>

## Available tools

- `akuma.query`
- `akuma.explain`
- `akuma.schema`
- `enzan.summary`
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

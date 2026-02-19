// Package main provides the Kaizen MCP server over stdio.
package main

import (
	"os"

	"github.com/kaizen-ai-systems/mcp-server/internal/mcp"
)

func main() {
	server := mcp.NewServer()
	server.LogStartup()
	if err := server.Serve(); err != nil {
		server.LogFatal(err)
		os.Exit(1)
	}
}

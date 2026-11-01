package mcp

const (
	serverName    = "kaizen-mcp"
	serverVersion = "1.0.0"
	protocol      = "2025-06-18"

	// Inbound payload limit, excluding LF or CRLF; see README.md, Protocol details.
	maxMessageBytes = 1 << 20
)

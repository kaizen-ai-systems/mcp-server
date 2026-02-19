package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MCP clients use Content-Length framing over stdio, but we also accept
// line-delimited JSON for local smoke tests.
func readMessage(reader *bufio.Reader) ([]byte, error) {
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			trimmed := strings.TrimSpace(firstLine)
			if trimmed == "" {
				return nil, io.EOF
			}
			if strings.HasPrefix(trimmed, "{") {
				return []byte(trimmed), nil
			}
		}
		return nil, err
	}

	trimmed := strings.TrimSpace(firstLine)
	if trimmed == "" {
		return nil, fmt.Errorf("received empty message")
	}
	if strings.HasPrefix(trimmed, "{") {
		return []byte(trimmed), nil
	}

	headers := []string{strings.TrimRight(firstLine, "\r\n")}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		clean := strings.TrimRight(line, "\r\n")
		if clean == "" {
			break
		}
		headers = append(headers, clean)
	}

	length, err := parseContentLength(headers)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}
	return payload, nil
}

func parseContentLength(headers []string) (int, error) {
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			continue
		}
		rawLen := strings.TrimSpace(parts[1])
		length, err := strconv.Atoi(rawLen)
		if err != nil || length <= 0 {
			return 0, fmt.Errorf("invalid Content-Length value: %q", rawLen)
		}
		return length, nil
	}
	return 0, fmt.Errorf("missing Content-Length header")
}

func writeMessage(writer *bufio.Writer, response jsonRPCResponse) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return writer.Flush()
}

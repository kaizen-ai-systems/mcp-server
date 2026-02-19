package mcp

import (
	"bufio"
	"strconv"
	"strings"
	"testing"
)

func TestParseContentLength(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    int
		wantErr bool
	}{
		{name: "valid", headers: []string{"Content-Length: 12"}, want: 12},
		{name: "mixed case", headers: []string{"content-length: 7"}, want: 7},
		{name: "missing", headers: []string{"X-Test: 1"}, wantErr: true},
		{name: "invalid", headers: []string{"Content-Length: nope"}, wantErr: true},
		{name: "zero", headers: []string{"Content-Length: 0"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContentLength(tt.headers)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReadMessageLineDelimitedJSON(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("{\"jsonrpc\":\"2.0\"}\n"))
	msg, err := readMessage(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != "{\"jsonrpc\":\"2.0\"}" {
		t.Fatalf("unexpected payload: %s", string(msg))
	}
}

func TestReadMessageFramed(t *testing.T) {
	payload := "{\"jsonrpc\":\"2.0\",\"method\":\"ping\"}"
	raw := "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + payload
	reader := bufio.NewReader(strings.NewReader(raw))
	msg, err := readMessage(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != payload {
		t.Fatalf("unexpected payload: %s", string(msg))
	}
}

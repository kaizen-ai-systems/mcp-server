package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// cmd/mcp/README.md:124,126 pins the input and exponent budgets to 1 MiB.
const testMessageLimit = 1_048_576

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

func TestReadMessageSkipsBlankLines(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n\r\n  \t\n{\"a\":1}\n\n"))
	msg, err := readMessage(reader)
	if err != nil || string(msg) != `{"a":1}` {
		t.Fatalf("read = %q, %v", msg, err)
	}
	if _, err := readMessage(reader); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestReadMessageEOF(t *testing.T) {
	for _, input := range []string{"", "\n\r\n \t", `{"a":1}`, "not json"} {
		t.Run(input, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(input))
			msg, err := readMessage(reader)
			if strings.TrimSpace(input) == "" {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("expected EOF, got %v", err)
				}
			} else if err != nil || string(msg) != input {
				t.Fatalf("read = %q, %v", msg, err)
			}
			if _, err := readMessage(reader); !errors.Is(err, io.EOF) {
				t.Fatalf("expected EOF, got %v", err)
			}
		})
	}
}

func TestReadMessageSizeBoundary(t *testing.T) {
	for _, size := range []int{testMessageLimit - 1, testMessageLimit, testMessageLimit + 1, 3 * testMessageLimit} {
		for _, ending := range []string{"\n", "\r\n", ""} {
			t.Run(fmt.Sprintf("%d/%q", size, ending), func(t *testing.T) {
				input := strings.Repeat("x", size)
				msg, err := readMessage(bufio.NewReader(strings.NewReader(input + ending)))
				if size > testMessageLimit {
					if !errors.Is(err, errMessageTooLarge) || msg != nil {
						t.Fatalf("expected oversize error, got payload length %d, %v", len(msg), err)
					}
				} else if err != nil || string(msg) != input {
					t.Fatalf("read length = %d, error = %v", len(msg), err)
				}
			})
		}
	}
	for _, size := range []int{testMessageLimit - 1, testMessageLimit} {
		msg, err := readMessage(bufio.NewReader(strings.NewReader(strings.Repeat("x", size) + "\r")))
		if size == testMessageLimit {
			if !errors.Is(err, errMessageTooLarge) {
				t.Fatalf("unterminated CR must count toward limit: %v", err)
			}
		} else if err != nil || len(msg) != testMessageLimit {
			t.Fatalf("unterminated CR boundary: length %d, %v", len(msg), err)
		}
	}
	reader := bufio.NewReader(strings.NewReader(strings.Repeat(" ", testMessageLimit+1) + "\n"))
	if _, err := readMessage(reader); !errors.Is(err, errMessageTooLarge) {
		t.Fatalf("oversized whitespace must not be skipped: %v", err)
	}
}

func TestReadMessageBufferBoundaries(t *testing.T) {
	for _, size := range []int{14, 15, 16, 31, testMessageLimit} {
		input := strings.Repeat("x", size)
		reader := bufio.NewReaderSize(strings.NewReader(input+"\r\n"), 16)
		msg, err := readMessage(reader)
		if err != nil || string(msg) != input {
			t.Fatalf("size %d: read length %d, %v", size, len(msg), err)
		}
	}
}

func TestReadMessageOversizeDoesNotDrain(t *testing.T) {
	input := strings.NewReader(strings.Repeat("x", 3*testMessageLimit) + "\n")
	if _, err := readMessage(bufio.NewReaderSize(input, 16)); !errors.Is(err, errMessageTooLarge) {
		t.Fatalf("expected oversize error, got %v", err)
	}
	if input.Len() < testMessageLimit {
		t.Fatal("oversized input was drained")
	}
}

func TestReadMessageReadErrorDiscardsPartialLine(t *testing.T) {
	wantErr := errors.New("read failure")
	input := io.MultiReader(strings.NewReader(`{"a":1}`), iotest.ErrReader(wantErr))
	msg, err := readMessage(bufio.NewReader(input))
	if !errors.Is(err, wantErr) || msg != nil {
		t.Fatalf("read = %q, %v", msg, err)
	}
}

func TestWriteMessageNewlineDelimited(t *testing.T) {
	var output bytes.Buffer
	response := jsonRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: map[string]string{"text": "first\nsecond"}}
	if err := writeMessage(bufio.NewWriter(&output), response); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "\n") != 1 || strings.Contains(output.String(), "Content-Length") {
		t.Fatalf("expected exactly one JSON line, got %q", output.String())
	}
	var decoded struct {
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil || decoded.Result["text"] != "first\nsecond" {
		t.Fatalf("round-trip = %#v, %v", decoded, err)
	}
}

func TestWriteMessageResponseShape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response jsonRPCResponse
		want     string
	}{
		{"nil success", jsonRPCResponse{JSONRPC: "2.0"}, `{"jsonrpc":"2.0","id":null,"result":{}}`},
		{"error clears result", jsonRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`0`), Result: map[string]int{"ignored": 1}, Error: &jsonRPCError{Code: -32600, Message: "invalid request"}}, `{"jsonrpc":"2.0","id":0,"error":{"code":-32600,"message":"invalid request"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writeMessage(bufio.NewWriter(&output), tc.response); err != nil {
				t.Fatal(err)
			}
			if output.String() != tc.want+"\n" {
				t.Fatalf("got %q, want %q", output.String(), tc.want+"\n")
			}
		})
	}
}

package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var errMessageTooLarge = errors.New("message exceeds 1 MiB input limit")

// readMessage accepts UTF-8 JSON lines, including a final line without LF.
// Oversize is terminal: do not drain an unbounded line to find its terminator.
func readMessage(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
			return nil, err
		}
		// Reserve at most two extra bytes for CRLF, excluded from the limit.
		if len(chunk) > maxMessageBytes+2-len(line) {
			return nil, errMessageTooLarge
		}
		if size := len(line) + len(chunk); size > cap(line) {
			capacity := 2*cap(line) + len(chunk)
			if capacity > maxMessageBytes+2 {
				capacity = maxMessageBytes + 2
			}
			grown := make([]byte, len(line), capacity)
			copy(grown, line)
			line = grown
		}
		line = append(line, chunk...)
		if errors.Is(err, bufio.ErrBufferFull) {
			if len(line) > maxMessageBytes && !(len(line) == maxMessageBytes+1 && line[len(line)-1] == '\r') {
				return nil, errMessageTooLarge
			}
			continue
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
		}
		if len(line) > maxMessageBytes {
			return nil, errMessageTooLarge
		}
		if len(bytes.TrimSpace(line)) != 0 {
			return line, nil
		}
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		line = line[:0]
	}
}

func writeMessage(writer *bufio.Writer, response jsonRPCResponse) error {
	if response.Error != nil {
		response.Result = nil
	} else if response.Result == nil {
		response.Result = map[string]interface{}{}
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	return writer.Flush()
}

// Stub MCP server for the agents package's mcp_driver_test.go.
// Implements the JSON-RPC 2.0 over stdio handshake (initialize +
// notifications/initialized + tools/call) and returns a canned
// response from the analyze tool.
//
// Behavior controlled via env vars (used by tests):
//   STUB_BEHAVIOR=ok               → return canned findings (default)
//   STUB_BEHAVIOR=error            → return JSON-RPC error
//   STUB_BEHAVIOR=invalid-severity → return finding with bogus severity
//
// Build: `go build -o stub_mcp_server ./testdata/stub_mcp_server`.
// The mcp_driver_test.go builds this on demand from within the test.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	rdr := bufio.NewReader(os.Stdin)
	wtr := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(wtr)
	for {
		line, err := rdr.ReadBytes('\n')
		if err != nil {
			return
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Result: map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]string{"name": "stub", "version": "0.0.0"},
					"capabilities":    map[string]any{},
				},
			})
			_ = wtr.Flush()
		case "notifications/initialized":
			// no response for notifications
		case "tools/call":
			behavior := os.Getenv("STUB_BEHAVIOR")
			if behavior == "" {
				behavior = "ok"
			}
			switch behavior {
			case "error":
				_ = enc.Encode(rpcResponse{
					JSONRPC: "2.0", ID: *req.ID,
					Error: &struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: -32000, Message: "stub-error"},
				})
			case "invalid-severity":
				findings := `[{"severity":"bogus","summary":"x","file":"a.go","line_range":"1","reasoning":"r","confidence":1.0}]`
				_ = enc.Encode(rpcResponse{
					JSONRPC: "2.0", ID: *req.ID,
					Result: map[string]any{
						"content": []map[string]string{{"type": "text", "text": findings}},
					},
				})
			default: // ok
				findings := `[{"severity":"issue","summary":"missing input validation","file":"main.go","line_range":"42-45","reasoning":"semgrep rule X","confidence":0.95}]`
				_ = enc.Encode(rpcResponse{
					JSONRPC: "2.0", ID: *req.ID,
					Result: map[string]any{
						"content": []map[string]string{{"type": "text", "text": findings}},
					},
				})
			}
			_ = wtr.Flush()
		default:
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: *req.ID,
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{Code: -32601, Message: fmt.Sprintf("method %q not found", req.Method)},
			})
			_ = wtr.Flush()
		}
	}
}

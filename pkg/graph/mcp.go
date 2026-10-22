package graph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MCPRequest represents an incoming JSON-RPC 2.0 request from an MCP host (Cursor, Antigravity, OpenCode, Claude Code).
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPToolResult struct {
	Content []MCPTextContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type MCPTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ServeMCP runs the Model Context Protocol stdio server for graph tools.
func ServeMCP(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	encoder := json.NewEncoder(out)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(lineStr), &req); err != nil {
			continue
		}

		resp := handleMCPRequest(&req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return err
			}
		}
	}
}

func handleMCPRequest(req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "go-graphify",
					"version": "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		return nil

	case "tools/list":
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "repo_map",
						"description": "Generates a compressed AST skeleton of the Go codebase or directory (signatures, structs, interfaces with function bodies stripped). Massive ~90% token reduction.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"dir": map[string]interface{}{
									"type":        "string",
									"description": "Root directory path to map (default: '.')",
								},
								"exported_only": map[string]interface{}{
									"type":        "boolean",
									"description": "If true, only exported structs and functions are included",
								},
							},
						},
					},
					{
						"name":        "who_calls",
						"description": "Graphify AST call graph lookup in pure Go: queries all caller locations for target function in microseconds (~30 tokens instead of reading entire files).",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"target": map[string]interface{}{
									"type":        "string",
									"description": "Function or method name to find callers for (e.g. 'NewDedicatedIPClient')",
								},
								"dir": map[string]interface{}{
									"type":        "string",
									"description": "Root directory to scan (default: '.')",
								},
							},
							"required": []string{"target"},
						},
					},
				},
			},
		}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32602, Message: "Invalid parameters"},
			}
		}

		resultText := ""
		switch params.Name {
		case "repo_map":
			dir := "."
			if d, ok := params.Arguments["dir"].(string); ok && d != "" {
				dir = d
			}
			exportedOnly := false
			if eo, ok := params.Arguments["exported_only"].(bool); ok {
				exportedOnly = eo
			}
			opts := DefaultSkeletonOptions()
			opts.ExportedOnly = exportedOnly
			res, err := GenerateRepoMap(dir, opts)
			if err != nil {
				resultText = fmt.Sprintf("Error generating repo map: %v", err)
			} else {
				resultText = res
			}

		case "who_calls":
			target, _ := params.Arguments["target"].(string)
			if target == "" {
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &MCPError{Code: -32602, Message: "target is required"},
				}
			}
			dir := "."
			if d, ok := params.Arguments["dir"].(string); ok && d != "" {
				dir = d
			}
			cg, err := BuildCallGraph(dir)
			if err != nil {
				resultText = fmt.Sprintf("Error building call graph: %v", err)
			} else {
				resultText = cg.FormatWhoCalls(target)
			}

		default:
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Tool not found: %s", params.Name)},
			}
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: MCPToolResult{
				Content: []MCPTextContent{
					{Type: "text", Text: resultText},
				},
			},
		}

	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		}
	}
}

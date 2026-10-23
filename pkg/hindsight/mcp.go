package hindsight

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

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

// ServeMCP runs a stdio Model Context Protocol server exposing retain, recall, and reflect.
func ServeMCP(store *Store, in io.Reader, out io.Writer) error {
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

		resp := handleMCPRequest(store, &req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return err
			}
		}
	}
}

func handleMCPRequest(store *Store, req *MCPRequest) *MCPResponse {
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
					"name":    "go-hindsight",
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
						"name":        "hindsight_retain",
						"description": "Store an atomic memory (fact, preference, lesson, decision) into long-term cognitive memory with automatic contradiction resolution.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"bank": map[string]interface{}{
									"type":        "string",
									"description": "Memory bank name (e.g. 'agent', 'vps', 'user', 'project')",
									"default":     "agent",
								},
								"type": map[string]interface{}{
									"type":        "string",
									"enum":        []string{"fact", "preference", "lesson", "decision"},
									"description": "Category of memory: fact, preference, lesson, or decision",
								},
								"subject": map[string]interface{}{
									"type":        "string",
									"description": "Specific subject or key identifier (e.g. 'vps_build_rule', 'bot_huntera_port')",
								},
								"content": map[string]interface{}{
									"type":        "string",
									"description": "The exact fact or knowledge to remember",
								},
							},
							"required": []string{"type", "subject", "content"},
						},
					},
					{
						"name":        "hindsight_recall",
						"description": "Retrieve relevant memories using Okapi BM25 lexical search and recency decay. Saves tokens by returning only exact matches.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"bank": map[string]interface{}{
									"type":        "string",
									"description": "Memory bank name to search within",
									"default":     "agent",
								},
								"query": map[string]interface{}{
									"type":        "string",
									"description": "Search query describing what you need to remember",
								},
								"top_k": map[string]interface{}{
									"type":        "integer",
									"description": "Maximum number of memories to return (default 5)",
									"default":     5,
								},
							},
							"required": []string{"query"},
						},
					},
					{
						"name":        "hindsight_reflect",
						"description": "Synthesizes all active memories in a bank into a structured mental model (preferences, lessons, decisions, facts).",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"bank": map[string]interface{}{
									"type":        "string",
									"description": "Memory bank name to reflect upon",
									"default":     "agent",
								},
							},
						},
					},
				},
			},
		}

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32602, Message: "Invalid parameters"},
			}
		}

		switch callParams.Name {
		case "hindsight_retain":
			bank, _ := callParams.Arguments["bank"].(string)
			if bank == "" {
				bank = "agent"
			}
			fTypeStr, _ := callParams.Arguments["type"].(string)
			if fTypeStr == "" {
				fTypeStr = "fact"
			}
			subject, _ := callParams.Arguments["subject"].(string)
			content, _ := callParams.Arguments["content"].(string)

			m, err := store.Retain(bank, FactType(fTypeStr), subject, content)
			if err != nil {
				return errorResult(req.ID, fmt.Sprintf("Retain failed: %v", err))
			}
			msg := fmt.Sprintf("✅ Memória retida com sucesso: [%s] (id=%s, bank=%s, type=%s)", m.Subject, m.ID, m.Bank, m.Type)
			return textResult(req.ID, msg)

		case "hindsight_recall":
			bank, _ := callParams.Arguments["bank"].(string)
			if bank == "" {
				bank = "agent"
			}
			query, _ := callParams.Arguments["query"].(string)
			topK := 5
			if tk, ok := callParams.Arguments["top_k"].(float64); ok && tk > 0 {
				topK = int(tk)
			}
			hits, err := store.Recall(bank, query, topK)
			if err != nil {
				return errorResult(req.ID, fmt.Sprintf("Recall failed: %v", err))
			}
			return textResult(req.ID, FormatRecalled(hits))

		case "hindsight_reflect":
			bank, _ := callParams.Arguments["bank"].(string)
			if bank == "" {
				bank = "agent"
			}
			report, err := store.Reflect(bank)
			if err != nil {
				return errorResult(req.ID, fmt.Sprintf("Reflect failed: %v", err))
			}
			return textResult(req.ID, report)

		default:
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Tool not found: %s", callParams.Name)},
			}
		}

	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		}
	}
}

func textResult(id interface{}, text string) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPToolResult{
			Content: []MCPTextContent{
				{Type: "text", Text: text},
			},
		},
	}
}

func errorResult(id interface{}, msg string) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPToolResult{
			Content: []MCPTextContent{
				{Type: "text", Text: msg},
			},
			IsError: true,
		},
	}
}

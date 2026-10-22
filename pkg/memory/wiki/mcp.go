package wiki

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// MCPRequest represents an incoming JSON-RPC 2.0 request from an MCP host (Antigravity, Cursor, OpenCode).
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

// ServeMCP runs the Model Context Protocol stdio server.
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

		resp := handleMCPRequest(store, req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return err
			}
		}
	}
}

func handleMCPRequest(store *Store, req MCPRequest) *MCPResponse {
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
					"name":    "ai-memory-go",
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
						"name":        "memory_search",
						"description": "Searches the Git-backed project knowledge wiki using high-speed SQLite FTS5 full-text search with BM25 ranking.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"query": map[string]interface{}{
									"type":        "string",
									"description": "Search query keywords (e.g. 'vps bot memory', 'assembly avx2', 'http connection pool')",
								},
								"limit": map[string]interface{}{
									"type":        "integer",
									"description": "Maximum number of results to return (default 5)",
								},
							},
							"required": []string{"query"},
						},
					},
					{
						"name":        "memory_remember",
						"description": "Records a new architectural decision, operational fact, or bot learning into the Git-backed Markdown wiki and reindexes it immediately.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"title": map[string]interface{}{
									"type":        "string",
									"description": "Title of the memory page or note",
								},
								"content": map[string]interface{}{
									"type":        "string",
									"description": "Detailed Markdown content",
								},
								"tags": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
									"description": "Optional tags (e.g. ['vps', 'bot', 'perf'])",
								},
							},
							"required": []string{"title", "content"},
						},
					},
					{
						"name":        "memory_read_page",
						"description": "Reads the full content of a specific markdown knowledge page from .ai-memory/.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"path": map[string]interface{}{
									"type":        "string",
									"description": "Relative path to page (e.g. 'architecture.md', 'stack.md', 'decisions.md')",
								},
							},
							"required": []string{"path"},
						},
					},
					{
						"name":        "memory_handoff_read",
						"description": "Retrieves the active cross-agent handoff packet (what previous agents like Cursor, OpenCode or Antigravity accomplished and what remains to do).",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
					{
						"name":        "memory_handoff_write",
						"description": "Writes a cross-agent handoff packet so subsequent agents (Cursor, Antigravity, OpenCode) can resume your work seamlessly.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"agent": map[string]interface{}{
									"type":        "string",
									"description": "Name of your agent (e.g. 'antigravity', 'cursor', 'opencode')",
								},
								"current_task": map[string]interface{}{
									"type":        "string",
									"description": "High-level goal or task in progress",
								},
								"completed_steps": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
									"description": "Tasks or steps completed in this session",
								},
								"next_steps": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
									"description": "Immediate next steps for the incoming agent",
								},
								"active_files": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
									"description": "Relevant files touched",
								},
								"context_notes": map[string]interface{}{
									"type":        "string",
									"description": "Crucial caveats, architectural rules, or constraints",
								},
							},
							"required": []string{"current_task"},
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
				Error:   &MCPError{Code: -32602, Message: "Invalid arguments format"},
			}
		}

		resultText, isErr := executeTool(store, callParams.Name, callParams.Arguments)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: MCPToolResult{
				Content: []MCPTextContent{
					{Type: "text", Text: resultText},
				},
				IsError: isErr,
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

func executeTool(store *Store, name string, args map[string]interface{}) (string, bool) {
	switch name {
	case "memory_search":
		query, _ := args["query"].(string)
		limit := 5
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		results, err := store.Search(query, limit)
		if err != nil {
			return fmt.Sprintf("Search error: %v", err), true
		}
		if len(results) == 0 {
			return fmt.Sprintf("Nenhum resultado encontrado na wiki para: %q", query), false
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### Resultados na Wiki para: %q (%d encontrados)\n\n", query, len(results)))
		for i, res := range results {
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)** (Score: %.2f)\n", i+1, res.Title, res.Path, res.Score))
			sb.WriteString(fmt.Sprintf("   > %s\n\n", res.Snippet))
		}
		return sb.String(), false

	case "memory_remember":
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		var tags []string
		if rawTags, ok := args["tags"].([]interface{}); ok {
			for _, t := range rawTags {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		}
		relPath, err := store.Remember("", title, content, tags)
		if err != nil {
			return fmt.Sprintf("Falha ao salvar memoria: %v", err), true
		}
		return fmt.Sprintf("✅ Memoria gravada com sucesso em: %s", relPath), false

	case "memory_read_page":
		path, _ := args["path"].(string)
		page, err := store.GetPage(path)
		if err != nil {
			return fmt.Sprintf("Falha ao ler pagina %s: %v", path, err), true
		}
		return page.Content, false

	case "memory_handoff_read":
		packet, err := store.LoadHandoff()
		if err != nil {
			return fmt.Sprintf("Falha ao carregar handoff: %v", err), true
		}
		if packet == nil {
			return "Nenhum pacote de handoff registrado anteriormente.", false
		}
		return RenderHandoffContext(packet), false

	case "memory_handoff_write":
		agent, _ := args["agent"].(string)
		currentTask, _ := args["current_task"].(string)
		notes, _ := args["context_notes"].(string)

		var completed, nextSteps, activeFiles []string
		if c, ok := args["completed_steps"].([]interface{}); ok {
			for _, item := range c {
				if s, ok := item.(string); ok {
					completed = append(completed, s)
				}
			}
		}
		if n, ok := args["next_steps"].([]interface{}); ok {
			for _, item := range n {
				if s, ok := item.(string); ok {
					nextSteps = append(nextSteps, s)
				}
			}
		}
		if a, ok := args["active_files"].([]interface{}); ok {
			for _, item := range a {
				if s, ok := item.(string); ok {
					activeFiles = append(activeFiles, s)
				}
			}
		}

		packet := HandoffPacket{
			Agent:          agent,
			Timestamp:      time.Now(),
			CurrentTask:    currentTask,
			CompletedSteps: completed,
			NextSteps:      nextSteps,
			ActiveFiles:    activeFiles,
			ContextNotes:   notes,
		}

		if err := store.SaveHandoff(packet); err != nil {
			return fmt.Sprintf("Falha ao salvar handoff: %v", err), true
		}
		return fmt.Sprintf("✅ Handoff cross-agent registrado com sucesso por %s!", agent), false

	default:
		return fmt.Sprintf("Ferramenta desconhecida: %s", name), true
	}
}

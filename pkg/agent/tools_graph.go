package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kbelludoo/agent-go/pkg/graph"
)

func init() {
	AvailableTools = append(AvailableTools,
		ToolDef{
			Type: "function",
			Function: FunctionDef{
				Name:        "repo_map",
				Description: "Generates an ultra-compressed AST skeleton of the codebase or directory (signatures, interfaces, structs with function bodies stripped). Provides global structural context with ~90% token reduction.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"dir": {
							"type": "string",
							"description": "Directory path to map (default: '.')"
						},
						"exported_only": {
							"type": "boolean",
							"description": "If true, only exported structs, interfaces and functions are included"
						}
					}
				}`),
			},
		},
		ToolDef{
			Type: "function",
			Function: FunctionDef{
				Name:        "who_calls",
				Description: "Graphify AST call graph lookup in pure Go: queries the codebase to find all caller locations for target function in microseconds (~30 tokens instead of reading entire files).",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"target": {
							"type": "string",
							"description": "Name of the target function or method to find callers for (e.g. 'NewDedicatedIPClient', 'ExecuteTool')"
						},
						"dir": {
							"type": "string",
							"description": "Root directory to scan (default: '.')"
						}
					},
					"required": ["target"]
				}`),
			},
		},
	)
}

func executeGraphTool(ctx context.Context, name, arguments string) (string, bool, error) {
	switch name {
	case "repo_map":
		var args struct {
			Dir          string `json:"dir"`
			ExportedOnly bool   `json:"exported_only"`
		}
		_ = json.Unmarshal([]byte(arguments), &args)
		if args.Dir == "" {
			args.Dir = "."
		}
		opts := graph.DefaultSkeletonOptions()
		opts.ExportedOnly = args.ExportedOnly
		res, err := graph.GenerateRepoMap(args.Dir, opts)
		return res, true, err

	case "who_calls":
		var args struct {
			Target string `json:"target"`
			Dir    string `json:"dir"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", true, fmt.Errorf("invalid arguments for who_calls: %w", err)
		}
		if args.Dir == "" {
			args.Dir = "."
		}
		cg, err := graph.BuildCallGraph(args.Dir)
		if err != nil {
			return fmt.Sprintf("Erro ao construir Call Graph: %v", err), true, nil
		}
		return cg.FormatWhoCalls(args.Target), true, nil

	default:
		return "", false, nil
	}
}

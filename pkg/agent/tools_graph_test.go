package agent

import (
	"context"
	"strings"
	"testing"
)

func TestGraphToolsExecution(t *testing.T) {
	ctx := context.Background()

	// 1. Test who_calls tool
	res, err := ExecuteTool(ctx, "who_calls", `{"target":"ExecuteTool","dir":"."}`)
	if err != nil {
		t.Fatalf("who_calls error: %v", err)
	}
	t.Logf("who_calls result:\n%s", res)
	if !strings.Contains(res, "Chamadas para 'ExecuteTool'") {
		t.Errorf("who_calls output inesperado: %s", res)
	}

	// 2. Test repo_map tool
	repoRes, err := ExecuteTool(ctx, "repo_map", `{"dir":".","exported_only":true}`)
	if err != nil {
		t.Fatalf("repo_map error: %v", err)
	}
	t.Logf("repo_map length: %d chars", len(repoRes))
	if !strings.Contains(repoRes, "type ToolDef struct") {
		t.Errorf("repo_map output não contém ToolDef: %s", repoRes)
	}
}

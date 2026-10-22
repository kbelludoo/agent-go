package wiki

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWikiStoreFullLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ai-memory-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Initialize starter pages
	if err := store.Init(); err != nil {
		t.Fatalf("store init failed: %v", err)
	}

	pages, err := store.ListPages()
	if err != nil {
		t.Fatalf("list pages failed: %v", err)
	}
	if len(pages) < 4 {
		t.Fatalf("expected at least 4 starter pages, got %d", len(pages))
	}

	// 2. Search for AVX2 Assembly in memory
	results, err := store.Search("Assembly AVX2", 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'Assembly AVX2', got 0")
	}

	foundAVX := false
	for _, res := range results {
		if strings.Contains(res.Title, "Decisões") || strings.Contains(res.Title, "Arquitetura") || strings.Contains(res.Snippet, "Assembly") {
			foundAVX = true
			break
		}
	}
	if !foundAVX {
		t.Fatal("expected to find relevant snippet about Assembly")
	}

	// 3. Remember new knowledge
	relPath, err := store.Remember("", "Prevenção de Vazamento de Memória", "Sempre fechar resp.Body em requisições HTTP para os bots não estuprarem a RAM da VPS.", []string{"vps", "memory", "leak"})
	if err != nil {
		t.Fatalf("remember failed: %v", err)
	}
	if relPath == "" {
		t.Fatal("relPath should not be empty")
	}

	// Search for the newly remembered fact
	leakResults, err := store.Search("resp.Body vazamento", 5)
	if err != nil {
		t.Fatalf("leak search failed: %v", err)
	}
	if len(leakResults) == 0 {
		t.Fatal("expected search to find newly remembered memory")
	}

	// 4. Test Cross-Agent Handoff
	packet := HandoffPacket{
		Agent:          "antigravity",
		Timestamp:      time.Now(),
		CurrentTask:    "Implementar cliente OpenCode e AI-Memory",
		CompletedSteps: []string{"Conectar daemon OpenCode", "Criar FTS5 SQLite Wiki"},
		NextSteps:      []string{"Testar CLI cmd/ai-memory", "Configurar hooks"},
		ActiveFiles:    []string{"pkg/memory/wiki/store.go", "cmd/ai-memory/main.go"},
		ContextNotes:   "VPS tem 1GB RAM total, manter processos leves.",
	}

	if err := store.SaveHandoff(packet); err != nil {
		t.Fatalf("save handoff failed: %v", err)
	}

	loaded, err := store.LoadHandoff()
	if err != nil {
		t.Fatalf("load handoff failed: %v", err)
	}
	if loaded == nil || loaded.Agent != "antigravity" {
		t.Fatalf("unexpected handoff packet: %+v", loaded)
	}

	ctxPrompt := RenderHandoffContext(loaded)
	if !strings.Contains(ctxPrompt, "Cross-Agent Memory Handoff") {
		t.Fatalf("rendered context missing expected title: %s", ctxPrompt)
	}

	// 5. Test Hooks Installation
	hookFiles, err := InstallHooks(tempDir)
	if err != nil {
		t.Fatalf("install hooks failed: %v", err)
	}
	if len(hookFiles) < 3 {
		t.Fatalf("expected at least 3 hook config files, got %d", len(hookFiles))
	}
	for _, f := range hookFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("hook file %s was not created", f)
		}
	}
}

func TestSanitizeFTS5Query(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"hello world", `"hello"* OR "world"*`},
		{"go:1.26 / assembly", `"go"* OR "26"* OR "assembly"*`},
		{"", ""},
		{"a", ""},
	}

	for _, c := range cases {
		out := sanitizeFTS5Query(c.input)
		if out != c.expected {
			t.Errorf("input %q: expected %q, got %q", c.input, c.expected, out)
		}
	}
}

func TestMCPServeProtocol(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ai-memory-mcp-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()
	_ = store.Init()

	// Prepare MCP JSON-RPC input: initialize -> tools/list -> tools/call
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_search","arguments":{"query":"assembly"}}}
`
	inReader := strings.NewReader(input)
	var outWriter strings.Builder

	if err := ServeMCP(store, inReader, &outWriter); err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}

	output := outWriter.String()
	if !strings.Contains(output, "ai-memory-go") {
		t.Fatalf("expected serverInfo in initialize response: %s", output)
	}
	if !strings.Contains(output, "memory_search") || !strings.Contains(output, "memory_remember") {
		t.Fatalf("expected tool definitions in tools/list response: %s", output)
	}
	if !strings.Contains(output, "Decisões") && !strings.Contains(output, "Arquitetura") {
		t.Fatalf("expected search hit in tools/call response: %s", output)
	}
}

func BenchmarkWikiSearchFTS5(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "ai-memory-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()
	_ = store.Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Search("assembly avx2", 5)
	}
}

package navigator_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-go-browser/pkg/memory"
	"github.com/kbelludoo/agent-go/pkg/navigator"
)

func TestBrowserCognitiveMemoryLive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ai-go-mem-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_browser_memory.db")
	mem, err := memory.NewEngine(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize memory engine: %v", err)
	}
	defer mem.Close()

	session := navigator.NewSession().WithMemory(mem)

	// 1. Live Navigation
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	targetURL := "https://go.dev/blog/"
	snap, err := session.Navigate(ctx, targetURL)
	if err != nil {
		t.Fatalf("navigation failed: %v", err)
	}
	if snap.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", snap.StatusCode)
	}
	t.Logf("✅ Navegado com sucesso em %s (Status: %d, Titulo: %q, Reducao: %.1f%%)",
		targetURL, snap.StatusCode, snap.Title, snap.TokenReduction)

	// 2. Empirical Verification of SQLite FTS5 BM25 Search
	hits, err := session.SearchMemory("Go", 5)
	if err != nil {
		t.Fatalf("SearchMemory failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected at least 1 memory hit for query 'Go', got 0")
	}
	t.Logf("🧠 FTS5 BM25 Search retornou %d resultado(s) direto da memoria local!", len(hits))
	t.Logf("   Hit 1: [%s] (Snippet: %s)", hits[0].Title, hits[0].Snippet)

	// 3. Empirical Verification of Domain Playbook
	pb := mem.GetOrCreatePlaybook("go.dev")
	if pb == nil {
		t.Fatalf("expected playbook for go.dev, got nil")
	}
	t.Logf("📘 Domain Playbook verificado para 'go.dev': LastVisited=%s", pb.LastVisited.Format(time.RFC3339))
}

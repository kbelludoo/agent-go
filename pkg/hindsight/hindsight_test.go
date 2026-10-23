package hindsight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "hindsight_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "hindsight.db")
	store, err := Open(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}
	return store, cleanup
}

func TestRetainAndSupersede(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Inserir fato inicial sobre a porta do Huntera
	m1, err := store.Retain("vps", TypeFact, "bot_huntera_port", "O bot Huntera escuta na porta 3000")
	if err != nil {
		t.Fatalf("Retain failed: %v", err)
	}
	if m1.ID == "" {
		t.Fatalf("expected non-empty ID")
	}

	// 2. Recall do fato
	hits, err := store.Recall("vps", "porta do bot Huntera", 5)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if !strings.Contains(hits[0].Memory.Content, "3000") {
		t.Fatalf("unexpected content: %s", hits[0].Memory.Content)
	}

	// 3. Modificar a porta para 3020 com o mesmo subject -> m1 deve ser superada (superseded)
	m2, err := store.Retain("vps", TypeFact, "bot_huntera_port", "O bot Huntera foi movido para a porta 3020")
	if err != nil {
		t.Fatalf("Retain update failed: %v", err)
	}
	if m2.ID == m1.ID {
		t.Fatalf("expected new ID for updated memory")
	}

	// 4. Recall deve trazer APENAS a memória nova (3020), sem conflito
	hits2, err := store.Recall("vps", "Huntera porta", 5)
	if err != nil {
		t.Fatalf("Recall after update failed: %v", err)
	}
	if len(hits2) != 1 {
		t.Fatalf("expected exactly 1 active hit after supersede, got %d", len(hits2))
	}
	if !strings.Contains(hits2[0].Memory.Content, "3020") {
		t.Fatalf("expected active memory to have 3020, got: %s", hits2[0].Memory.Content)
	}
}

func TestReflectMentalModel(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, _ = store.Retain("agent", TypePreference, "tech_stack", "Sempre utilizar Go puro nativo sem CGO")
	_, _ = store.Retain("agent", TypeLesson, "vps_tmpfs", "Compilações na VPS devem usar GOTMPDIR no disco físico para não lotar tmpfs")
	_, _ = store.Retain("agent", TypeDecision, "token_policy", "Nunca repetir dados completos se puder usar diff ou TOON")

	report, err := store.Reflect("agent")
	if err != nil {
		t.Fatalf("Reflect failed: %v", err)
	}

	if !strings.Contains(report, "PREFERÊNCIAS E DIRETRIZES") {
		t.Fatalf("missing preferences header: %s", report)
	}
	if !strings.Contains(report, "Sempre utilizar Go puro") {
		t.Fatalf("missing preference content: %s", report)
	}
	if !strings.Contains(report, "LIÇÕES APRENDIDAS") {
		t.Fatalf("missing lessons header: %s", report)
	}
}

func BenchmarkRetain(b *testing.B) {
	dir, err := os.MkdirTemp("", "bench_hindsight_*")
	if err != nil {
		b.Fatalf("failed temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := Open(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("failed open: %v", err)
	}
	defer store.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Retain("bench", TypeFact, "bench_metric", "Operação de gravação contínua em SQLite WAL")
	}
}

func BenchmarkRecall(b *testing.B) {
	dir, err := os.MkdirTemp("", "bench_recall_*")
	if err != nil {
		b.Fatalf("failed temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := Open(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("failed open: %v", err)
	}
	defer store.Close()

	for i := 0; i < 50; i++ {
		_, _ = store.Retain("bench", TypeFact, "", "Fato de teste sobre arquitetura de sistemas distribuídos e bots")
	}
	_, _ = store.Retain("bench", TypeLesson, "target_key", "Esta é a lição crucial sobre recuperação de memória Hindsight em Go")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Recall("bench", "lição crucial recuperação Hindsight", 3)
	}
}

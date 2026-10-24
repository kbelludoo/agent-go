package hindsight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMBVMStore_RetainRecallSlice(t *testing.T) {
	dir, err := os.MkdirTemp("", "mbvm_store_test_*")
	if err != nil {
		t.Fatalf("failed temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	filePath := filepath.Join(dir, "memory.mbvm")
	store, err := OpenMBVM(filePath)
	if err != nil {
		t.Fatalf("failed OpenMBVM: %v", err)
	}
	defer store.Close()

	// 1. Inserir fatos
	m1, err := store.Retain("vps", TypeFact, "bot_huntera", "Huntera roda na porta 3000 com latencia de 1.7ms")
	if err != nil {
		t.Fatalf("Retain failed: %v", err)
	}
	if m1.ID == "" {
		t.Fatalf("expected non-empty ID")
	}

	_, err = store.Retain("vps", TypeLesson, "vps_tmpfs", "Sempre usar GOTMPDIR em disco fisico para builds Go na VPS")
	if err != nil {
		t.Fatalf("Retain lesson failed: %v", err)
	}

	// 2. Recall com busca BM25
	hits, err := store.Recall("vps", "onde roda o Huntera", 2)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits, got 0")
	}
	if !strings.Contains(hits[0].Memory.Content, "3000") {
		t.Fatalf("expected 3000 in recalled memory, got %s", hits[0].Memory.Content)
	}

	// 3. Smart Slicing de Memória (@MBVM:memory_slice)
	sliced := store.Slice("vps", "bot_huntera", 1)
	if !strings.Contains(sliced, "@MBVM:memory_slice") {
		t.Fatalf("expected @MBVM:memory_slice header, got:\n%s", sliced)
	}
	if !strings.Contains(sliced, "Huntera roda na porta 3000") {
		t.Fatalf("expected Huntera in slice, got:\n%s", sliced)
	}

	// 4. Fechar e reabrir para validar persistência binária atômica e CRC32
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	storeReopened, err := OpenMBVM(filePath)
	if err != nil {
		t.Fatalf("OpenMBVM reload failed: %v", err)
	}
	defer storeReopened.Close()

	reflectReport, err := storeReopened.Reflect("vps")
	if err != nil {
		t.Fatalf("Reflect failed: %v", err)
	}
	if !strings.Contains(reflectReport, "GOTMPDIR em disco fisico") {
		t.Fatalf("expected reloaded lesson in reflect report, got:\n%s", reflectReport)
	}
}

func BenchmarkMBVMStore_Retain(b *testing.B) {
	dir, err := os.MkdirTemp("", "bench_mbvm_*")
	if err != nil {
		b.Fatalf("failed temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := OpenMBVM(filepath.Join(dir, "bench.mbvm"))
	if err != nil {
		b.Fatalf("failed OpenMBVM: %v", err)
	}
	defer store.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Retain("bench", TypeFact, "bench_sym", "Operacao em micro-bytecode MBVM-DB com intern de strings")
	}
}

func BenchmarkMBVMStore_Recall(b *testing.B) {
	dir, err := os.MkdirTemp("", "bench_mbvm_recall_*")
	if err != nil {
		b.Fatalf("failed temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := OpenMBVM(filepath.Join(dir, "bench.mbvm"))
	if err != nil {
		b.Fatalf("failed OpenMBVM: %v", err)
	}
	defer store.Close()

	for i := 0; i < 50; i++ {
		_, _ = store.Retain("bench", TypeFact, "", "Fato de observacao em micro-bytecode")
	}
	_, _ = store.Retain("bench", TypeLesson, "target_lesson", "Lição de ouro sobre compressao extrema e subgrafo de memoria")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Recall("bench", "compressao extrema subgrafo", 3)
	}
}

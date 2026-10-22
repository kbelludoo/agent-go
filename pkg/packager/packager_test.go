package packager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackagerFunctionality(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "main.go")
	src1 := `package main

// This is a test comment that should be pruned
import "fmt"

func main() {


	// Another comment
	fmt.Println("hello world")
}
`
	if err := os.WriteFile(file1, []byte(src1), 0644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultPackOptions(tmpDir)
	res, err := Pack(opts)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	if res.FilesCount != 1 {
		t.Fatalf("Esperava 1 arquivo, obteve %d", res.FilesCount)
	}

	if strings.Contains(res.Content, "This is a test comment") {
		t.Error("Comentário não foi podado")
	}

	if !strings.Contains(res.Content, `fmt.Println("hello world")`) {
		t.Error("Código real foi perdido")
	}

	t.Logf("Bytes originais: %d -> Bytes empacotados: %d (Redução: %.1f%%)",
		res.OriginalBytes, res.PackedBytes, (1.0-res.CompressionRatio)*100)
}

func TestPackagerRealBenchmark(t *testing.T) {
	root := `../../pkg`
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("pkg root not found")
	}

	start := time.Now()
	opts := DefaultPackOptions(root)
	res, err := Pack(opts)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] %d arquivos empacotados em: %v", res.FilesCount, duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Original: %d bytes -> Compactado: %d bytes (~%d tokens)",
		res.OriginalBytes, res.PackedBytes, res.EstimatedTokens)
	t.Logf("📉 [TAXA DE COMPRESSÃO REAL] %.2f%% de redução direta", (1.0-res.CompressionRatio)*100)
}

func BenchmarkPackager(b *testing.B) {
	root := `../../pkg`
	opts := DefaultPackOptions(root)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Pack(opts)
	}
}

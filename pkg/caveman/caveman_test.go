package caveman

import (
	"strings"
	"testing"
	"time"
)

func TestStripProlixity(t *testing.T) {
	sample := `Certainly! Here is the updated implementation you requested:

func Add(a, b int) int {
	return a + b
}

Hope this helps! Let me know if you have any questions or need further assistance.`

	start := time.Now()
	cleaned := StripProlixity(sample)
	duration := time.Since(start)

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Caveman filter executado em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Original: %d chars -> Limpo: %d chars (Economia: %.1f%%)",
		len(sample), len(cleaned), float64(len(sample)-len(cleaned))/float64(len(sample))*100.0)
	t.Logf("Texto Pós-Caveman:\n%s", cleaned)

	// Verifications
	if strings.Contains(cleaned, "Certainly!") {
		t.Error("Preâmbulo não foi removido!")
	}
	if strings.Contains(cleaned, "Hope this helps!") {
		t.Error("Epílogo não foi removido!")
	}
	if !strings.Contains(cleaned, "func Add(a, b int) int") {
		t.Error("Código principal foi corrompido!")
	}
}

func BenchmarkStripProlixity(b *testing.B) {
	sample := `Sure thing! Here is the code:
func Hello() { println("world") }
Hope this helps!`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StripProlixity(sample)
	}
}

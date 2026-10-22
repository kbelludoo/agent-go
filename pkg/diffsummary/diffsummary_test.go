package diffsummary

import (
	"strings"
	"testing"
	"time"
)

func TestSummarizeDiff(t *testing.T) {
	rawDiff := `diff --git a/pkg/agent/llm.go b/pkg/agent/llm.go
index 123456..789abc 100644
--- a/pkg/agent/llm.go
+++ b/pkg/agent/llm.go
@@ -550,6 +550,15 @@ func (c *LLMClient) Complete() {
-	oldClient := http.DefaultClient
+	newClient := network.NewDedicatedIPClient(prov.LocalIP)
+	return newClient.Do(req)
 }
+
+func NewDedicatedHelper() string {
+	return "ok"
+}
diff --git a/pkg/network/binding.go b/pkg/network/binding.go
new file mode 100644
index 000000..def456
--- /dev/null
+++ b/pkg/network/binding.go
@@ -0,0 +1,20 @@
+package network
+
+type TokenBinding struct {
+	Token string
+}
+
+func ParseTokenIPBindings() {}
`

	start := time.Now()
	res := SummarizeDiff(rawDiff)
	duration := time.Since(start)

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Diff resumido em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Diff Original: %d chars (~%d tokens) -> Resumo: %d chars (~%d tokens)",
		len(rawDiff), len(rawDiff)/4, len(res.SummaryText), res.EstimatedTokens)
	t.Logf("Resumo Estrutural Gerado:\n%s", res.SummaryText)

	// Verifications
	if res.TotalFiles != 2 {
		t.Fatalf("Esperava 2 arquivos, obteve %d", res.TotalFiles)
	}
	if !strings.Contains(res.SummaryText, "CRIADO] pkg/network/binding.go") {
		t.Error("Arquivo criado não foi detectado!")
	}
	if !strings.Contains(res.SummaryText, "func NewDedicatedHelper") {
		t.Error("Função adicionada não foi detectada!")
	}
}

func BenchmarkSummarizeDiff(b *testing.B) {
	sample := `diff --git a/main.go b/main.go
index 1..2 100644
--- a/main.go
+++ b/main.go
@@ -10,3 +10,5 @@ func main() {
+	newFeature()
+}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SummarizeDiff(sample)
	}
}

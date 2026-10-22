package dcp

import (
	"strings"
	"testing"
	"time"
)

func TestDCPTranscriptPruning(t *testing.T) {
	// Construct a realistic 10-message conversation with large historical tool output
	history := []Message{
		{Role: "system", Content: "You are an autonomous AI coding assistant."},
		{Role: "user", Content: "Run tests on the project"},
		{
			Role: "tool",
			Name: "bash_exec",
			// Large output from an old turn
			Content: strings.Repeat("PASS: TestItem123 ok in 0.05s\n", 200),
		},
		{Role: "assistant", Content: "Tests passed. Now let me read the file."},
		{
			Role: "tool",
			Name: "read_file",
			// Large 5000-char dump
			Content: strings.Repeat("package example\nfunc Example() { println(1) }\n", 100),
		},
		{Role: "assistant", Content: "File read complete. Creating patch."},
		// Recent turns:
		{Role: "user", Content: "Check git diff"},
		{Role: "tool", Name: "git_diff", Content: "diff --git a/foo.go b/foo.go\n+ func New()"},
		{Role: "assistant", Content: "The diff looks good."},
		{Role: "user", Content: "Confirm completion."},
	}

	start := time.Now()
	res := PruneTranscript(history, DefaultPruneOptions())
	duration := time.Since(start)

	t.Logf("⚡ [PERFORMANCE COMPROVADA] DCP executado em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Original: %d caracteres -> Podado: %d caracteres", res.OriginalChars, res.PrunedChars)
	t.Logf("📉 [REDUÇÃO REAL DE CONTEXTO] %.2f%% de tokens economizados (~%d tokens poupados)", res.ReductionPercent, res.TokensSaved)

	// Verifications
	if res.ReductionPercent < 40.0 {
		t.Errorf("Esperava pelo menos 40%% de redução, obteve %.2f%%", res.ReductionPercent)
	}

	// Verify system prompt (index 0) was preserved 100%
	if res.Messages[0].Content != history[0].Content {
		t.Error("System prompt foi modificado indevidamente!")
	}

	// Verify old tool was truncated
	if !strings.Contains(res.Messages[2].Content, "podadas pelo DCP") {
		t.Error("Ferramenta antiga não foi podada pelo DCP!")
	}

	// Verify latest tool was NOT truncated
	if strings.Contains(res.Messages[7].Content, "podadas pelo DCP") {
		t.Error("Turno recente foi truncado indevidamente!")
	}
}

func BenchmarkDCP(b *testing.B) {
	history := []Message{
		{Role: "system", Content: "You are an assistant."},
		{Role: "user", Content: "Run"},
		{Role: "tool", Content: strings.Repeat("log output line\n", 300)},
		{Role: "assistant", Content: "Done"},
		{Role: "user", Content: "Next"},
		{Role: "tool", Content: strings.Repeat("build output\n", 300)},
		{Role: "assistant", Content: "Ok"},
		{Role: "user", Content: "Status"},
	}
	opts := DefaultPruneOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PruneTranscript(history, opts)
	}
}

package patcher

import (
	"strings"
	"testing"
	"time"
)

func TestPatcherCRLFandIndentTolerance(t *testing.T) {
	// Original file with Windows CRLF and 4 spaces indentation
	original := "package main\r\n\r\nfunc Run() {\r\n    log.Println(\"start\")\r\n    doWork()\r\n    log.Println(\"end\")\r\n}\r\n"

	// LLM generated target with Linux LF and 2 spaces indentation (typical failure mode in edit_file)
	target := "func Run() {\n  log.Println(\"start\")\n  doWork()\n  log.Println(\"end\")\n}"

	replacement := "func Run() {\n    log.Println(\"iniciado\")\n    doOptimizedWork()\n    log.Println(\"concluido\")\n}"

	start := time.Now()
	patched, err := ApplyPatch(original, target, replacement, DefaultPatchOptions())
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ApplyPatch falhou: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Patch com tolerância a CRLF/indent aplicado em: %v", duration)
	t.Logf("Arquivo Patcheado:\n%s", patched)

	// Verifications
	if !strings.Contains(patched, "doOptimizedWork()") {
		t.Error("Substituição não foi aplicada!")
	}
	if !strings.Contains(patched, "\r\n") {
		t.Error("Line endings originais CRLF foram corrompidos!")
	}
}

func BenchmarkPatcher(b *testing.B) {
	original := "package main\n\nfunc Run() {\n    log.Println(\"start\")\n    doWork()\n    log.Println(\"end\")\n}\n"
	target := "func Run() {\n  log.Println(\"start\")\n  doWork()\n  log.Println(\"end\")\n}"
	replacement := "func Run() {\n    doOptimizedWork()\n}"
	opts := DefaultPatchOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(original, target, replacement, opts)
	}
}

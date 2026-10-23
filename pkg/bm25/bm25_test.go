package bm25

import (
	"fmt"
	"testing"
)

func TestBM25Search(t *testing.T) {
	docs := map[string]string{
		"pkg/vault/vault.go":         "package vault\nfunc EncryptAES256GCM(plaintext []byte, passphrase string) ([]byte, error) { return nil, nil }",
		"pkg/scraper/scraper.go":     "package scraper\nfunc ScrapeHTMLToMarkdown(url string) string { return \"\" }",
		"pkg/sanitizer/sanitizer.go": "package sanitizer\nfunc RedactSecretKeys(input string) string { return \"\" }",
		"pkg/graph/callgraph.go":     "package graph\nfunc BuildASTCallGraph(dir string) error { return nil }",
	}

	idx := NewIndex(docs)
	hits := idx.Search("encrypt aes passphrase", 2)
	if len(hits) == 0 {
		t.Fatalf("expected at least 1 hit")
	}
	if hits[0].ID != "pkg/vault/vault.go" {
		t.Fatalf("expected top hit pkg/vault/vault.go, got %s", hits[0].ID)
	}
}

func BenchmarkBM25Search(b *testing.B) {
	docs := make(map[string]string, 50)
	for i := 0; i < 50; i++ {
		docs[fmt.Sprintf("pkg/mod%d/file.go", i)] = fmt.Sprintf(
			"package mod%d\n// Handler for service %d\nfunc ProcessRequest%d(payload string) error { return nil }", i, i, i)
	}
	docs["pkg/target/auth.go"] = "package target\nfunc VerifyJWTTokenSignature(token string, secretKey []byte) bool { return true }"

	idx := NewIndex(docs)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Search("verify jwt token signature", 3)
	}
}

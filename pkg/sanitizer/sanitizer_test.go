package sanitizer

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizerRedactionAndRestore(t *testing.T) {
	redactor := NewRedactor()

	rawText := `
OPENAI_API_KEY=sk-proj-abc12345678901234567890
GITHUB_TOKEN=ghp_123456789012345678901234567890123456
auth_token: "mySuperSecretPassword123"
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0
SSH_KEY:
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Y1u4tq...fakekey...
-----END RSA PRIVATE KEY-----
`

	start := time.Now()
	redacted := redactor.Redact(rawText)
	duration := time.Since(start)

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Sanitização realizada em: %v", duration)
	t.Logf("Texto Redigido:\n%s", redacted)

	// Verifications
	if strings.Contains(redacted, "sk-proj-abc12345678901234567890") {
		t.Error("OpenAI API Key não foi redigida!")
	}
	if strings.Contains(redacted, "ghp_123456789012345678901234567890123456") {
		t.Error("GitHub Token não foi redigido!")
	}
	if strings.Contains(redacted, "mySuperSecretPassword123") {
		t.Error("Password não foi redigido!")
	}
	if strings.Contains(redacted, "BEGIN RSA PRIVATE KEY") {
		t.Error("Chave privada não foi redigida!")
	}

	// Test restoration
	restored := redactor.Restore(redacted)
	if !strings.Contains(restored, "sk-proj-abc12345678901234567890") {
		t.Error("Falha na restauração da chave original!")
	}
}

func BenchmarkSanitizerRedact(b *testing.B) {
	redactor := NewRedactor()
	sample := `config: apiKey="sk-abcdef123456789012345678" and token="ghp_098765432109876543210987654321098765"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redactor.Redact(sample)
	}
}

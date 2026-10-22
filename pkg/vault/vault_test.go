package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVaultEncryptionAndTamperDetection(t *testing.T) {
	secretPayload := []byte("AGENTROUTER_API_KEY=sk-test-secret-123456\nDATABASE_PASSWORD=supersecure999")
	password := "MasterVaultKey2026!"

	start := time.Now()
	ciphertext, err := Encrypt(secretPayload, password)
	encDuration := time.Since(start)

	if err != nil {
		t.Fatalf("Encrypt falhou: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Encriptação AES-256-GCM em: %v (tamanho: %d bytes)", encDuration, len(ciphertext))

	// Decrypt
	startDec := time.Now()
	decrypted, err := Decrypt(ciphertext, password)
	decDuration := time.Since(startDec)

	if err != nil {
		t.Fatalf("Decrypt falhou: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Decriptação AES-256-GCM em: %v", decDuration)

	if !bytes.Equal(secretPayload, decrypted) {
		t.Fatal("Payload decriptado não confere com o original!")
	}

	// Tamper Test (alterando 1 byte no ciphertext)
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, tamperErr := Decrypt(tampered, password)
	if tamperErr == nil {
		t.Fatal("Esperava falha de autenticação ao detectar dado adulterado!")
	}
	t.Logf("🛡️ [SEGURANÇA COMPROVADA] Adulteração detectada com sucesso pelo Auth Tag: %v", tamperErr)

	// Test In-Memory Env Loading
	vaultFile := filepath.Join(t.TempDir(), ".env.vault")
	if err := os.WriteFile(vaultFile, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadVaultIntoEnv(vaultFile, password)
	if err != nil {
		t.Fatalf("LoadVaultIntoEnv falhou: %v", err)
	}

	if loaded != 2 {
		t.Fatalf("Esperava 2 variáveis carregadas, obteve %d", loaded)
	}

	if os.Getenv("AGENTROUTER_API_KEY") != "sk-test-secret-123456" {
		t.Error("Variável carregada na memória não confere!")
	}
}

func BenchmarkVaultEncryption(b *testing.B) {
	data := []byte("API_KEY=sk-123456789012345678901234567890\nDB_PASS=secret")
	pass := "StrongPassphrase123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Encrypt(data, pass)
	}
}

func BenchmarkVaultDecryption(b *testing.B) {
	data := []byte("API_KEY=sk-123456789012345678901234567890\nDB_PASS=secret")
	pass := "StrongPassphrase123"
	cipher, _ := Encrypt(data, pass)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decrypt(cipher, pass)
	}
}

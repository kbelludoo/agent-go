package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Encrypt encrypts plaintext using AES-256-GCM derived from passphrase.
func Encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}

	key := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}

	// Output format: [NONCE 12 bytes] + [CIPHERTEXT + 16 bytes AUTH TAG]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM ciphertext using passphrase.
func Decrypt(ciphertext []byte, passphrase string) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}

	key := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, errors.New("decryption failed: invalid passphrase or corrupted/tampered data")
	}

	return plaintext, nil
}

// LoadVaultIntoEnv decrypts vaultPath and loads all KEY=VALUE pairs into os.Setenv without writing to disk.
func LoadVaultIntoEnv(vaultPath, passphrase string) (int, error) {
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		return 0, fmt.Errorf("read vault file: %w", err)
	}

	decrypted, err := Decrypt(data, passphrase)
	if err != nil {
		return 0, err
	}

	count := 0
	lines := strings.Split(string(decrypted), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			os.Setenv(k, v)
			count++
		}
	}

	return count, nil
}

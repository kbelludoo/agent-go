package schemamcp

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestInspectSQLiteSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			email TEXT UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE INDEX idx_orders_user ON orders(user_id);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	start := time.Now()
	schema, err := InspectSQLiteSchema(dbPath)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("InspectSQLiteSchema falhou: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Schema extraído em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Schema completo formatado em: %d caracteres (~%d tokens)",
		len(schema.Markdown), schema.EstimatedTokens)
	t.Logf("Markdown do Schema:\n%s", schema.Markdown)

	// Verifications
	if len(schema.Tables) != 2 {
		t.Fatalf("Esperava 2 tabelas, obteve %d", len(schema.Tables))
	}
	if !strings.Contains(schema.Markdown, "Tabela `users`") {
		t.Error("Tabela users ausente no Markdown!")
	}
	if !strings.Contains(schema.Markdown, "idx_orders_user") {
		t.Error("Índice idx_orders_user ausente no Markdown!")
	}
}

func BenchmarkInspectSQLiteSchema(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		b.Fatal(err)
	}
	_, _ = db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);`)
	db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = InspectSQLiteSchema(dbPath)
	}
}

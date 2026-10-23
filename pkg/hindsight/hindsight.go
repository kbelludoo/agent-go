package hindsight

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kbelludoo/agent-go/pkg/bm25"
	_ "modernc.org/sqlite"
)

// FactType categorizes atomic memory observations.
type FactType string

const (
	TypeFact       FactType = "fact"       // Objective facts (e.g., "port is 3000")
	TypePreference FactType = "preference" // User preferences (e.g., "always pure Go")
	TypeLesson     FactType = "lesson"     // Learned lessons / error avoidances
	TypeDecision   FactType = "decision"   // Architectural or code decisions
)

// Memory represents a stored atomic unit of hindsight memory.
type Memory struct {
	ID         string    `json:"id"`
	Bank       string    `json:"bank"`
	Type       FactType  `json:"type"`
	Subject    string    `json:"subject"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	AccessCount int      `json:"access_count"`
	Superseded bool      `json:"superseded"`
	SupersededBy string  `json:"superseded_by,omitempty"`
}

// ScoredMemory pairs a recalled memory with its composite relevance score.
type ScoredMemory struct {
	Memory Memory
	Score  float64
}

// Store is the lightweight pure-Go Hindsight engine.
type Store struct {
	dbPath string
	db     *sql.DB
	mu     sync.RWMutex
}

// Open initializes or connects to an embedded Hindsight SQLite database.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	// High concurrency pragmas for embedded SQLite
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed executing pragma %s: %w", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		bank TEXT NOT NULL,
		type TEXT NOT NULL,
		subject TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		access_count INTEGER NOT NULL DEFAULT 0,
		superseded INTEGER NOT NULL DEFAULT 0,
		superseded_by TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_memories_bank_active ON memories(bank, superseded);
	CREATE INDEX IF NOT EXISTS idx_memories_subject ON memories(subject);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed creating schema: %w", err)
	}

	return &Store{
		dbPath: dbPath,
		db:     db,
	}, nil
}

// Close gracefully closes the underlying store.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Retain stores a new observation. If a previous active memory shares the same bank and subject,
// and represents a conflicting/updated state, the old memory is automatically marked as superseded.
func (s *Store) Retain(bank string, fType FactType, subject, content string) (Memory, error) {
	if bank == "" {
		bank = "default"
	}
	if fType == "" {
		fType = TypeFact
	}
	subject = strings.TrimSpace(subject)
	content = strings.TrimSpace(content)
	if content == "" {
		return Memory{}, fmt.Errorf("memory content cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%d", bank, fType, subject, content, now.UnixNano())
	hash := sha256.Sum256([]byte(hashInput))
	newID := hex.EncodeToString(hash[:8])

	tx, err := s.db.Begin()
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()

	// Check if an active memory exists with exact same subject in this bank.
	// If subject is distinct and non-empty, mark old memories for this subject as superseded.
	if subject != "" {
		_, err := tx.Exec(`
			UPDATE memories 
			SET superseded = 1, superseded_by = ?, updated_at = ?
			WHERE bank = ? AND subject = ? AND superseded = 0 AND id != ?
		`, newID, now.Unix(), bank, subject, newID)
		if err != nil {
			return Memory{}, fmt.Errorf("failed superseding old memory: %w", err)
		}
	}

	_, err = tx.Exec(`
		INSERT INTO memories (id, bank, type, subject, content, created_at, updated_at, access_count, superseded, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, '')
	`, newID, bank, string(fType), subject, content, now.Unix(), now.Unix())
	if err != nil {
		return Memory{}, fmt.Errorf("failed inserting memory: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Memory{}, err
	}

	return Memory{
		ID:         newID,
		Bank:       bank,
		Type:       fType,
		Subject:    subject,
		Content:    content,
		CreatedAt:  now,
		UpdatedAt:  now,
		AccessCount: 0,
		Superseded: false,
	}, nil
}

// Recall performs a hybrid lexical BM25 + recency retrieval over active memories in a bank.
func (s *Store) Recall(bank, query string, topK int) ([]ScoredMemory, error) {
	if bank == "" {
		bank = "default"
	}
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	rows, err := s.db.Query(`
		SELECT id, bank, type, subject, content, created_at, updated_at, access_count, superseded, superseded_by
		FROM memories
		WHERE bank = ? AND superseded = 0
	`, bank)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("query active memories failed: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	docsMap := make(map[string]string)

	for rows.Next() {
		var m Memory
		var cAt, uAt int64
		var tStr string
		var sup int
		if err := rows.Scan(&m.ID, &m.Bank, &tStr, &m.Subject, &m.Content, &cAt, &uAt, &m.AccessCount, &sup, &m.SupersededBy); err != nil {
			s.mu.RUnlock()
			return nil, err
		}
		m.Type = FactType(tStr)
		m.CreatedAt = time.Unix(cAt, 0)
		m.UpdatedAt = time.Unix(uAt, 0)
		m.Superseded = (sup == 1)
		memories = append(memories, m)

		// Full searchable text: subject + content + type
		docsMap[m.ID] = fmt.Sprintf("%s %s %s", m.Subject, m.Type, m.Content)
	}
	s.mu.RUnlock()

	if len(memories) == 0 {
		return nil, nil
	}

	// 1. BM25 Lexical scoring
	idx := bm25.NewIndex(docsMap)
	bm25Hits := idx.Search(query, len(memories))
	scoreMap := make(map[string]float64, len(bm25Hits))
	for _, h := range bm25Hits {
		scoreMap[h.ID] = h.Score
	}

	// 2. Composite Score: BM25 score + Recency Boost
	now := time.Now()
	var scored []ScoredMemory

	for _, m := range memories {
		baseScore := scoreMap[m.ID]
		if baseScore <= 0 {
			// Substring fallback
			qLower := strings.ToLower(query)
			if strings.Contains(strings.ToLower(m.Subject), qLower) || strings.Contains(strings.ToLower(m.Content), qLower) {
				baseScore = 1.0
			} else {
				continue
			}
		}

		// Recency boost: half-life decay of 30 days
		daysOld := now.Sub(m.UpdatedAt).Hours() / 24.0
		recencyFactor := math.Exp(-daysOld / 30.0) // 1.0 down to ~0.37 after 30 days
		finalScore := baseScore * (1.0 + 0.2*recencyFactor)

		scored = append(scored, ScoredMemory{
			Memory: m,
			Score:  finalScore,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > topK {
		scored = scored[:topK]
	}

	// Update access count asynchronously in background
	if len(scored) > 0 {
		go s.bumpAccessCounts(scored)
	}

	return scored, nil
}

func (s *Store) bumpAccessCounts(items []ScoredMemory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range items {
		_, _ = s.db.Exec(`UPDATE memories SET access_count = access_count + 1 WHERE id = ?`, it.Memory.ID)
	}
}

// Reflect synthesizes all active memories in a bank into a compact mental model report.
func (s *Store) Reflect(bank string) (string, error) {
	if bank == "" {
		bank = "default"
	}

	s.mu.RLock()
	rows, err := s.db.Query(`
		SELECT id, bank, type, subject, content, created_at, updated_at, access_count, superseded, superseded_by
		FROM memories
		WHERE bank = ? AND superseded = 0
		ORDER BY type ASC, updated_at DESC
	`, bank)
	if err != nil {
		s.mu.RUnlock()
		return "", err
	}
	defer rows.Close()

	byType := make(map[FactType][]Memory)
	total := 0
	for rows.Next() {
		var m Memory
		var cAt, uAt int64
		var tStr string
		var sup int
		if err := rows.Scan(&m.ID, &m.Bank, &tStr, &m.Subject, &m.Content, &cAt, &uAt, &m.AccessCount, &sup, &m.SupersededBy); err != nil {
			s.mu.RUnlock()
			return "", err
		}
		m.Type = FactType(tStr)
		m.CreatedAt = time.Unix(cAt, 0)
		m.UpdatedAt = time.Unix(uAt, 0)
		byType[m.Type] = append(byType[m.Type], m)
		total++
	}
	s.mu.RUnlock()

	if total == 0 {
		return fmt.Sprintf("Nenhuma memória ativa registrada no banco '%s'.", bank), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "=== MODELO MENTAL CONSOLIDADO [Banco: %s | %d memórias ativas] ===\n\n", bank, total)

	typeOrder := []FactType{TypePreference, TypeLesson, TypeDecision, TypeFact}
	typeHeaders := map[FactType]string{
		TypePreference: "🎯 PREFERÊNCIAS E DIRETRIZES",
		TypeLesson:     "⚠️ LIÇÕES APRENDIDAS (NÃO REPETIR ERROS)",
		TypeDecision:   "📐 DECISÕES DE ARQUITETURA",
		TypeFact:       "📌 FATOS E ESTADOS OPERACIONAIS",
	}

	for _, t := range typeOrder {
		items := byType[t]
		if len(items) == 0 {
			continue
		}
		sb.WriteString(typeHeaders[t])
		sb.WriteString(":\n")
		for _, it := range items {
			if it.Subject != "" {
				fmt.Fprintf(&sb, "  • [%s] %s\n", it.Subject, it.Content)
			} else {
				fmt.Fprintf(&sb, "  • %s\n", it.Content)
			}
		}
		sb.WriteByte('\n')
	}

	return strings.TrimSpace(sb.String()), nil
}

// FormatRecalled turns recalled memories into a high-density, minimal-token context block.
func FormatRecalled(hits []ScoredMemory) string {
	if len(hits) == 0 {
		return "Nenhuma memória relevante encontrada."
	}
	var sb strings.Builder
	sb.WriteString("[CONTEXTO DE MEMÓRIA LONGA / HINDSIGHT]:\n")
	for _, h := range hits {
		m := h.Memory
		prefix := ""
		if m.Subject != "" {
			prefix = "[" + m.Subject + "] "
		}
		fmt.Fprintf(&sb, "• (%s) %s%s\n", m.Type, prefix, m.Content)
	}
	return strings.TrimSpace(sb.String())
}

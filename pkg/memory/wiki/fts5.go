package wiki

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

var (
	nonWordChars = regexp.MustCompile(`[^\p{L}\p{N}_\-]+`)
)

// DB encapsulates the SQLite FTS5 index for fast full-text search.
type DB struct {
	db *sql.DB
}

// OpenDB opens or creates the SQLite FTS5 database at dbPath.
func OpenDB(dbPath string) (*DB, error) {
	// Enable WAL mode and foreign keys for high performance
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DB{db: db}, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS wiki_meta (
		key TEXT PRIMARY KEY,
		val TEXT
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS wiki_fts USING fts5(
		path,
		title,
		tags,
		links,
		content,
		tokenize = 'porter unicode61'
	);
	`
	_, err := db.Exec(schema)
	return err
}

// Close closes the underlying SQLite database connection.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// UpsertPage indexes or re-indexes a wiki page in SQLite FTS5.
func (d *DB) UpsertPage(p *Page) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Remove existing entry if present
	_, err = tx.Exec("DELETE FROM wiki_fts WHERE path = ?", p.Path)
	if err != nil {
		return fmt.Errorf("failed to remove existing fts page: %w", err)
	}

	// 2. Insert updated document
	tagsJoined := strings.Join(p.Tags, " ")
	linksJoined := strings.Join(p.Links, " ")

	_, err = tx.Exec(
		"INSERT INTO wiki_fts(path, title, tags, links, content) VALUES (?, ?, ?, ?, ?)",
		p.Path, p.Title, tagsJoined, linksJoined, p.Content,
	)
	if err != nil {
		return fmt.Errorf("failed to insert fts page: %w", err)
	}

	return tx.Commit()
}

// DeletePage removes a page from the FTS5 index.
func (d *DB) DeletePage(path string) error {
	_, err := d.db.Exec("DELETE FROM wiki_fts WHERE path = ?", path)
	return err
}

// Search queries the FTS5 index using BM25 ranking and extracts contextual snippets.
func (d *DB) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	// Query with BM25 ranking and snippet extraction
	querySQL := `
	SELECT path, title, tags, links, snippet(wiki_fts, 4, '**', '**', '...', 25), bm25(wiki_fts) AS rank
	FROM wiki_fts
	WHERE wiki_fts MATCH ?
	ORDER BY rank ASC
	LIMIT ?;
	`

	rows, err := d.db.Query(querySQL, ftsQuery, limit)
	if err != nil {
		// Fallback to simple prefix query if syntax fails
		prefixQuery := buildPrefixQuery(query)
		if prefixQuery != ftsQuery {
			rows, err = d.db.Query(querySQL, prefixQuery, limit)
		}
		if err != nil {
			return nil, fmt.Errorf("fts5 search error (%s): %w", ftsQuery, err)
		}
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var (
			path, title, tagsRaw, linksRaw, snippet string
			rank                                    float64
		)
		if err := rows.Scan(&path, &title, &tagsRaw, &linksRaw, &snippet, &rank); err != nil {
			return nil, err
		}

		var tags []string
		if strings.TrimSpace(tagsRaw) != "" {
			tags = strings.Fields(tagsRaw)
		}

		var links []string
		if strings.TrimSpace(linksRaw) != "" {
			links = strings.Fields(linksRaw)
		}

		results = append(results, SearchResult{
			Path:    path,
			Title:   title,
			Snippet: snippet,
			Score:   -rank, // bm25 returns negative numbers where smaller is better; invert for intuitive score
			Tags:    tags,
			Links:   links,
		})
	}

	return results, rows.Err()
}

// Count returns the total number of indexed pages.
func (d *DB) Count() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM wiki_fts").Scan(&count)
	return count, err
}

func sanitizeFTS5Query(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	tokens := nonWordChars.Split(raw, -1)
	var validTokens []string
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if len(tok) >= 2 {
			validTokens = append(validTokens, tok)
		}
	}

	if len(validTokens) == 0 {
		return ""
	}

	// Build MATCH expression: token1* OR token2*
	var matchParts []string
	for _, tok := range validTokens {
		matchParts = append(matchParts, fmt.Sprintf(`"%s"*`, tok))
	}
	return strings.Join(matchParts, " OR ")
}

func buildPrefixQuery(raw string) string {
	tokens := strings.Fields(raw)
	var parts []string
	for _, tok := range tokens {
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, tok)
		if clean != "" {
			parts = append(parts, clean+"*")
		}
	}
	return strings.Join(parts, " ")
}

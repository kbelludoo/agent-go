package bm25

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultK1 = 1.5
	defaultB  = 0.75
)

// Document represents an indexed file or text chunk.
type Document struct {
	ID       string
	Text     string
	TermFreq map[string]int
	Length   int
}

// ScoredDoc represents a search hit with its Okapi BM25 score.
type ScoredDoc struct {
	ID      string
	Score   float64
	Snippet string
}

// Index is an in-memory Okapi BM25 inverted index.
type Index struct {
	Docs      []Document
	DocFreq   map[string]int
	AvgDocLen float64
}

// NewIndex builds a BM25 index from a slice of (id, text) documents.
func NewIndex(items map[string]string) *Index {
	idx := &Index{
		Docs:    make([]Document, 0, len(items)),
		DocFreq: make(map[string]int),
	}

	totalLen := 0
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, id := range keys {
		text := items[id]
		tokens := Tokenize(text)
		tf := make(map[string]int, len(tokens)/2+1)
		for _, tok := range tokens {
			tf[tok]++
		}
		for term := range tf {
			idx.DocFreq[term]++
		}
		idx.Docs = append(idx.Docs, Document{
			ID:       id,
			Text:     text,
			TermFreq: tf,
			Length:   len(tokens),
		})
		totalLen += len(tokens)
	}

	if len(idx.Docs) > 0 {
		idx.AvgDocLen = float64(totalLen) / float64(len(idx.Docs))
	}
	return idx
}

// IndexDirectory walks a directory and indexes source/doc files into chunks.
func IndexDirectory(rootDir string) (*Index, int, error) {
	items := make(map[string]string)
	totalBytes := 0

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".go", ".md", ".ts", ".js", ".py", ".json", ".yaml", ".yml", ".txt":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 || len(data) > 256*1024 {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			rel = path
		}
		items[filepath.ToSlash(rel)] = string(data)
		totalBytes += len(data)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return NewIndex(items), totalBytes, nil
}

// Search ranks documents against query using Okapi BM25 and returns topK hits.
func (idx *Index) Search(query string, topK int) []ScoredDoc {
	if len(idx.Docs) == 0 || topK <= 0 {
		return nil
	}
	qTokens := Tokenize(query)
	if len(qTokens) == 0 {
		return nil
	}

	nDocs := float64(len(idx.Docs))
	var results []ScoredDoc

	for _, doc := range idx.Docs {
		score := 0.0
		for _, qt := range qTokens {
			tf := float64(doc.TermFreq[qt])
			if tf == 0 {
				continue
			}
			df := float64(idx.DocFreq[qt])
			idf := math.Log(1.0 + (nDocs-df+0.5)/(df+0.5))
			norm := tf * (defaultK1 + 1.0) / (tf + defaultK1*(1.0-defaultB+defaultB*(float64(doc.Length)/idx.AvgDocLen)))
			score += idf * norm
		}
		if score > 0 {
			results = append(results, ScoredDoc{
				ID:      doc.ID,
				Score:   score,
				Snippet: extractBestSnippet(doc.Text, qTokens, 240),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ID < results[j].ID
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

// FormatResults formats search hits into compact LLM context markdown.
func FormatResults(hits []ScoredDoc) string {
	if len(hits) == 0 {
		return "Nenhum resultado encontrado."
	}
	var sb strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&sb, "[%d] %s (score: %.2f)\n%s\n\n", i+1, h.ID, h.Score, h.Snippet)
	}
	return strings.TrimSpace(sb.String())
}

func extractBestSnippet(text string, qTokens []string, maxLen int) string {
	lines := strings.Split(text, "\n")
	bestIdx := 0
	bestHits := -1

	for i, line := range lines {
		lower := strings.ToLower(line)
		hits := 0
		for _, qt := range qTokens {
			if strings.Contains(lower, qt) {
				hits++
			}
		}
		if hits > bestHits {
			bestHits = hits
			bestIdx = i
		}
	}

	start := bestIdx - 1
	if start < 0 {
		start = 0
	}
	end := start + 5
	if end > len(lines) {
		end = len(lines)
	}

	snippet := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if len(snippet) > maxLen {
		snippet = snippet[:maxLen] + "..."
	}
	return snippet
}

// Tokenize splits identifier and natural language text into lowercase terms,
// handling camelCase and snake_case boundaries.
func Tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder

	flush := func() {
		if cur.Len() >= 2 {
			tokens = append(tokens, strings.ToLower(cur.String()))
		}
		cur.Reset()
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		if isAlphaNum(c) {
			// Split camelCase boundary: lowercase followed by uppercase
			if c >= 'A' && c <= 'Z' && cur.Len() > 0 {
				prev := s[i-1]
				if prev >= 'a' && prev <= 'z' {
					flush()
				}
			}
			cur.WriteByte(c)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

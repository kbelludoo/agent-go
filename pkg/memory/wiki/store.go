package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	headingRegex = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	wikiLinkRegex = regexp.MustCompile(`\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|([^\]]+))?\]\]`)
	hashtagRegex  = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_\-]+)`)
)

// Store manages the markdown wiki repository and its companion SQLite FTS5 index.
type Store struct {
	mu      sync.RWMutex
	RootDir string
	db      *DB
}

// NewStore creates or opens a Store located at rootDir (e.g. ".ai-memory").
func NewStore(rootDir string) (*Store, error) {
	if rootDir == "" {
		rootDir = ".ai-memory"
	}

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create memory directory %s: %w", rootDir, err)
	}

	dbPath := filepath.Join(rootDir, "index.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, err
	}

	s := &Store{
		RootDir: rootDir,
		db:      db,
	}

	// Ensure .gitignore exists to ignore local index database and temp files
	_ = s.ensureGitignore()

	return s, nil
}

func (s *Store) ensureGitignore() error {
	giPath := filepath.Join(s.RootDir, ".gitignore")
	if _, err := os.Stat(giPath); os.IsNotExist(err) {
		content := "# Derived cache files - do not commit to git\nindex.db\nindex.db-*\n*.tmp\n*.log\n"
		return os.WriteFile(giPath, []byte(content), 0644)
	}
	return nil
}

// Close closes the underlying SQLite database.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Init creates folder structure and seeds baseline knowledge pages if empty.
func (s *Store) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pagesDir := filepath.Join(s.RootDir, "pages")
	sessionsDir := filepath.Join(s.RootDir, "sessions")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}

	// Create starter pages if no pages exist
	starterPages := map[string]string{
		"architecture.md": `# Arquitetura do Sistema

## Visão Geral
Sistema de agentes de IA híbrido em **Go nativo + Assembly AVX2**, projetado para monitoramento, geração de código e automação de bots para jogos Idle.

## Componentes Principais
- **Core Agent**: Loop de raciocinio, ferramentas de compilacao e teste ('go build', 'go test').
- **Memória Wiki (ai-memory)**: Base de conhecimento em Markdown versionada por Git com índice SQLite FTS5 instantâneo.
- **Rede Neural Bi-Encoder (8K 1-Bit)**: Kernels em Assembly AVX2 para busca semântica em ultra-alta velocidade.
- **OpenCode & ACP**: Integração para roteamento de provedores e clientes headless.
- **Watchdog VPS**: Monitor de memória e processos para ambiente de recursos restritos.

## Relações
Ver [[stack]] para especificações de runtime e [[vps_and_bots]] para inventário dos bots.
`,
		"stack.md": `# Stack Tecnológica & Restrições

#stack #go #assembly #sqlite #performance

- **Linguagem Principal**: Go 1.26+ (compilação estática, zero CGO).
- **Kernels Numéricos**: AVX2 256-bit SIMD em Assembly puro (pkg/neural/neural_amd64.s).
- **Banco de Conhecimento**: Markdown puro versionado no Git + SQLite FTS5 (pkg/memory/wiki).
- **Conexões HTTP/WS**: Pooled HTTP client com rotação de IPs e headers compatíveis.
- **Restrição Crítica**: Zero dependência de Python ou serviços externos pesados na VPS (1GB RAM).
`,
		"decisions.md": `# Registro de Decisões de Arquitetura (ADR)

#adr #decisions

## ADR-001: Memória Baseada em Markdown Git-Backed
- **Decisão**: Usar Markdown no estilo [[architecture]] como fonte da verdade, com SQLite FTS5 como índice derivado descartável.
- **Motivo**: Permite leitura direta por desenvolvedores, versionamento pelo Git e handoff limpo entre Antigravity, Cursor, OpenCode e Agent-Go.

## ADR-002: Assembly AVX2 para Dot Product e Hamming
- **Decisão**: Implementar operações de 1-bit e INT8 em Assembly x86-64 manual.
- **Motivo**: Executa em ~55 nanossegundos na VPS sem sobrecarregar a CPU.
`,
		"vps_and_bots.md": `# Ambiente VPS & Inventário de Bots

#vps #bots #oracle-cloud

- **Servidor**: Oracle Cloud VPS (129.153.157.17), processador AMD EPYC (Zen 1), 1GB RAM total.
- **Bots Ativos**: 9 bots de jogos Idle rodando nas portas 3000 a 3010 e porta 8090.
- **Vigilância**: Watchdog nativo rodando a cada 45s para reiniciar bots e manter consumo < 500MB RAM.
`,
	}

	for filename, content := range starterPages {
		filePath := filepath.Join(s.RootDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return err
			}
		}
	}

	// Index everything
	_, err := s.indexUnlocked()
	return err
}

// IndexAll scans all markdown files inside RootDir and indexes them into SQLite FTS5.
func (s *Store) IndexAll() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexUnlocked()
}

func (s *Store) indexUnlocked() (int, error) {
	var count int
	err := filepath.Walk(s.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(s.RootDir, path)
		if err != nil {
			return nil
		}
		// Convert to unix-style slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)

		page, err := s.loadPageFile(path, relPath)
		if err != nil {
			return nil
		}

		if err := s.db.UpsertPage(page); err != nil {
			return err
		}
		count++
		return nil
	})

	return count, err
}

func (s *Store) loadPageFile(fullPath, relPath string) (*Page, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	// Extract Title
	title := ""
	if m := headingRegex.FindStringSubmatch(content); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	}

	// Extract Hashtags
	tagMap := make(map[string]bool)
	tagMatches := hashtagRegex.FindAllStringSubmatch(content, -1)
	for _, m := range tagMatches {
		if len(m) > 1 && len(m[1]) > 1 {
			tagMap[strings.ToLower(m[1])] = true
		}
	}
	var tags []string
	for t := range tagMap {
		tags = append(tags, t)
	}

	// Extract [[wiki-links]]
	linkMap := make(map[string]bool)
	linkMatches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	for _, m := range linkMatches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			linkMap[strings.TrimSpace(m[1])] = true
		}
	}
	var links []string
	for l := range linkMap {
		links = append(links, l)
	}

	fi, _ := os.Stat(fullPath)
	modTime := time.Now()
	if fi != nil {
		modTime = fi.ModTime()
	}

	return &Page{
		Path:      relPath,
		Title:     title,
		Tags:      tags,
		Links:     links,
		Content:   content,
		UpdatedAt: modTime,
	}, nil
}

// Remember writes or updates a markdown knowledge page and updates the index immediately.
func (s *Store) Remember(relPath, title, content string, tags []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if relPath == "" {
		slug := slugify(title)
		if slug == "" {
			slug = fmt.Sprintf("memo-%d", time.Now().Unix())
		}
		relPath = filepath.Join("pages", slug+".md")
	} else if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	relPath = filepath.ToSlash(relPath)

	fullPath := filepath.Join(s.RootDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}

	// Format markdown with header and tags if not present
	var sb strings.Builder
	if !strings.HasPrefix(strings.TrimSpace(content), "#") && title != "" {
		sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	}
	if len(tags) > 0 {
		var formattedTags []string
		for _, t := range tags {
			t = strings.TrimPrefix(t, "#")
			formattedTags = append(formattedTags, "#"+t)
		}
		sb.WriteString(strings.Join(formattedTags, " ") + "\n\n")
	}
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}

	finalContent := sb.String()
	if err := os.WriteFile(fullPath, []byte(finalContent), 0644); err != nil {
		return "", err
	}

	// Reindex page
	page, err := s.loadPageFile(fullPath, relPath)
	if err == nil {
		_ = s.db.UpsertPage(page)
	}

	return relPath, nil
}

// Search searches across all indexed markdown pages using FTS5 with BM25 ranking.
func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.Search(query, limit)
}

// GetPage loads a page by its relative path.
func (s *Store) GetPage(relPath string) (*Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	relPath = filepath.ToSlash(relPath)
	fullPath := filepath.Join(s.RootDir, filepath.FromSlash(relPath))
	return s.loadPageFile(fullPath, relPath)
}

// ListPages returns all markdown pages relative paths.
func (s *Store) ListPages() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pages []string
	err := filepath.Walk(s.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			if rel, err := filepath.Rel(s.RootDir, path); err == nil {
				pages = append(pages, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return pages, err
}

func slugify(text string) string {
	text = strings.ToLower(text)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug := reg.ReplaceAllString(text, "-")
	return strings.Trim(slug, "-")
}

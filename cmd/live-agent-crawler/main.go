package main

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"ai-go-browser/browser"
	"ai-go-browser/pkg/memory"
	"ai-go-browser/pkg/search"
	"github.com/kbelludoo/agent-go/pkg/agent"
	"github.com/kbelludoo/agent-go/pkg/navigator"
)

// DistinctSiteRecord represents a verified unique website visited and reasoned by the autonomous AI engine.
type DistinctSiteRecord struct {
	ID                  uint64  `json:"id"`
	Domain              string  `json:"domain"`
	Category            string  `json:"category"`
	URL                 string  `json:"url"`
	Title               string  `json:"title"`
	StatusCode          int     `json:"status_code"`
	DurationMs          int64   `json:"duration_ms"`
	RawBytes              int     `json:"raw_bytes"`
	MarkdownBytes         int     `json:"markdown_bytes"`
	CSOBytes              int     `json:"cso_bytes,omitempty"`
	TokenReductionPct     float64 `json:"token_reduction_pct"`
	CSOTokenReductionPct  float64 `json:"cso_token_reduction_pct,omitempty"`
	InteractiveElements int     `json:"interactive_elements"`
	RSSMiB              float64 `json:"rss_mib"`
	FirstSeen           string  `json:"first_seen"`
	AISummary           string  `json:"ai_summary,omitempty"`
	AIProvider          string  `json:"ai_provider,omitempty"`
	JA4                 string  `json:"ja4,omitempty"`
}

// QuarantineSiteRecord represents a site that failed (e.g. 403, 405, 504) saved for browser improvement research.
type QuarantineSiteRecord struct {
	ID           uint64 `json:"id"`
	Domain       string `json:"domain"`
	URL          string `json:"url"`
	StatusCode   int    `json:"status_code"`
	ErrorMessage string `json:"error_message"`
	FirstSeen    string `json:"first_seen"`
	LastSeen     string `json:"last_seen"`
	Attempts     int    `json:"attempts"`
	Diagnosis    string `json:"diagnosis"`
}

// TelemetryPayload represents the full state served to GitHub Pages and the API.
type TelemetryPayload struct {
	LastUpdated           string               `json:"last_updated"`
	UptimeSeconds         int64                `json:"uptime_seconds"`
	HostInfo              string               `json:"host_info"`
	BrowserEngine         string               `json:"browser_engine"`
	AIEngine              string               `json:"ai_engine"`
	TotalVisits           uint64               `json:"total_visits"`
	DistinctSitesCount    int                  `json:"distinct_sites_count"`
	SuccessfulVisits      uint64               `json:"successful_visits"`
	QuarantineSitesCount  int                  `json:"quarantine_sites_count"`
	AvgTokenReductionPct  float64              `json:"avg_token_reduction_pct"`
	TotalRawBytes         int64                `json:"total_raw_bytes"`
	TotalMarkdownBytes    int64                `json:"total_markdown_bytes"`
	TotalCSOBytes         int64                `json:"total_cso_bytes,omitempty"`
	BytesSaved            int64                `json:"bytes_saved"`
	AvgCSOReductionPct    float64              `json:"avg_cso_reduction_pct,omitempty"`
	CurrentRSSMiB         float64              `json:"current_rss_mib"`
	ChromiumRAMEstimate   string               `json:"chromium_ram_estimate"`
	CognitiveMemoryActive bool                 `json:"cognitive_memory_active"`
	CognitiveMemoryDB     string               `json:"cognitive_memory_db,omitempty"`
	RecentVisits          []DistinctSiteRecord `json:"recent_visits"`
}

var defaultSeedURLs = []string{
	"https://go.dev/blog/",
	"https://news.ycombinator.com/",
	"https://thehackernews.com/",
	"https://en.wikipedia.org/wiki/Special:Random",
	"https://pt.wikipedia.org/wiki/Especial:Aleat%C3%B3ria",
	"https://github.com/trending",
	"https://tls.peet.ws/api/all",
	"https://httpbin.org/headers",
	"https://lobste.rs/",
	"https://slashdot.org/",
	"https://phys.org/",
	"https://arstechnica.com/",
}

var searchTopicKeywords = []string{
	"golang open source development",
	"cybersecurity zero day vulnerabilities",
	"artificial intelligence llm architecture",
	"linux kernel distributed systems",
	"cloud native kubernetes microservices",
	"quantum computing cryptography",
	"database storage engine btree lsm",
	"rust memory safety programming",
	"webassembly wasm browser performance",
	"devops security continuous delivery",
}

type GoogleNewsRSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
		} `xml:"item"`
	} `xml:"channel"`
}

func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func isValidExplorationURL(rawURL string) bool {
	if len(rawURL) < 10 || len(rawURL) > 300 {
		return false
	}
	if strings.Contains(rawURL, "chrome-extension") || strings.Contains(rawURL, "javascript:") || strings.Contains(rawURL, "mailto:") {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if _, err := url.QueryUnescape(u.RawPath); err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// Skip login/redirect walls that do not add content diversity
	if host == "accounts.google.com" || host == "l.facebook.com" || host == "login.microsoftonline.com" || host == "t.co" {
		return false
	}
	lowerPath := strings.ToLower(u.Path)
	for _, badExt := range []string{".dmg", ".exe", ".msi", ".iso", ".zip", ".tar.gz", ".mp4", ".mp3", ".avi", ".apk"} {
		if strings.HasSuffix(lowerPath, badExt) {
			return false
		}
	}
	if strings.Contains(lowerPath, "/signin") || strings.Contains(lowerPath, "/login") || strings.Contains(lowerPath, "/oauth") {
		return false
	}
	return true
}

func categorizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "Web Aberta"
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case strings.Contains(host, "hackernews") || strings.Contains(host, "security") || strings.Contains(host, "bleepingcomputer"):
		return "Cibersegurança & Defesa"
	case strings.Contains(host, "github") || strings.Contains(host, "gitlab") || strings.Contains(host, "crates") || strings.Contains(host, "golang") || strings.Contains(host, "rust-lang"):
		return "Dev, Repos & Compiladores"
	case strings.Contains(host, "wikipedia") || strings.Contains(host, "phys.org") || strings.Contains(host, "nature") || strings.Contains(host, "arxiv"):
		return "Ciência & Enciclopédias"
	case strings.Contains(host, "ycombinator") || strings.Contains(host, "lobste.rs") || strings.Contains(host, "slashdot") || strings.Contains(host, "arstechnica"):
		return "Comunidades & Notícias Tech"
	case strings.Contains(host, "tls.peet.ws") || strings.Contains(host, "httpbin"):
		return "Auditoria uTLS & Fingerprint"
	case strings.Contains(host, "react") || strings.Contains(host, "vue") || strings.Contains(host, "svelte") || strings.Contains(host, "nextjs") || strings.Contains(host, "angular") || strings.Contains(host, "mozilla") || strings.Contains(host, "web.dev"):
		return "Frameworks & Web Standards"
	case strings.Contains(host, "docker") || strings.Contains(host, "kubernetes") || strings.Contains(host, "redis") || strings.Contains(host, "postgresql"):
		return "Infraestrutura & Cloud"
	default:
		return "Documentações & Web Aberta"
	}
}

func diagnoseFailure(statusCode int, err error) string {
	if err != nil {
		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "context deadline"):
			return "Timeout de conexão: Servidor lento ou conexão descartada pelo firewall. Sugestão: Aumentar timeout ou chavear TLS dialer."
		case strings.Contains(errStr, "handshake"):
			return "Falha de handshake TLS: Servidor pode exigir suite de criptografia específica ou ALPN restrito. Sugestão: Simular Firefox ou Safari TLS."
		default:
			return fmt.Sprintf("Erro de transporte de rede: %s. Sugestão: Verificar rota de egress e resolução DNS.", errStr)
		}
	}

	switch statusCode {
	case 405:
		return "HTTP 405 Method Not Allowed: Servidor rejeita certas requisições uTLS/H2 ou exige barra final ('/'). Sugestão: Testar fallback HTTP/1.1 e normalizar trailing slash."
	case 403:
		return "HTTP 403 Forbidden: WAF / Cloudflare acionado (IP de Datacenter detectado). Sugestão: Ativar rotação com IP residencial ou simular cookies prévios com stealth."
	case 429:
		return "HTTP 429 Too Many Requests: Limite de taxa do servidor atingido. Sugestão: Adicionar jitter e delay de re-tentativa exponencial."
	case 500, 502, 503, 504:
		return fmt.Sprintf("HTTP %d Gateway/Server Error: Servidor de destino com instabilidade temporária. Sugestão: Re-enfileirar após 15 minutos.", statusCode)
	default:
		return fmt.Sprintf("HTTP %d inesperado. Salvo para análise e aprimoramento do motor de navegação.", statusCode)
	}
}

// DiscoverDynamicURLs fetches fresh URLs from Google News RSS and DuckDuckGo HTML without browser overhead.
func DiscoverDynamicURLs(ctx context.Context, query string) []string {
	var discovered []string

	// 1. Google News RSS Feeds
	feedURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", url.QueryEscape(query))
	client := &http.Client{
		Transport: browser.NewSafeTransport(false),
		Timeout:   8 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		if resp, err := client.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var rss GoogleNewsRSS
			if xml.Unmarshal(body, &rss) == nil {
				for i, item := range rss.Channel.Items {
					if item.Link != "" {
						discovered = append(discovered, item.Link)
						if i >= 15 {
							break
						}
					}
				}
			}
		}
	}

	// 2. DuckDuckGo HTML organic search via pkg/search
	searchEngine := search.NewEngine()
	searchCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if ddgRes, err := searchEngine.Search(searchCtx, query); err == nil {
		for i, r := range ddgRes.Results {
			if r.URL != "" && strings.HasPrefix(r.URL, "http") {
				discovered = append(discovered, r.URL)
				if i >= 10 {
					break
				}
			}
		}
	}

	return discovered
}

func main() {
	addr := flag.String("addr", ":8082", "HTTP API listen address (CORS enabled for live dashboard)")
	outputJSON := flag.String("output", "docs/live_crawler_data.json", "Path to save live telemetry JSON for GitHub Pages")
	distinctJSON := flag.String("distinct-output", "docs/distinct_sites.json", "Path to save distinct sites catalog")
	quarantineJSON := flag.String("quarantine-output", "docs/quarantine_sites.json", "Path to save quarantined failed sites for browser self-healing research")
	seedFile := flag.String("seed", "data/distinct_seed_sites.txt", "Path to text file containing seed URLs")
	memoryDB := flag.String("memory-db", "data/browser_memory.db", "Path to SQLite persistent cognitive memory database")
	intervalSec := flag.Int("interval", 12, "Seconds between autonomous site visits")
	gitPushInterval := flag.Int("git-interval", 0, "Minutes between internal git commit/push (0 = managed by systemd timer)")
	flag.Parse()

	log.Printf("🚀 Iniciando AI-Go-Browser + Agent-Go Autonomous AI Crawler na VPS")
	log.Printf("📊 Telemetria ao vivo: %s | Catálogo de sites distintos: %s", *outputJSON, *distinctJSON)
	log.Printf("🌐 Servidor HTTP API na porta: %s", *addr)

	// Ensure directories exist
	_ = os.MkdirAll(filepath.Dir(*outputJSON), 0755)
	_ = os.MkdirAll(filepath.Dir(*distinctJSON), 0755)
	_ = os.MkdirAll(filepath.Dir(*memoryDB), 0755)

	session := navigator.NewSession()
	memEngine, err := memory.NewEngine(*memoryDB)
	if err != nil {
		log.Printf("⚠️ Erro ao iniciar Cognitive Memory Engine em %s: %v", *memoryDB, err)
	} else {
		session.WithMemory(memEngine)
		log.Printf("🧠 [Cognitive Memory] Browser Playbooks & FTS5 ativados em %s", *memoryDB)
	}
	llmClient := agent.NewLLMClient("", "", "")
	log.Printf("🤖 Agent-Go LLM Cascade inicializado com %d provedor(es) de IA configurado(s)", len(llmClient.Providers))

	var (
		mu           sync.RWMutex
		visitCounter uint64
		successCount uint64
		recentVisits []DistinctSiteRecord
		distinctMap   = make(map[string]DistinctSiteRecord)
		quarantineMap = make(map[string]QuarantineSiteRecord)
		startTime     = time.Now()
		totalRaw     int64
		totalMD      int64
		totalCSO     int64
		visitedSet   = make(map[string]bool)
		keywordIdx   = 0
	)

	// Load seeds from file if exists
	var seedList []string
	if file, err := os.Open(*seedFile); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				seedList = append(seedList, line)
			}
		}
		_ = file.Close()
		log.Printf("📋 Carregadas %d URLs sementes do arquivo %s", len(seedList), *seedFile)
	}

	if len(seedList) == 0 {
		seedList = defaultSeedURLs
	}

	// Try reading existing distinct catalog to preserve history across restarts
	if data, err := os.ReadFile(*distinctJSON); err == nil {
		var catalog struct {
			Sites []DistinctSiteRecord `json:"distinct_sites"`
		}
		if err := json.Unmarshal(data, &catalog); err == nil {
			for _, s := range catalog.Sites {
				distinctMap[s.URL] = s
				visitedSet[s.URL] = true
			}
			log.Printf("🔄 Catálogo recuperado: %d sites distintos anteriores", len(distinctMap))
		}
	}

	// Try reading existing quarantine catalog
	if data, err := os.ReadFile(*quarantineJSON); err == nil {
		var qCatalog struct {
			Sites []QuarantineSiteRecord `json:"quarantine_sites"`
		}
		if err := json.Unmarshal(data, &qCatalog); err == nil {
			for _, s := range qCatalog.Sites {
				quarantineMap[s.URL] = s
			}
			log.Printf("🧪 Quarentena recuperada: %d sites para laboratório de self-healing", len(quarantineMap))
		}
	}

	if len(distinctMap) > 0 {
		successCount = uint64(len(distinctMap))
		visitCounter = uint64(len(distinctMap) + len(quarantineMap))
	}

	// Dynamic queue: prioritize re-testing healable quarantine sites first (up to 15 per restart), then unvisited seeds
	urlQueue := make([]string, 0, len(seedList)*2)
	requeuedQuarantine := 0
	for qURL, qSite := range quarantineMap {
		if qSite.Attempts < 4 && isValidExplorationURL(qURL) {
			delete(visitedSet, qURL)
			urlQueue = append(urlQueue, qURL)
			requeuedQuarantine++
			if requeuedQuarantine >= 15 {
				break
			}
		} else if !isValidExplorationURL(qURL) {
			// Clean invalid extension/binary URLs from quarantine
			delete(quarantineMap, qURL)
		}
	}
	if requeuedQuarantine > 0 {
		log.Printf("🩹 [Self-Healing] %d sites da quarentena enfileirados para re-teste via Cloudflare WARP + uTLS", requeuedQuarantine)
	}
	for _, u := range seedList {
		if !visitedSet[u] || strings.Contains(u, "Random") || strings.Contains(u, "Aleat") {
			urlQueue = append(urlQueue, u)
		}
	}

	getNextURL := func() string {
		mu.Lock()
		defer mu.Unlock()

		// If queue running low, trigger dynamic search discovery!
		if len(urlQueue) < 15 {
			kw := searchTopicKeywords[keywordIdx%len(searchTopicKeywords)]
			keywordIdx++
			log.Printf("🔍 Fila baixa (%d itens). Buscando sites frescos na web via Google News RSS & DuckDuckGo para: %q", len(urlQueue), kw)
			go func(topic string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				newLinks := DiscoverDynamicURLs(ctx, topic)
				mu.Lock()
				added := 0
				for _, l := range newLinks {
					if isValidExplorationURL(l) && !visitedSet[l] && len(urlQueue) < 500 {
						urlQueue = append(urlQueue, l)
						added++
					}
				}
				mu.Unlock()
				log.Printf("✨ Descoberta concluída: +%d sites adicionados à fila para %q", added, topic)
			}(kw)
		}

		if len(urlQueue) == 0 {
			for _, u := range seedList {
				urlQueue = append(urlQueue, u)
			}
		}

		u := urlQueue[0]
		urlQueue = urlQueue[1:]
		return u
	}

	addURLToQueueLocked := func(u string) {
		if isValidExplorationURL(u) && !visitedSet[u] && len(urlQueue) < 500 {
			urlQueue = append(urlQueue, u)
		}
	}

	buildPayload := func() TelemetryPayload {
		mu.RLock()
		defer mu.RUnlock()

		avgRed := 0.0
		if totalRaw > 0 {
			avgRed = (1.0 - float64(totalMD)/float64(totalRaw)) * 100
		}
		avgCSORed := 0.0
		if totalMD > 0 && totalCSO > 0 {
			avgCSORed = (1.0 - float64(totalCSO)/float64(totalMD)) * 100
		}

		curRSS := 0.0
		mem := browser.ReadProcessMemory(os.Getpid())
		if mem.Available {
			curRSS = mem.RSSMiB
		}

		recentCopy := make([]DistinctSiteRecord, len(recentVisits))
		copy(recentCopy, recentVisits)

		return TelemetryPayload{
			LastUpdated:          time.Now().UTC().Format(time.RFC3339),
			UptimeSeconds:        int64(time.Since(startTime).Seconds()),
			HostInfo:             "Oracle Cloud VPS (147.15.109.14 - Ubuntu x86_64, 1GB RAM)",
			BrowserEngine:        "ai-go-browser v2.0 (uTLS Chrome 120 + CSO Token Reducer + SQLite FTS5)",
			AIEngine:             "Agent-Go Multi-Provider LLM Cascade (Auto-Fallback)",
			TotalVisits:          visitCounter,
			DistinctSitesCount:   len(distinctMap),
			SuccessfulVisits:     successCount,
			QuarantineSitesCount: len(quarantineMap),
			AvgTokenReductionPct: avgRed,
			TotalRawBytes:        totalRaw,
			TotalMarkdownBytes:   totalMD,
			TotalCSOBytes:        totalCSO,
			BytesSaved:           totalRaw - totalMD,
			AvgCSOReductionPct:   avgCSORed,
			CurrentRSSMiB:        curRSS,
			ChromiumRAMEstimate:  "900 - 1400 MB (Causaria OOM Kill nesta VPS de 1GB)",
			CognitiveMemoryActive: memEngine != nil,
			CognitiveMemoryDB:    *memoryDB,
			RecentVisits:         recentCopy,
		}
	}

	saveData := func() {
		mu.RLock()
		payload := buildPayload()

		distinctList := make([]DistinctSiteRecord, 0, len(distinctMap))
		for _, v := range distinctMap {
			distinctList = append(distinctList, v)
		}

		quarantineList := make([]QuarantineSiteRecord, 0, len(quarantineMap))
		for _, v := range quarantineMap {
			quarantineList = append(quarantineList, v)
		}
		mu.RUnlock()

		if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
			_ = atomicWriteFile(*outputJSON, b)
		}

		catalogPayload := struct {
			LastUpdated   string               `json:"last_updated"`
			TotalDistinct int                  `json:"total_distinct_sites"`
			DistinctSites []DistinctSiteRecord `json:"distinct_sites"`
		}{
			LastUpdated:   time.Now().UTC().Format(time.RFC3339),
			TotalDistinct: len(distinctList),
			DistinctSites: distinctList,
		}

		if b, err := json.MarshalIndent(catalogPayload, "", "  "); err == nil {
			_ = atomicWriteFile(*distinctJSON, b)
		}

		quarantinePayload := struct {
			LastUpdated     string                 `json:"last_updated"`
			TotalQuarantine int                    `json:"total_quarantine"`
			QuarantineSites []QuarantineSiteRecord `json:"quarantine_sites"`
		}{
			LastUpdated:     time.Now().UTC().Format(time.RFC3339),
			TotalQuarantine: len(quarantineList),
			QuarantineSites: quarantineList,
		}

		if b, err := json.MarshalIndent(quarantinePayload, "", "  "); err == nil {
			_ = atomicWriteFile(*quarantineJSON, b)
		}
	}

	// 1. HTTP Server for Live Dashboard (CORS enabled)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/live", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			payload := buildPayload()
			_ = json.NewEncoder(w).Encode(payload)
		})

		mux.HandleFunc("/api/distinct", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/json")
			mu.RLock()
			distinctList := make([]DistinctSiteRecord, 0, len(distinctMap))
			for _, v := range distinctMap {
				distinctList = append(distinctList, v)
			}
			mu.RUnlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total": len(distinctList),
				"sites": distinctList,
			})
		})

		mux.HandleFunc("/api/quarantine", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/json")
			mu.RLock()
			qList := make([]QuarantineSiteRecord, 0, len(quarantineMap))
			for _, v := range quarantineMap {
				qList = append(qList, v)
			}
			mu.RUnlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total": len(qList),
				"sites": qList,
			})
		})

		mux.HandleFunc("/api/memory/search", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/json")
			query := r.URL.Query().Get("q")
			if query == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"total": 0, "hits": []any{}})
				return
			}
			hits, err := session.SearchMemory(query, 10)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"query": query,
				"total": len(hits),
				"hits":  hits,
			})
		})

		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		srv := &http.Server{
			Addr:         *addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		log.Printf("📡 Servidor API escutando em http://0.0.0.0%s/api/live", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ Erro no servidor HTTP: %v", err)
		}
	}()

	// 2. Periodic Git commit & push worker (updates GitHub Pages repository)
	if *gitPushInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(*gitPushInterval) * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				saveData()
				cmd := exec.Command("git", "add", *outputJSON, *distinctJSON)
				_ = cmd.Run()
				cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("chore(telemetry): update distinct sites catalog [%s]", time.Now().UTC().Format("15:04:05")))
				_ = cmd.Run()
				cmd = exec.Command("git", "push")
				_ = cmd.Run()
			}
		}()
	}

	// 3. Autonomous Crawler Loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(*intervalSec) * time.Second)
	defer ticker.Stop()

	performVisit := func() {
		targetURL := getNextURL()
		navCtx, navCancel := context.WithTimeout(ctx, 12*time.Second)
		defer navCancel()

		parsed, _ := url.Parse(targetURL)
		domain := ""
		if parsed != nil {
			domain = parsed.Hostname()
		}

		log.Printf("🌐 [%s] Navegando via uTLS: %s", time.Now().Format("15:04:05"), targetURL)
		snap, err := session.Navigate(navCtx, targetURL)

		// Adaptive Self-Healing Retry on 403/405
		if err == nil && snap != nil && (snap.StatusCode == 403 || snap.StatusCode == 405) {
			var healURL string
			if strings.Contains(domain, "reddit.com") && !strings.Contains(domain, "old.reddit.com") {
				healURL = strings.Replace(targetURL, domain, "old.reddit.com", 1)
			} else if snap.StatusCode == 405 && !strings.HasSuffix(targetURL, "/") {
				healURL = targetURL + "/"
			}
			if healURL != "" {
				log.Printf("   🩹 [Self-Healing] Tentando rota alternativa para HTTP %d: %s", snap.StatusCode, healURL)
				healCtx, healCancel := context.WithTimeout(ctx, 10*time.Second)
				if healSnap, healErr := session.Navigate(healCtx, healURL); healErr == nil && healSnap.StatusCode < 400 {
					snap = healSnap
					log.Printf("   ✨ [Self-Healing] Sucesso! Site recuperado via %s (Status %d)", healURL, snap.StatusCode)
				}
				healCancel()
			}
		}

		record := DistinctSiteRecord{
			Domain:    domain,
			Category:  categorizeURL(targetURL),
			URL:       targetURL,
			FirstSeen: time.Now().UTC().Format(time.RFC3339),
		}

		var discoveredLinks []string

		if err != nil {
			log.Printf("   ⚠️ Falha ao navegar em %s: %v", targetURL, err)
			record.StatusCode = 504
			record.Title = fmt.Sprintf("Erro: %v", err)
		} else {
			record.Title = snap.Title
			record.StatusCode = snap.StatusCode
			record.DurationMs = snap.DurationMs
			record.RawBytes = snap.RawBytes
			record.MarkdownBytes = snap.MarkdownBytes
			record.CSOBytes = snap.CSOBytes
			record.TokenReductionPct = snap.TokenReduction
			if snap.MarkdownBytes > 0 && snap.CSOBytes > 0 {
				record.CSOTokenReductionPct = (1.0 - float64(snap.CSOBytes)/float64(snap.MarkdownBytes)) * 100
				if record.CSOTokenReductionPct < 0 {
					record.CSOTokenReductionPct = 0
				}
			}
			record.InteractiveElements = len(snap.Elements)
			record.RSSMiB = snap.RSSMiB

			if targetURL == "https://tls.peet.ws/api/all" && snap.CleanText != "" {
				var peetData struct {
					JA4 string `json:"ja4"`
				}
				if json.Unmarshal([]byte(snap.CleanText), &peetData) == nil && peetData.JA4 != "" {
					record.JA4 = peetData.JA4
				}
			}

			// Add discovered external links to queue for autonomous exploration
			for _, el := range snap.Elements {
				if el.Tag == "a" && el.Href != "" && (el.Type == "link" || el.Type == "") {
					if len(el.Href) > 10 && strings.HasPrefix(el.Href, "http") {
						linkParsed, _ := url.Parse(el.Href)
						if linkParsed != nil && linkParsed.Hostname() != domain {
							discoveredLinks = append(discoveredLinks, el.Href)
							if len(discoveredLinks) >= 3 {
								break
							}
						}
					}
				}
			}

			// 4. Agent-Go AI Reasoning with Compact Semantic Outline (CSO, saving ~45% extra LLM tokens)
			if snap.StatusCode == 200 && (snap.CSOText != "" || snap.CleanText != "") {
				snippet := snap.CSOText
				if snippet == "" {
					snippet = snap.CleanText
				}
				if len(snippet) > 1100 {
					snippet = snippet[:1100]
				}
				aiPrompt := fmt.Sprintf("Você é o Agent-Go explorando a web com o ai-go-browser.\nURL: %s\nTítulo: %s\nTrecho da Página (Formato Semântico Compacto):\n%s\n\nEm uma ou duas frases curtas em português, resuma a essência deste site e o que ele oferece.", targetURL, snap.Title, snippet)

				aiCtx, aiCancel := context.WithTimeout(ctx, 8*time.Second)
				msg, prov, aiErr := llmClient.Complete(aiCtx, []agent.Message{
					{Role: "user", Content: aiPrompt},
				}, nil)
				aiCancel()

				if aiErr == nil && msg != nil && strings.TrimSpace(msg.Content) != "" {
					record.AISummary = strings.TrimSpace(msg.Content)
					record.AIProvider = prov
					log.Printf("   🤖 [IA %s]: %s", prov, record.AISummary)
				}
			}
		}

		// Check if site failed and needs quarantine for browser improvement
		isFailure := (err != nil) || (snap != nil && snap.StatusCode >= 400)
		var qRec *QuarantineSiteRecord
		if isFailure {
			status := 504
			errMsg := "Timeout ou falha de rede"
			if err != nil {
				errMsg = err.Error()
			} else if snap != nil {
				status = snap.StatusCode
				errMsg = fmt.Sprintf("HTTP %d", snap.StatusCode)
			}
			qRec = &QuarantineSiteRecord{
				Domain:       domain,
				URL:          targetURL,
				StatusCode:   status,
				ErrorMessage: errMsg,
				LastSeen:     time.Now().UTC().Format(time.RFC3339),
				Diagnosis:    diagnoseFailure(status, err),
			}
		}

		// Update state safely with mutex
		mu.Lock()
		visitCounter++
		record.ID = visitCounter
		visitedSet[targetURL] = true

		if qRec != nil {
			if existing, ok := quarantineMap[targetURL]; ok {
				qRec.ID = existing.ID
				qRec.FirstSeen = existing.FirstSeen
				qRec.Attempts = existing.Attempts + 1
			} else {
				qRec.ID = uint64(len(quarantineMap) + 1)
				qRec.FirstSeen = time.Now().UTC().Format(time.RFC3339)
				qRec.Attempts = 1
			}
			quarantineMap[targetURL] = *qRec
			log.Printf("   🧪 [Laboratório de Quarentena] Site %s registrado: %s", targetURL, qRec.Diagnosis)
		} else if _, wasQuarantined := quarantineMap[targetURL]; wasQuarantined {
			delete(quarantineMap, targetURL)
			log.Printf("   ✨ [Self-Healing] Site %s curado com sucesso (HTTP %d) e removido da quarentena!", targetURL, record.StatusCode)
		}

		if err == nil {
			successCount++
			totalRaw += int64(snap.RawBytes)
			totalMD += int64(snap.MarkdownBytes)
			totalCSO += int64(snap.CSOBytes)
			for _, dl := range discoveredLinks {
				addURLToQueueLocked(dl)
			}
			log.Printf("   ✅ [%s] Status: %d | Latência: %dms | Bruto: %d KB | MD: %d KB (-%.1f%%) | CSO: %d KB (-%.1f%% tokens) | RSS: %.2f MiB | Distintos: %d",
				record.Category, snap.StatusCode, snap.DurationMs, snap.RawBytes/1024, snap.MarkdownBytes/1024, snap.TokenReduction, snap.CSOBytes/1024, record.CSOTokenReductionPct, snap.RSSMiB, len(distinctMap)+1)
		}

		distinctMap[targetURL] = record
		recentVisits = append([]DistinctSiteRecord{record}, recentVisits...)
		if len(recentVisits) > 50 {
			recentVisits = recentVisits[:50]
		}
		mu.Unlock()

		saveData()
	}

	// Run first visit immediately
	performVisit()

	for {
		select {
		case <-sigChan:
			log.Printf("🛑 Sinal de interrupção recebido. Salvando dados e encerrando...")
			saveData()
			return
		case <-ticker.C:
			performVisit()
		}
	}
}

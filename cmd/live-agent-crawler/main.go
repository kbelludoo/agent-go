package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"ai-go-browser/browser"
	"github.com/kbelludoo/agent-go/pkg/navigator"
)

// VisitMetric represents a single empirical observation of a live website.
type VisitMetric struct {
	ID                  uint64  `json:"id"`
	Timestamp           string  `json:"timestamp"`
	URL                 string  `json:"url"`
	Title               string  `json:"title"`
	StatusCode          int     `json:"status_code"`
	DurationMs          int64   `json:"duration_ms"`
	RawBytes            int     `json:"raw_bytes"`
	MarkdownBytes       int     `json:"markdown_bytes"`
	TokenReductionPct   float64 `json:"token_reduction_pct"`
	InteractiveElements int     `json:"interactive_elements"`
	RSSMiB              float64 `json:"rss_mib"`
	JA4                 string  `json:"ja4,omitempty"`
}

// TelemetryPayload represents the full state served to GitHub Pages and the API.
type TelemetryPayload struct {
	LastUpdated          string        `json:"last_updated"`
	UptimeSeconds        int64         `json:"uptime_seconds"`
	HostInfo             string        `json:"host_info"`
	BrowserEngine        string        `json:"browser_engine"`
	TotalVisits          uint64        `json:"total_visits"`
	SuccessfulVisits     uint64        `json:"successful_visits"`
	AvgTokenReductionPct float64       `json:"avg_token_reduction_pct"`
	TotalRawBytes        int64         `json:"total_raw_bytes"`
	TotalMarkdownBytes   int64         `json:"total_markdown_bytes"`
	BytesSaved           int64         `json:"bytes_saved"`
	CurrentRSSMiB        float64       `json:"current_rss_mib"`
	ChromiumRAMEstimate  string        `json:"chromium_ram_estimate"`
	RecentVisits         []VisitMetric `json:"recent_visits"`
}

var defaultSeedURLs = []string{
	"https://go.dev/blog/",
	"https://news.ycombinator.com/",
	"https://thehackernews.com/",
	"https://en.wikipedia.org/wiki/Special:Random",
	"https://github.com/trending",
	"https://tls.peet.ws/api/all",
	"https://httpbin.org/headers",
	"https://lobste.rs/",
	"https://phys.org/",
}

func main() {
	addr := flag.String("addr", ":8082", "HTTP API listen address (CORS enabled for live dashboard)")
	outputJSON := flag.String("output", "docs/live_crawler_data.json", "Path to save live telemetry JSON for GitHub Pages")
	intervalSec := flag.Int("interval", 15, "Seconds between autonomous site visits")
	gitPushInterval := flag.Int("git-interval", 5, "Minutes between git commit/push of telemetry")
	flag.Parse()

	log.Printf("🚀 Iniciando AI-Go-Browser Live Autonomous Crawler na VPS")
	log.Printf("📊 Telemetria será gravada em: %s", *outputJSON)
	log.Printf("🌐 Servidor HTTP API na porta: %s", *addr)

	// Ensure output directory exists
	dir := filepath.Dir(*outputJSON)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	session := navigator.NewSession()
	var (
		mu           sync.RWMutex
		visitCounter uint64
		successCount uint64
		visits       []VisitMetric
		startTime    = time.Now()
		totalRaw     int64
		totalMD      int64
		lastJA4      string
	)

	// Try reading existing telemetry data to preserve history across restarts
	if data, err := os.ReadFile(*outputJSON); err == nil {
		var existing TelemetryPayload
		if err := json.Unmarshal(data, &existing); err == nil {
			visitCounter = existing.TotalVisits
			successCount = existing.SuccessfulVisits
			totalRaw = existing.TotalRawBytes
			totalMD = existing.TotalMarkdownBytes
			visits = existing.RecentVisits
			log.Printf("🔄 Histórico recuperado: %d visitas anteriores", visitCounter)
		}
	}

	// Dynamic queue with seed URLs
	urlQueue := make([]string, len(defaultSeedURLs))
	copy(urlQueue, defaultSeedURLs)
	queueIdx := 0

	getNextURL := func() string {
		mu.Lock()
		defer mu.Unlock()
		if len(urlQueue) == 0 {
			urlQueue = append(urlQueue, defaultSeedURLs...)
		}
		u := urlQueue[queueIdx%len(urlQueue)]
		queueIdx++
		return u
	}

	addURLToQueue := func(u string) {
		mu.Lock()
		defer mu.Unlock()
		if len(urlQueue) > 100 {
			// keep queue bounded
			urlQueue = urlQueue[len(urlQueue)-50:]
		}
		urlQueue = append(urlQueue, u)
	}

	buildPayload := func() TelemetryPayload {
		mu.RLock()
		defer mu.RUnlock()

		avgRed := 0.0
		if totalRaw > 0 {
			avgRed = (1.0 - float64(totalMD)/float64(totalRaw)) * 100
		}

		curRSS := 0.0
		mem := browser.ReadProcessMemory(os.Getpid())
		if mem.Available {
			curRSS = mem.RSSMiB
		}

		recentCopy := make([]VisitMetric, len(visits))
		copy(recentCopy, visits)

		return TelemetryPayload{
			LastUpdated:          time.Now().UTC().Format(time.RFC3339),
			UptimeSeconds:        int64(time.Since(startTime).Seconds()),
			HostInfo:             "Oracle Cloud VPS (147.15.109.14 - Ubuntu x86_64, 1GB RAM)",
			BrowserEngine:        "ai-go-browser v2.0 (uTLS Chrome 120 + JA4 GREASE + Native DOM Action)",
			TotalVisits:          visitCounter,
			SuccessfulVisits:     successCount,
			AvgTokenReductionPct: avgRed,
			TotalRawBytes:        totalRaw,
			TotalMarkdownBytes:   totalMD,
			BytesSaved:           totalRaw - totalMD,
			CurrentRSSMiB:        curRSS,
			ChromiumRAMEstimate:  "900 - 1400 MB (Causaria OOM Kill imediato na VPS de 1GB)",
			RecentVisits:         recentCopy,
		}
	}

	saveJSON := func() {
		payload := buildPayload()
		bytesData, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			_ = os.WriteFile(*outputJSON, bytesData, 0644)
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
				saveJSON()
				cmd := exec.Command("git", "add", *outputJSON)
				_ = cmd.Run()
				cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("chore(telemetry): live crawler update [%s] [skip ci]", time.Now().UTC().Format("15:04:05")))
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

	// Perform initial visit immediately
	performVisit := func() {
		targetURL := getNextURL()
		navCtx, navCancel := context.WithTimeout(ctx, 12*time.Second)
		defer navCancel()

		log.Printf("🌐 [%s] Navegando via uTLS: %s", time.Now().Format("15:04:05"), targetURL)
		snap, err := session.Navigate(navCtx, targetURL)

		mu.Lock()
		visitCounter++
		metric := VisitMetric{
			ID:        visitCounter,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			URL:       targetURL,
		}

		if err != nil {
			log.Printf("   ⚠️ Falha ao navegar em %s: %v", targetURL, err)
			metric.StatusCode = 504
			metric.Title = fmt.Sprintf("Erro: %v", err)
		} else {
			successCount++
			metric.Title = snap.Title
			metric.StatusCode = snap.StatusCode
			metric.DurationMs = snap.DurationMs
			metric.RawBytes = snap.RawBytes
			metric.MarkdownBytes = snap.MarkdownBytes
			metric.TokenReductionPct = snap.TokenReduction
			metric.InteractiveElements = len(snap.Elements)
			metric.RSSMiB = snap.RSSMiB

			totalRaw += int64(snap.RawBytes)
			totalMD += int64(snap.MarkdownBytes)

			// If auditing TLS peet ws, parse JA4
			if targetURL == "https://tls.peet.ws/api/all" && snap.CleanText != "" {
				var peetData struct {
					JA4 string `json:"ja4"`
				}
				if json.Unmarshal([]byte(snap.CleanText), &peetData) == nil && peetData.JA4 != "" {
					lastJA4 = peetData.JA4
					metric.JA4 = lastJA4
				}
			}

			// Add discovered links to queue for autonomous crawling
			addedCount := 0
			for _, el := range snap.Elements {
				if el.Tag == "a" && el.Href != "" && (el.Type == "link" || el.Type == "") {
					if len(el.Href) > 10 && (el.Href[:4] == "http") {
						addURLToQueue(el.Href)
						addedCount++
						if addedCount >= 3 {
							break
						}
					}
				}
			}

			log.Printf("   ✅ Status: %d | Latência: %dms | Bruto: %d KB | Redução Tokens: %.1f%% | RSS Processo: %.2f MiB",
				snap.StatusCode, snap.DurationMs, snap.RawBytes/1024, snap.TokenReduction, snap.RSSMiB)
		}

		// Keep recent 50 visits
		visits = append([]VisitMetric{metric}, visits...)
		if len(visits) > 50 {
			visits = visits[:50]
		}
		mu.Unlock()

		saveJSON()
	}

	// Run first visit immediately
	performVisit()

	for {
		select {
		case <-sigChan:
			log.Printf("🛑 Sinal de interrupção recebido. Salvando dados e encerrando...")
			saveJSON()
			return
		case <-ticker.C:
			performVisit()
		}
	}
}

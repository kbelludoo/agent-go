package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-go-browser/browser"
	"github.com/kbelludoo/agent-go/pkg/navigator"
)

type ProbeResult struct {
	ID                  int     `json:"id"`
	URL                 string  `json:"url"`
	Title               string  `json:"title"`
	StatusCode          int     `json:"status_code"`
	LatencyMs           int64   `json:"latency_ms"`
	RawBytes            int     `json:"raw_bytes"`
	MarkdownBytes       int     `json:"markdown_bytes"`
	TokenReductionPct   float64 `json:"token_reduction_pct"`
	InteractiveElements int     `json:"interactive_elements"`
	RSSMiB              float64 `json:"rss_mib"`
	JA4                 string  `json:"ja4,omitempty"`
}

type ProbeTelemetry struct {
	Timestamp            string        `json:"timestamp"`
	HostInfo             string        `json:"host_info"`
	Engine               string        `json:"engine"`
	TotalTested          int           `json:"total_tested"`
	AvgTokenReductionPct float64       `json:"avg_token_reduction_pct"`
	CurrentRSSMiB        float64       `json:"current_rss_mib"`
	ChromiumComparison   string        `json:"chromium_comparison"`
	Results              []ProbeResult `json:"recent_visits"`
}

func main() {
	targetSites := []string{
		"https://go.dev/blog/",
		"https://news.ycombinator.com/",
		"https://en.wikipedia.org/wiki/Special:Random",
		"https://tls.peet.ws/api/all",
		"https://thehackernews.com/",
	}

	fmt.Println("==========================================================================================")
	fmt.Println("🔬 TESTE RÁPIDO CONTROLADO: AI-GO-BROWSER + AGENT-GO // EMPIRICAL VALIDATION")
	fmt.Println("==========================================================================================")

	session := navigator.NewSession()
	var results []ProbeResult
	var totalRaw, totalMD int64

	for i, u := range targetSites {
		fmt.Printf("[%d/%d] Navegando via uTLS Chrome: %s ... ", i+1, len(targetSites), u)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		snap, err := session.Navigate(ctx, u)
		cancel()

		res := ProbeResult{
			ID:  i + 1,
			URL: u,
		}

		if err != nil {
			fmt.Printf("❌ ERRO (%v)\n", err)
			res.StatusCode = 504
			res.Title = fmt.Sprintf("Erro: %v", err)
		} else {
			res.Title = snap.Title
			res.StatusCode = snap.StatusCode
			res.LatencyMs = snap.DurationMs
			res.RawBytes = snap.RawBytes
			res.MarkdownBytes = snap.MarkdownBytes
			res.TokenReductionPct = snap.TokenReduction
			res.InteractiveElements = len(snap.Elements)
			res.RSSMiB = snap.RSSMiB

			totalRaw += int64(snap.RawBytes)
			totalMD += int64(snap.MarkdownBytes)

			if u == "https://tls.peet.ws/api/all" && snap.CleanText != "" {
				var peet struct {
					JA4 string `json:"ja4"`
				}
				if json.Unmarshal([]byte(snap.CleanText), &peet) == nil {
					res.JA4 = peet.JA4
				}
			}

			fmt.Printf("OK (Status %d | %dms | Redução %.1f%% | RSS: %.1f MiB)\n",
				res.StatusCode, res.LatencyMs, res.TokenReductionPct, res.RSSMiB)
		}

		results = append(results, res)
	}

	avgReduction := 0.0
	if totalRaw > 0 {
		avgReduction = (1.0 - float64(totalMD)/float64(totalRaw)) * 100
	}

	mem := browser.ReadProcessMemory(os.Getpid())
	curRSS := mem.RSSMiB

	payload := ProbeTelemetry{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		HostInfo:             "Oracle Cloud VPS (147.15.109.14 - Ubuntu x86_64, 1GB RAM)",
		Engine:               "ai-go-browser (uTLS JA4 Chrome 120 + Native DOM Reducer)",
		TotalTested:          len(results),
		AvgTokenReductionPct: avgReduction,
		CurrentRSSMiB:        curRSS,
		ChromiumComparison:   "900 - 1400 MB (Inviável nesta máquina de 1GB)",
		Results:              results,
	}

	// Save to JSON
	outPath := "docs/live_crawler_data.json"
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)
	if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.WriteFile(outPath, data, 0644)
	}

	fmt.Println("------------------------------------------------------------------------------------------")
	fmt.Printf("📊 RESULTADO CONSOLIDADO:\n")
	fmt.Printf("   • Média de Redução de Tokens: %.2f%%\n", avgReduction)
	fmt.Printf("   • Consumo de Memória Física RSS: %.2f MiB\n", curRSS)
	fmt.Printf("   • Comparativo Chromium: ~950 MiB (Economia de %.1f%% de RAM)\n", (1.0-curRSS/950.0)*100)
	fmt.Printf("   • Arquivo gravado: %s\n", outPath)
	fmt.Println("==========================================================================================")
}

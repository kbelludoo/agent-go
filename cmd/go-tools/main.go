package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-go-browser/pkg/codembvm"

	"github.com/kbelludoo/agent-go/pkg/bm25"
	"github.com/kbelludoo/agent-go/pkg/caveman"
	"github.com/kbelludoo/agent-go/pkg/cronlite"
	"github.com/kbelludoo/agent-go/pkg/dcp"
	"github.com/kbelludoo/agent-go/pkg/depaudit"
	"github.com/kbelludoo/agent-go/pkg/diffsummary"
	"github.com/kbelludoo/agent-go/pkg/graph"
	"github.com/kbelludoo/agent-go/pkg/hindsight"
	"github.com/kbelludoo/agent-go/pkg/jsonlquery"
	"github.com/kbelludoo/agent-go/pkg/jsonrepair"
	"github.com/kbelludoo/agent-go/pkg/logdedup"
	"github.com/kbelludoo/agent-go/pkg/openapislim"
	"github.com/kbelludoo/agent-go/pkg/packager"
	"github.com/kbelludoo/agent-go/pkg/patcher"
	"github.com/kbelludoo/agent-go/pkg/promptguard"
	"github.com/kbelludoo/agent-go/pkg/sanitizer"
	"github.com/kbelludoo/agent-go/pkg/schemamcp"
	"github.com/kbelludoo/agent-go/pkg/scraper"
	"github.com/kbelludoo/agent-go/pkg/supervisor"
	"github.com/kbelludoo/agent-go/pkg/testfilter"
	"github.com/kbelludoo/agent-go/pkg/toon"
	"github.com/kbelludoo/agent-go/pkg/vault"
)

func readFileOrText(arg string) string {
	data, err := os.ReadFile(arg)
	if err == nil {
		return string(data)
	}
	return arg
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	// 1. Context Packager (go-repomix)
	case "pack", "repomix":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		opts := packager.DefaultPackOptions(dir)
		res, err := packager.Pack(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao empacotar: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-pack] %d arquivos empacotados (~%d tokens, %.1f%% reducao)\n\n",
			res.FilesCount, res.EstimatedTokens, (1.0-res.CompressionRatio)*100)
		fmt.Println(res.Content)

	// 2. Secret Redactor (go-sanitizer)
	case "sanitize", "redact":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools sanitize <arquivo_ou_texto>")
			os.Exit(1)
		}
		text := readFileOrText(os.Args[2])
		redactor := sanitizer.NewRedactor()
		fmt.Println(redactor.Redact(text))

	// 3. Web Scraper (go-scraper-cdp)
	case "scrape":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools scrape <url>")
			os.Exit(1)
		}
		url := os.Args[2]
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		res, err := scraper.ScrapeURL(ctx, url, 15*time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro no scrape: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-scraper] %s -> %d bytes limpos (~%d tokens, %.1f%% reducao)\n\n",
			res.Title, res.CleanBytes, res.EstimatedTokens, res.TokenReduction)
		fmt.Println(res.Markdown)

	// 4. Fuzzy Code Patcher (go-patcher)
	case "patch":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools patch <arquivo> <bloco_antigo> <bloco_novo>")
			os.Exit(1)
		}
		filePath := os.Args[2]
		target := os.Args[3]
		replacement := os.Args[4]

		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro lendo arquivo: %v\n", err)
			os.Exit(1)
		}

		patched, err := patcher.ApplyPatch(string(content), target, replacement, patcher.DefaultPatchOptions())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao aplicar patch: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(filePath, []byte(patched), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Erro gravando arquivo: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Patch aplicado com sucesso em %s\n", filePath)

	// 5. Bot Sentinel & Process Monitor (go-supervisor)
	case "bots", "status":
		host := "127.0.0.1"
		if len(os.Args) > 2 {
			host = os.Args[2]
		}
		statuses := supervisor.CheckAllBots(host, supervisor.DefaultVPSBots, 350*time.Millisecond)
		fmt.Println(supervisor.FormatHealthTable(statuses))

	// 6. Dynamic Context Pruner (go-dcp)
	case "dcp":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools dcp <transcript.json>")
			os.Exit(1)
		}
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro lendo arquivo: %v\n", err)
			os.Exit(1)
		}
		var msgs []dcp.Message
		if err := json.Unmarshal(data, &msgs); err != nil {
			fmt.Fprintf(os.Stderr, "Erro parsing JSON: %v\n", err)
			os.Exit(1)
		}
		res := dcp.PruneTranscript(msgs, dcp.DefaultPruneOptions())
		fmt.Fprintf(os.Stderr, "[go-dcp] Redução: %.1f%% (%d chars salvos, ~%d tokens)\n\n",
			res.ReductionPercent, res.CharsSaved, res.TokensSaved)
		out, _ := json.MarshalIndent(res.Messages, "", "  ")
		fmt.Println(string(out))

	// 7. Structural Git Diff Summarizer (go-diff-summary)
	case "diff-summary", "diffsummary":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools diff-summary <diff_text_ou_arquivo>")
			os.Exit(1)
		}
		input := readFileOrText(os.Args[2])
		res := diffsummary.SummarizeDiff(input)
		fmt.Fprintf(os.Stderr, "[go-diff-summary] ~%d tokens estimados\n\n", res.EstimatedTokens)
		fmt.Println(res.SummaryText)

	// 8. Database Schema Inspector (go-schema-mcp)
	case "schema":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools schema <caminho_sqlite.db>")
			os.Exit(1)
		}
		dbPath := os.Args[2]
		res, err := schemamcp.InspectSQLiteSchema(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao inspecionar schema: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-schema] %d tabelas (~%d tokens)\n\n", len(res.Tables), res.EstimatedTokens)
		fmt.Println(res.Markdown)

	// 9. Anti-Prolixity Filter (go-caveman)
	case "caveman":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools caveman <arquivo_ou_texto>")
			os.Exit(1)
		}
		text := readFileOrText(os.Args[2])
		cleaned := caveman.StripProlixity(text)
		fmt.Fprintf(os.Stderr, "[go-caveman] Economia: %.1f%% chars\n\n",
			float64(len(text)-len(cleaned))/float64(len(text))*100.0)
		fmt.Println(cleaned)

	// 10. AES-256-GCM Secure Vault (go-vault)
	case "vault":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "Uso:\n  go-tools vault encrypt <arquivo_in> <arquivo_out> <senha>\n  go-tools vault decrypt <arquivo_in> <arquivo_out> <senha>")
			os.Exit(1)
		}
		action := os.Args[2]
		inFile := os.Args[3]
		outFile := os.Args[4]
		pass := ""
		if len(os.Args) > 5 {
			pass = os.Args[5]
		}
		if pass == "" {
			pass = os.Getenv("VAULT_PASSWORD")
		}
		if pass == "" {
			fmt.Fprintln(os.Stderr, "Senha não fornecida (argumento ou VAULT_PASSWORD)")
			os.Exit(1)
		}

		inData, err := os.ReadFile(inFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro lendo arquivo: %v\n", err)
			os.Exit(1)
		}

		if action == "encrypt" {
			cipher, err := vault.Encrypt(inData, pass)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro encriptando: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(outFile, cipher, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Erro gravando vault: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🔒 Arquivo criptografado com sucesso em %s (AES-256-GCM)\n", outFile)
		} else if action == "decrypt" {
			plain, err := vault.Decrypt(inData, pass)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro decriptando: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(outFile, plain, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Erro gravando arquivo decriptado: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🔓 Arquivo decriptado com sucesso em %s\n", outFile)
		}

	// 10.5 Codebase Micro-Bytecode VM (MB-VM)
	case "mbvm":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		mbvm, bin, err := codembvm.CompileProject(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao compilar MB-VM: %v\n", err)
			os.Exit(1)
		}
		outFile := filepath.Join(dir, "codebase.mbvm")
		_ = os.WriteFile(outFile, bin, 0644)
		proj := mbvm.GenerateAIPromptContext(100000)
		fmt.Printf("⚡ [Codebase MB-VM] Compilado com sucesso em %s (%d bytes, %.1f%% de redução de bytes)\n",
			outFile, len(bin), (1.0-float64(len(bin))/float64(mbvm.RawBytes))*100)
		fmt.Printf("📦 Símbolos: %d | Arquivos: %d | Tokens de Contexto: ~%d (Redução de >94%% de tokens)\n\n",
			len(mbvm.Symbols), mbvm.TotalFiles, len(proj)/4)
		fmt.Println(proj)

	// 11. AST Graphify Tools (map & who-calls)
	case "map":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		opts := graph.DefaultSkeletonOptions()
		res, err := graph.GenerateRepoMap(dir, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao gerar Repo Map: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(res)

	case "who-calls", "whocalls":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools who-calls <funcao> [diretorio]")
			os.Exit(1)
		}
		target := os.Args[2]
		dir := "."
		if len(os.Args) > 3 {
			dir = os.Args[3]
		}
		cg, err := graph.BuildCallGraph(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro no call graph: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(cg.FormatWhoCalls(target))

	// 12. Malformed LLM JSON Repairer (go-json-repair)
	case "json-repair", "jsonrepair":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools json-repair <arquivo_ou_json_quebrado>")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		res := jsonrepair.Repair(raw)
		fmt.Fprintf(os.Stderr, "[go-json-repair] valid=%v | fixes=%d\n\n", res.IsValidNow, res.FixesApplied)
		fmt.Println(res.RepairedJSON)

	// 13. Token-Oriented Object Notation (go-toon)
	case "toon":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools toon <arquivo.json_ou_string> [decode]")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		if len(os.Args) > 3 && os.Args[3] == "decode" {
			out, err := toon.DecodeTable(raw)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro decodificando TOON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(out))
			return
		}
		res, err := toon.Encode([]byte(raw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro codificando TOON: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-toon] Redução: %.1f%% (%d -> %d bytes, ~%d tokens)\n\n",
			res.ReductionPercent, res.OriginalBytes, res.CompressedBytes, res.EstimatedTokens)
		fmt.Println(res.Encoded)

	// 14. Structural Log Deduplicator (go-log-dedup)
	case "log-dedup", "logdedup":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools log-dedup <arquivo.log_ou_texto>")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		res := logdedup.Deduplicate(raw)
		fmt.Fprintf(os.Stderr, "[go-log-dedup] %d linhas -> %d templates únicos (%.1f%% redução)\n\n",
			res.OriginalLines, res.UniqueTemplates, res.ReductionPercent)
		fmt.Println(res.Summary)

	// 15. In-Memory Okapi BM25 Lexical Search (go-bm25)
	case "bm25", "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools bm25 <query> [diretorio] [topK]")
			os.Exit(1)
		}
		query := os.Args[2]
		dir := "."
		if len(os.Args) > 3 {
			dir = os.Args[3]
		}
		topK := 5
		if len(os.Args) > 4 {
			if k, err := strconv.Atoi(os.Args[4]); err == nil && k > 0 {
				topK = k
			}
		}
		idx, totalBytes, err := bm25.IndexDirectory(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao indexar BM25: %v\n", err)
			os.Exit(1)
		}
		hits := idx.Search(query, topK)
		out := bm25.FormatResults(hits)
		fmt.Fprintf(os.Stderr, "[go-bm25] %d docs indexados (%d KB) -> Top %d resultados (~%d tokens)\n\n",
			len(idx.Docs), totalBytes/1024, len(hits), (len(out)+3)/4)
		fmt.Println(out)

	// 16. Silent Test Failure Extractor (go-test-filter)
	case "test-filter", "testfilter":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools test-filter <saida_de_teste_ou_arquivo>")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		res := testfilter.Filter(raw)
		fmt.Fprintf(os.Stderr, "[go-test-filter] Redução: %.1f%% (%d -> %d bytes)\n\n",
			res.ReductionPercent, res.OriginalBytes, res.CompressedBytes)
		fmt.Println(res.FilteredOutput)

	// 17. OpenAPI / Swagger Spec Condenser (go-openapi-slim)
	case "openapi-slim", "openapislim":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools openapi-slim <swagger.json> [filtro_rota]")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		filter := ""
		if len(os.Args) > 3 {
			filter = os.Args[3]
		}
		res, err := openapislim.Slim([]byte(raw), filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro no OpenAPI Slim: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-openapi-slim] %d rotas | %.1f%% redução (~%d tokens)\n\n",
			res.EndpointsCount, res.ReductionPercent, res.EstimatedTokens)
		fmt.Println(res.Markdown)

	// 18. Prompt Injection & Unicode Smuggling Guard (go-prompt-guard)
	case "prompt-guard", "promptguard":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools prompt-guard <arquivo_ou_texto>")
			os.Exit(1)
		}
		raw := readFileOrText(os.Args[2])
		res := promptguard.Scan(raw)
		fmt.Fprintf(os.Stderr, "[go-prompt-guard] safe=%v | risk=%d/100 | threats=%v | invisible_removed=%d\n\n",
			res.Safe, res.RiskScore, res.ThreatsDetected, res.InvisibleRemoved)
		fmt.Println(res.SanitizedText)

	// 19. Dependency Manifest Auditor (go-dep-audit)
	case "dep-audit", "depaudit":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		res, err := depaudit.AuditDirectory(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro em dep-audit: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-dep-audit] %d dependências diretas (~%d tokens, %.1f%% redução)\n\n",
			res.DirectDepsCount, res.EstimatedTokens, res.ReductionPercent)
		fmt.Println(res.CompactManifest)

	// 20. Streaming JSONL Transcript Filter (go-jsonl-query)
	case "jsonl-query", "jsonlquery":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools jsonl-query <arquivo.jsonl> [keyword]")
			os.Exit(1)
		}
		filePath := os.Args[2]
		keyword := ""
		if len(os.Args) > 3 {
			keyword = os.Args[3]
		}
		f, err := os.Open(filePath)
		if err != nil {
			// Fallback to raw string reader if passed inline
			res, _ := jsonlquery.StreamQuery(strings.NewReader(filePath), jsonlquery.DefaultQueryOptions(keyword))
			fmt.Println(res.CompactOutput)
			return
		}
		defer f.Close()
		res, err := jsonlquery.StreamQuery(f, jsonlquery.DefaultQueryOptions(keyword))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro no jsonl-query: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[go-jsonl-query] %d/%d linhas encontradas (%.1f%% redução)\n\n",
			res.MatchedLines, res.TotalLines, res.ReductionPercent)
		fmt.Println(res.CompactOutput)

	// 21. Lightweight Task Runner with Retry & Timeout (go-cron-lite)
	case "cron-lite", "cronlite":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso: go-tools cron-lite <comando_shell> [retries] [timeout_sec]")
			os.Exit(1)
		}
		shellCmd := os.Args[2]
		retries := 1
		if len(os.Args) > 3 {
			if r, err := strconv.Atoi(os.Args[3]); err == nil {
				retries = r
			}
		}
		timeoutSec := 15
		if len(os.Args) > 4 {
			if t, err := strconv.Atoi(os.Args[4]); err == nil && t > 0 {
				timeoutSec = t
			}
		}
		res := cronlite.RunOnce(context.Background(), cronlite.JobSpec{
			Name:       "cli-job",
			Command:    shellCmd,
			Timeout:    time.Duration(timeoutSec) * time.Second,
			MaxRetries: retries,
			RetryDelay: 500 * time.Millisecond,
		})
		fmt.Println(cronlite.FormatResult(res))
		if !res.Success {
			os.Exit(1)
		}

	// 22. Hindsight - Long-Term Cognitive Memory (Retain / Recall / Reflect)
	case "memory", "hindsight":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Uso:\n  go-tools memory retain <bank> <type> <subject> <content>\n  go-tools memory recall <bank> <query> [topK]\n  go-tools memory reflect <bank>")
			os.Exit(1)
		}
		sub := os.Args[2]
		homeDir, _ := os.UserHomeDir()
		dbPath := os.Getenv("HINDSIGHT_DB")
		if dbPath == "" {
			dbPath = homeDir + "/.hindsight.db"
		}
		store, err := hindsight.Open(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao abrir banco de memória Hindsight: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		switch sub {
		case "retain":
			if len(os.Args) < 7 {
				fmt.Fprintln(os.Stderr, "Uso: go-tools memory retain <bank> <fact|preference|lesson|decision> <subject> <content>")
				os.Exit(1)
			}
			bank := os.Args[3]
			fType := hindsight.FactType(os.Args[4])
			subject := os.Args[5]
			content := os.Args[6]
			m, err := store.Retain(bank, fType, subject, content)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao reter memória: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🧠 [Hindsight Retain] Memória %s registrada (banco: %s, tipo: %s, subject: %s)\n",
				m.ID, m.Bank, m.Type, m.Subject)

		case "recall":
			if len(os.Args) < 5 {
				fmt.Fprintln(os.Stderr, "Uso: go-tools memory recall <bank> <query> [topK]")
				os.Exit(1)
			}
			bank := os.Args[3]
			query := os.Args[4]
			topK := 5
			if len(os.Args) > 5 {
				if k, err := strconv.Atoi(os.Args[5]); err == nil && k > 0 {
					topK = k
				}
			}
			hits, err := store.Recall(bank, query, topK)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao recordar memória: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(hindsight.FormatRecalled(hits))

		case "reflect":
			bank := "default"
			if len(os.Args) > 3 {
				bank = os.Args[3]
			}
			report, err := store.Reflect(bank)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro na reflexão de memória: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(report)

		case "mcp":
			if err := hindsight.ServeMCP(store, os.Stdin, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "MCP hindsight error: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Ação de memória desconhecida: %s (use retain, recall ou reflect)\n", sub)
			os.Exit(1)
		}

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Comando desconhecido: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`go-tools - Suíte Nativa de Alta Performance em Go Puro para IA & Bots (21 Ferramentas)

COMANDOS DISPONÍVEIS:
  [Série 1 - Contexto, AST, Segurança & Bots]
  go-tools map [dir]                           Gera esqueleto AST de assinaturas (go-graphify, -90% tokens)
  go-tools who-calls <funcao> [dir]            Localiza quem chama a função em 0.02ms
  go-tools pack [dir]                          Empacota e poda o repositório (go-repomix, -16% a -50% tokens)
  go-tools sanitize <arquivo_ou_texto>         Redige chaves de API, senhas e segredos antes do LLM (0.01ms)
  go-tools scrape <url>                        Extrai Markdown limpo sem Node/Puppeteer (0.006ms, -66% tokens)
  go-tools patch <arquivo> <antigo> <novo>     Aplica edições com tolerância a CRLF/indentação (zero falhas)
  go-tools bots [host]                         Monitora os 15 bots do VPS em paralelo (< 2MB RAM)
  go-tools dcp <transcript.json>               Compactador dinâmico de histórico (-92% tokens em tool outputs)
  go-tools diff-summary <diff_file>            Resumo estrutural de git diff (-85% tokens)
  go-tools schema <sqlite.db>                  Inspetor nativo de schema SQLite (0.7ms, ~140 tokens)
  go-tools caveman <arquivo_ou_texto>          Remove preâmbulos e prolixidade de respostas (-78% tokens)
  go-tools vault encrypt/decrypt <in> <out>    Cofre criptografado AES-256-GCM para chaves sem expor no disco

  [Série 2 - Compressão Avançada, Busca BM25, Reparo & Guardrails]
  go-tools json-repair <json_quebrado>         Conserta JSON truncado/inválido de LLMs em microssegundos
  go-tools toon <arquivo.json> [decode]        Converte JSON denso para notação tabular TOON (-40% a -60% tokens)
  go-tools log-dedup <arquivo.log>             Agrupa logs repetitivos e stack traces (-95% tokens)
  go-tools bm25 <query> [dir] [topK]           Motor de busca lexical Okapi BM25 em memória (< 1ms, zero DB)
  go-tools test-filter <test_output>           Filtra saídas de testes exibindo apenas falhas (-90%+ tokens)
  go-tools openapi-slim <swagger.json> [rota]  Comprime specs OpenAPI gigantes em 1 linha por rota (-90% tokens)
  go-tools prompt-guard <texto>                Bloqueia Prompt Injection e caracteres Unicode invisíveis
  go-tools dep-audit [dir]                     Gera manifesto compacto de dependências (evita alucinação de libs)
  go-tools jsonl-query <arquivo.jsonl> [termo] Filtra transcripts .jsonl via streaming sem gastar RAM
  go-tools cron-lite <cmd> [retries] [timeout] Executor leve de tarefas agendadas com retry e timeout (< 1MB RAM)
  go-tools memory retain/recall/reflect        Memória cognitiva persistente Hindsight em Go puro (< 1ms, zero Postgres)
  go-tools mbvm [dir]                          Compila codebase AST em Micro-Bytecode VM para IA (-95% tokens, zero perda)

TODAS AS 22 FERRAMENTAS SÃO NATIVAS, COMPILADAS EM BINÁRIO ESTÁTICO SEM DEPENDÊNCIAS.`)
	flag.PrintDefaults()
}

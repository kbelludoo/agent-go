package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kbelludoo/agent-go/pkg/memory/wiki"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	// Global or local memory directory (defaults to .ai-memory in current directory)
	memoryDir := os.Getenv("AI_MEMORY_DIR")
	if memoryDir == "" {
		memoryDir = ".ai-memory"
	}

	switch command {
	case "init":
		cmdInit(memoryDir)

	case "index":
		cmdIndex(memoryDir)

	case "search":
		cmdSearch(memoryDir, os.Args[2:])

	case "remember":
		cmdRemember(memoryDir, os.Args[2:])

	case "page":
		cmdPage(memoryDir, os.Args[2:])

	case "handoff":
		cmdHandoff(memoryDir, os.Args[2:])

	case "install-hooks":
		cmdInstallHooks()

	case "mcp":
		cmdMCP(memoryDir)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Comando desconhecido: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("================================================================")
	fmt.Println("   AI-Memory (Go) - Git-Backed Knowledge Wiki & Cross-Agent Handoff")
	fmt.Println("   Compatível com: Antigravity, Cursor, OpenCode e Agent-Go")
	fmt.Println("================================================================")
	fmt.Println("Uso: ai-memory <comando> [argumentos]")
	fmt.Println("")
	fmt.Println("Comandos disponíveis:")
	fmt.Println("  init                     Inicializa o repositório .ai-memory/ com templates e FTS5")
	fmt.Println("  index                    Varre e indexa todos os arquivos .md no SQLite FTS5")
	fmt.Println("  search <termo>           Busca de texto completo ultra-rápida via FTS5 BM25")
	fmt.Println("  remember [opções] <texto> Registra um novo conhecimento ou decisão arquitetural")
	fmt.Println("  page list                Lista todas as páginas Markdown do repositório")
	fmt.Println("  page read <caminho>      Exibe o conteúdo completo de uma página")
	fmt.Println("  handoff [read|write]     Lê ou escreve o pacote de transição entre agentes")
	fmt.Println("  install-hooks            Gera regras para Antigravity (.gemini), Cursor (.cursor) e OpenCode")
	fmt.Println("  mcp                      Inicia o servidor MCP stdio (para IDEs e agentes)")
	fmt.Println("")
	fmt.Println("Exemplos:")
	fmt.Println("  ai-memory init")
	fmt.Println("  ai-memory search \"vps bot memory\"")
	fmt.Println("  ai-memory remember --title \"Timeout HTTP\" \"Sempre definir timeout no client.\"")
	fmt.Println("  ai-memory handoff read")
	fmt.Println("  ai-memory install-hooks")
}

func getStore(dir string) *wiki.Store {
	store, err := wiki.NewStore(dir)
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível abrir o banco de memória em %s: %v\n", dir, err)
		os.Exit(1)
	}
	return store
}

func cmdInit(dir string) {
	fmt.Printf("[ai-memory] Inicializando repositório de memória em: %s\n", dir)
	store := getStore(dir)
	defer store.Close()

	if err := store.Init(); err != nil {
		fmt.Printf("[ERRO] Falha ao inicializar templates: %v\n", err)
		os.Exit(1)
	}

	pages, _ := store.ListPages()
	fmt.Printf("[Sucesso] Repositório inicializado com %d páginas Markdown prontas e indexadas!\n", len(pages))
	for _, p := range pages {
		fmt.Printf("  - %s\n", p)
	}
}

func cmdIndex(dir string) {
	store := getStore(dir)
	defer store.Close()

	fmt.Printf("[ai-memory] Sincronizando e re-indexando páginas em %s...\n", dir)
	count, err := store.IndexAll()
	if err != nil {
		fmt.Printf("[ERRO] Falha ao indexar: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[Sucesso] %d página(s) indexadas com sucesso no SQLite FTS5!\n", count)
}

func cmdSearch(dir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Uso: ai-memory search \"<termo de busca>\"")
		os.Exit(1)
	}

	query := strings.Join(args, " ")
	store := getStore(dir)
	defer store.Close()

	results, err := store.Search(query, 10)
	if err != nil {
		fmt.Printf("[ERRO] Falha na busca FTS5: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("Nenhum resultado encontrado para: %q\n", query)
		return
	}

	fmt.Printf("\nResultados da Busca para: %q (%d encontrados):\n", query, len(results))
	fmt.Println("----------------------------------------------------------------")
	for i, r := range results {
		fmt.Printf("[%d] %s (%s) - Score BM25: %.2f\n", i+1, r.Title, r.Path, r.Score)
		if len(r.Tags) > 0 {
			fmt.Printf("    Tags: #%s\n", strings.Join(r.Tags, " #"))
		}
		if len(r.Links) > 0 {
			fmt.Printf("    Links: [[%s]]\n", strings.Join(r.Links, "]] [[" ))
		}
		fmt.Printf("    Trecho: %s\n\n", r.Snippet)
	}
}

func cmdRemember(dir string, args []string) {
	fs := flag.NewFlagSet("remember", flag.ExitOnError)
	title := fs.String("title", "", "Título da nota ou decisão")
	tagsStr := fs.String("tags", "", "Tags separadas por vírgula (ex: vps,watchdog,leak)")
	_ = fs.Parse(args)

	content := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(content) == "" {
		fmt.Println("Uso: ai-memory remember [--title \"...\"] [--tags \"...\"] \"conteúdo da nota\"")
		os.Exit(1)
	}

	var tags []string
	if *tagsStr != "" {
		for _, part := range strings.Split(*tagsStr, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	if *title == "" {
		words := strings.Fields(content)
		if len(words) > 5 {
			*title = strings.Join(words[:5], " ") + "..."
		} else {
			*title = strings.Join(words, " ")
		}
	}

	store := getStore(dir)
	defer store.Close()

	relPath, err := store.Remember("", *title, content, tags)
	if err != nil {
		fmt.Printf("[ERRO] Falha ao salvar memória: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[Sucesso] Memória gravada em: %s e indexada no FTS5!\n", relPath)
}

func cmdPage(dir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Uso: ai-memory page <list|read> [caminho]")
		os.Exit(1)
	}

	sub := args[0]
	store := getStore(dir)
	defer store.Close()

	switch sub {
	case "list":
		pages, err := store.ListPages()
		if err != nil {
			fmt.Printf("[ERRO] Falha ao listar páginas: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Páginas na Wiki (%d):\n", len(pages))
		for _, p := range pages {
			fmt.Printf("  - %s\n", p)
		}

	case "read":
		if len(args) < 2 {
			fmt.Println("Uso: ai-memory page read <caminho.md>")
			os.Exit(1)
		}
		page, err := store.GetPage(args[1])
		if err != nil {
			fmt.Printf("[ERRO] Falha ao ler página: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("----------------------------------------------------------------")
		fmt.Printf("Documento: %s (Título: %s)\n", page.Path, page.Title)
		fmt.Println("----------------------------------------------------------------")
		fmt.Println(page.Content)

	default:
		fmt.Printf("Subcomando de página desconhecido: %s\n", sub)
		os.Exit(1)
	}
}

func cmdHandoff(dir string, args []string) {
	store := getStore(dir)
	defer store.Close()

	action := "read"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "read", "show":
		packet, err := store.LoadHandoff()
		if err != nil {
			fmt.Printf("[ERRO] Falha ao ler handoff: %v\n", err)
			os.Exit(1)
		}
		if packet == nil {
			fmt.Println("Nenhum pacote de handoff registrado ainda.")
			return
		}
		fmt.Println(wiki.RenderHandoffContext(packet))

	case "write", "save":
		fs := flag.NewFlagSet("handoff write", flag.ExitOnError)
		agent := fs.String("agent", "cli", "Nome do agente (antigravity, cursor, opencode, agent-go)")
		task := fs.String("task", "", "Objetivo ou tarefa principal")
		notes := fs.String("notes", "", "Notas técnicas importantes")
		completedStr := fs.String("completed", "", "Etapas concluídas separadas por ponto-e-vírgula")
		nextStr := fs.String("next", "", "Próximas etapas separadas por ponto-e-vírgula")
		_ = fs.Parse(args[1:])

		var completed, nextSteps []string
		if *completedStr != "" {
			for _, part := range strings.Split(*completedStr, ";") {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					completed = append(completed, trimmed)
				}
			}
		}
		if *nextStr != "" {
			for _, part := range strings.Split(*nextStr, ";") {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					nextSteps = append(nextSteps, trimmed)
				}
			}
		}

		packet := wiki.HandoffPacket{
			Agent:          *agent,
			CurrentTask:    *task,
			CompletedSteps: completed,
			NextSteps:      nextSteps,
			ContextNotes:   *notes,
		}

		if err := store.SaveHandoff(packet); err != nil {
			fmt.Printf("[ERRO] Falha ao salvar handoff: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[Sucesso] Handoff registrado por %s em .ai-memory/handoff.json e sessions/!\n", *agent)

	default:
		fmt.Printf("Ação de handoff desconhecida: %s (use 'read' ou 'write')\n", action)
		os.Exit(1)
	}
}

func cmdInstallHooks() {
	fmt.Println("[ai-memory] Instalando regras e configurações para Antigravity, Cursor e OpenCode...")
	installed, err := wiki.InstallHooks(".")
	if err != nil {
		fmt.Printf("[ERRO] Falha ao instalar hooks: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nArquivos de configuração e regras instalados com sucesso:")
	for _, f := range installed {
		fmt.Printf("  ✅ %s\n", f)
	}
	fmt.Println("\nTodos os agentes (Antigravity, Cursor, OpenCode e Agent-Go) agora compartilham a mesma memória!")
}

func cmdMCP(dir string) {
	store := getStore(dir)
	defer store.Close()

	if err := wiki.ServeMCP(store, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

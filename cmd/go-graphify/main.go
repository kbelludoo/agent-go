package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kbelludoo/agent-go/pkg/graph"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
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
			fmt.Fprintln(os.Stderr, "Uso: go-graphify who-calls <nome_da_funcao> [diretorio]")
			os.Exit(1)
		}
		target := os.Args[2]
		dir := "."
		if len(os.Args) > 3 {
			dir = os.Args[3]
		}
		cg, err := graph.BuildCallGraph(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao construir Call Graph: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(cg.FormatWhoCalls(target))

	case "mcp":
		if err := graph.ServeMCP(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Comando desconhecido: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`go-graphify - Knowledge Graph de AST e Skeletonizer de Código em Go Puro

USO COMO CLI DIRETO:
  go-graphify map [diretorio]               Gera o RepoMap comprimido (esqueleto AST com 90% menos tokens)
  go-graphify who-calls <funcao> [dir]      Localiza instantaneamente quem chama a função (em 0.02ms)

USO COMO SERVIDOR MCP (Model Context Protocol):
  go-graphify mcp                           Inicia servidor stdio MCP para Cursor, OpenCode, Claude Code e Antigravity

EXEMPLOS:
  go-graphify map ./pkg
  go-graphify who-calls NewDedicatedIPClient ./pkg
  go-graphify mcp`)
	flag.PrintDefaults()
}

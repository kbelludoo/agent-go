package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSkeletonizeAndCallGraph(t *testing.T) {
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "service.go")
	srcA := `package testpkg

import "fmt"

type Config struct {
	Port int
	Host string
}

type Service interface {
	Start() error
}

type Server struct {
	cfg Config
}

func NewServer(cfg Config) *Server {
	fmt.Println("initializing...")
	return &Server{cfg: cfg}
}

func (s *Server) Start() error {
	HelperLog("starting server")
	return nil
}

func HelperLog(msg string) {
	fmt.Println(msg)
}
`
	if err := os.WriteFile(fileA, []byte(srcA), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Test Skeleton Generation
	repoMap, err := GenerateRepoMap(tmpDir, DefaultSkeletonOptions())
	if err != nil {
		t.Fatalf("GenerateRepoMap error: %v", err)
	}

	// Ensure bodies were stripped
	if strings.Contains(repoMap, "initializing...") {
		t.Error("Corpo da função NewServer não foi removido!")
	}
	if strings.Contains(repoMap, "starting server") {
		t.Error("Corpo do método Start não foi removido!")
	}
	if !strings.Contains(repoMap, "type Config struct") {
		t.Error("Struct Config foi perdida!")
	}
	if !strings.Contains(repoMap, "func NewServer(cfg Config) *Server") {
		t.Error("Assinatura de NewServer foi perdida!")
	}

	// 2. Test Call Graph (Graphify in Go)
	cg, err := BuildCallGraph(tmpDir)
	if err != nil {
		t.Fatalf("BuildCallGraph error: %v", err)
	}

	whoCallsHelper := cg.WhoCalls("HelperLog")
	if len(whoCallsHelper) == 0 {
		t.Fatal("Esperava chamada para HelperLog mas encontrou 0")
	}

	formatted := cg.FormatWhoCalls("HelperLog")
	if !strings.Contains(formatted, "Server.Start") {
		t.Errorf("Esperava chamada de Server.Start para HelperLog, obteve:\n%s", formatted)
	}
}

func TestRealCodebaseBenchmarkAndTokenCompression(t *testing.T) {
	root := `../../pkg`
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("Diretório pkg não encontrado para benchmark")
	}

	start := time.Now()
	repoMap, err := GenerateRepoMap(root, DefaultSkeletonOptions())
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Erro ao gerar Repo Map: %v", err)
	}

	mapChars := len(repoMap)
	estTokens := mapChars / 4

	t.Logf("⚡ [PERFORMANCE COMPROVADA] RepoMap gerado em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] Tamanho do esqueleto: %d chars (~%d tokens)", mapChars, estTokens)

	startCG := time.Now()
	cg, err := BuildCallGraph(root)
	cgDuration := time.Since(startCG)
	if err != nil {
		t.Fatalf("Erro ao construir Call Graph: %v", err)
	}

	t.Logf("⚡ [GRAPHIFY SPEED] Call Graph construído em: %v (%d declarações, %d targets)", cgDuration, len(cg.Declarations), len(cg.Reverse))

	queryStart := time.Now()
	whoCalls := cg.FormatWhoCalls("NewDedicatedIPClient")
	queryDuration := time.Since(queryStart)

	t.Logf("⚡ [TEMPO DE CONSULTA] WhoCalls('NewDedicatedIPClient') respondido em: %v (~%d tokens)", queryDuration, len(whoCalls)/4)
	t.Logf("Resultado da consulta:\n%s", whoCalls)
}

func BenchmarkCallGraphQuery(b *testing.B) {
	root := `../../pkg`
	cg, err := BuildCallGraph(root)
	if err != nil {
		b.Skip("BuildCallGraph error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cg.WhoCalls("NewDedicatedIPClient")
	}
}

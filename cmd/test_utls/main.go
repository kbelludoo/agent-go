package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kbelludoo/agent-go/pkg/navigator"
)

func main() {
	session := navigator.NewSession()

	targets := []string{
		"https://www.inpe.br",
		"https://azure.microsoft.com/blog/",
		"https://events.linuxfoundation.org/sponsor-AAIF-AgentsForums26?utm_source=luma",
	}

	for _, target := range targets {
		fmt.Printf("\n🌐 Testando Self-Healing em %s...\n", target)
		tCtx, tCancel := context.WithTimeout(context.Background(), 15*time.Second)
		start := time.Now()
		snap, err := session.Navigate(tCtx, target)
		tCancel()
		if err != nil {
			fmt.Printf("❌ Erro: %v\n", err)
			continue
		}
		fmt.Printf("✅ Sucesso! Status: %d | Latência: %dms | Título: %q | Bruto: %d KB -> Markdown: %d KB (%.1f%% red.)\n",
			snap.StatusCode, time.Since(start).Milliseconds(), snap.Title, snap.RawBytes/1024, snap.MarkdownBytes/1024, snap.TokenReduction)
	}
}


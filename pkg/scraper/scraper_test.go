package scraper

import (
	"strings"
	"testing"
	"time"
)

func TestExtractReadable(t *testing.T) {
	htmlData := `<!DOCTYPE html>
<html>
<head>
	<title>Documentação da API do Go Bot</title>
	<style>
		body { background: #fff; color: #333; margin: 0; }
		.sidebar { width: 250px; display: flex; }
	</style>
	<script>
		console.log("analytics tracking code...");
		window.ga = window.ga || function(){};
	</script>
</head>
<body>
	<header>
		<nav>
			<a href="/home">Home</a> | <a href="/docs">Docs</a>
		</nav>
	</header>
	<main>
		<h1>Guia de Otimização de Bots</h1>
		<p>Este guia explica como executar bots de alta frequência usando zero overhead em Go.</p>
		<h2>Principais Benefícios</h2>
		<ul>
			<li>Menos de 2MB de memória RAM por bot</li>
			<li>Latência inferior a 1 milissegundo</li>
			<li>Sem dependência de Node.js ou Bun</li>
		</ul>
		<code>net.Dialer.LocalAddr = &net.TCPAddr{IP: ip}</code>
	</main>
	<footer>
		<p>© 2026 Todos os direitos reservados. Termos de uso e privacidade.</p>
	</footer>
</body>
</html>`

	start := time.Now()
	res, err := ExtractReadable(htmlData)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExtractReadable failed: %v", err)
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Extração de HTML para Markdown em: %v", duration)
	t.Logf("📊 [ECONOMIA DE TOKENS] HTML Original: %d bytes -> Markdown Limpo: %d bytes (Redução: %.2f%%)",
		res.RawBytes, res.CleanBytes, res.TokenReduction)
	t.Logf("Conteúdo Extraído:\n%s", res.Markdown)

	// Verifications
	if strings.Contains(res.Markdown, "analytics tracking code") {
		t.Error("Tag <script> não foi descartada!")
	}
	if strings.Contains(res.Markdown, "background: #fff") {
		t.Error("Tag <style> não foi descartada!")
	}
	if strings.Contains(res.Markdown, "Termos de uso e privacidade") {
		t.Error("Tag <footer> não foi descartada!")
	}
	if !strings.Contains(res.Markdown, "# Guia de Otimização de Bots") {
		t.Error("Título H1 foi perdido!")
	}
	if !strings.Contains(res.Markdown, "• Menos de 2MB de memória RAM por bot") {
		t.Error("Item de lista foi perdido!")
	}
}

func BenchmarkExtractReadable(b *testing.B) {
	htmlData := `<html><head><script>alert(1);</script></head><body><h1>Título</h1><p>Parágrafo com texto explicativo longo para teste de velocidade de parsing nativo em Go.</p></body></html>`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractReadable(htmlData)
	}
}

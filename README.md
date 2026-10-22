# ⚡ Agent-Go: Autonomous AI Programming & Bot Coprocessor

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Assembly](https://img.shields.io/badge/AVX2-x86--64%20Assembly-red?style=flat)](https://en.wikipedia.org/wiki/Advanced_Vector_Extensions)
[![SQLite](https://img.shields.io/badge/FTS5-BM25%20Wiki-blue?style=flat&logo=sqlite)](https://sqlite.org/fts5.html)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Agente autônomo e coprocessador de programação de ultra-baixo consumo (< 15 MB de RAM) desenvolvido em **Go nativo + Assembly AVX2 puro**, com memória Git-backed Markdown Wiki (FTS5 BM25), integração OpenCode headless e aceleração SIMD 1-bit.

---

## 🌟 Principais Recursos

### 1. 🧠 Kernels Numéricos em Assembly AVX2 (`pkg/neural/`)
- **Float32 Dot Product & Cosine Similarity**: Vetorização 256 bits com `VFMADD231PS` (~75 ns / ~264 ns).
- **INT8 Quantized Similarity**: `VPBROADCASTB`, `VPMADDUBSW` e `VPADDD`.
- **1-Bit 8K Bi-Encoder Hamming Similarity**: `VPXOR` + `VPOPCNT` em ~55 ns na VPS AMD EPYC.
- **Quantização Pentária / INT4**: Compactação extrema para pesos de redes neurais.

### 2. 📚 AI-Memory Git-Backed Markdown Wiki (`pkg/memory/wiki/`, `cmd/ai-memory`)
- **Fonte da Verdade no Git**: Todas as memórias e decisões técnicas residem em arquivos Markdown puros (`.ai-memory/`).
- **Zero LLM Overhead**: Índice SQLite FTS5 puro (sem CGO) com ranking BM25 e busca em **~278 microssegundos (0.27 ms)**.
- **Cross-Agent Handoff**: Transição transparente de contexto entre **Antigravity, Cursor, OpenCode e Agent-Go** (`handoff.json` + `sessions/`).
- **Servidor MCP Nativo (JSON-RPC 2.0)**: Suporta `memory_search`, `memory_remember`, `memory_read_page`, `memory_handoff_read` e `memory_handoff_write`.

### 3. 🌐 OpenCode Headless Daemon SDK (`pkg/opencode/`, `cmd/opencode-connect`)
- **Auto-Discovery Multiplataforma**: Detecta instâncias do OpenCode em execução (`0x70de` / 28894 no Windows, 49374 no Linux) via `service.json`.
- **Autenticação HMAC-SHA256**: Emissão e verificação de tokens de sessão com validade de 30 dias.
- **Server-Sent Events (SSE)**: Conexão contínua com `/api/event/feed`.

### 4. 🔀 Multi-Provider LLM Router (`pkg/router/`)
- Roteamento resiliente com fallback automático:
  1. OpenCode Headless Daemon (rotacionando contas)
  2. AgentRouter (`codex_cli_rs` client spoofing)
  3. Provedores upstream (Apinex, TokenHarbor, Groq, Mistral, OpenRouter)
- IP Binding rotativo para mitigação de rate limits.

---

## 🚀 Como Iniciar

### 1. Compilar o Agente e as Ferramentas CLI
```bash
# Compilar o Agente principal
go build -o agent-go.exe .

# Compilar o utilitário de memória
go build -o ai-memory.exe ./cmd/ai-memory

# Compilar o conector OpenCode
go build -o opencode-connect.exe ./cmd/opencode-connect
```

### 2. Usar o Sistema AI-Memory
```bash
# Fazer busca instantânea no repositório de conhecimento
.\ai-memory.exe search "assembly avx2 watchdog"

# Gravar uma nova decisão técnica
.\ai-memory.exe remember --title "Regra de Timeout" --tags "vps,http" "Sempre passar context.WithTimeout nas chamadas HTTP."

# Handoff entre agentes (ex: salvar progresso para o Cursor)
.\ai-memory.exe handoff write -agent "antigravity" -task "Otimizar socket" -completed "Keep-alive ajustado" -next "Testar na VPS"
```

### 3. Iniciar o Agente Completo
```bash
# No Windows:
.\agent-go.exe -serve 8888 -agent -router

# No Linux (VPS Oracle Cloud):
./agent-go-linux -serve 8888 -agent -router
```

---

## 🧪 Testes Automatizados

Executar toda a suíte de testes do projeto:
```bash
go test ./pkg/...
```

Executar os benchmarks de velocidade:
```bash
# Benchmark de Similaridade de 1-bit e AVX2
go test -bench=BenchmarkSimd ./pkg/neural/...

# Benchmark de Busca FTS5 BM25
go test -bench=BenchmarkWikiSearchFTS5 -benchmem ./pkg/memory/wiki/...
```

---

## 📄 Licença
Distribuído sob licença MIT. Criado por [kbelludoo](https://github.com/kbelludoo).

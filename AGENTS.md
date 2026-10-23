# Diretrizes de Memória Longa (Hindsight em Go Puro)

Este repositório utiliza o **Hindsight Nativo em Go Puro** (`go-tools memory`) para persistência de memória cognitiva entre sessões e agentes (Cursor, OpenCode, Antigravity, CLIs).

## Protocolo de Memória dos Agentes

### 1. Antes de Iniciar Tarefas Críticas (Auto-Recall)
Consulte a memória ativa antes de propor soluções ou alterar a infraestrutura para não repetir erros passados:
```bash
go-tools memory recall agent "<termo_da_tarefa>"
```
Ou via ferramenta MCP `hindsight_recall(bank="agent", query="...")`.

### 2. Ao Concluir um Fix ou Aprender uma Regra Nova (Auto-Retain)
Sempre que o usuário definir uma preferência de desenvolvimento ou após corrigir um bug de sistema/VPS, salve a lição atômica:
```bash
go-tools memory retain agent lesson <identificador_curto> "<descricao_da_regra_ou_correcao>"
```
Ou via ferramenta MCP `hindsight_retain(bank="agent", type="lesson", subject="...", content="...")`.

### 3. Diretrizes Ativas no Banco `agent`:
- **Preferência:** Sempre priorizar ferramentas em Go puro sem CGO ou Python/Node.js adicionais.
- **Lição VPS:** Na VPS Oracle, compilações Go devem definir `export GOTMPDIR=/home/ubuntu/tmp` para não esgotar o disco em memória `tmpfs` (/tmp).
- **Monitoramento:** A frota possui 15 bots em produção (portas 3000-3015 e 8090) e deve ser verificada via `go-tools bots 127.0.0.1`.

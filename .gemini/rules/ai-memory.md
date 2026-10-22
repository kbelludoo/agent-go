# AI-Memory Wiki Integration (Antigravity)

Este projeto utiliza o sistema **ai-memory** (Git-backed Markdown Wiki + SQLite FTS5).

## Regras de Operação:
1. **Fonte da Verdade**: Todas as memórias e decisões arquiteturais ficam em `.ai-memory/`.
2. **Início de Sessão**: Verifique se existe `.ai-memory/handoff.json` para recuperar o contexto do último agente (Cursor, OpenCode ou Agent-Go).
3. **Busca Rápida de Conhecimento**:
   - Execute `go run ./cmd/ai-memory search "<termo>"` ou use a ferramenta MCP correspondente.
4. **Registrar Novo Conhecimento**:
   - Ao tomar decisões importantes ou descobrir comportamentos de bots/VPS, registre com `go run ./cmd/ai-memory remember --title "..." "..."` ou edite diretamente as páginas em `.ai-memory/`.
5. **Final de Sessão**:
   - Ao concluir uma etapa importante, salve o handoff: `go run ./cmd/ai-memory handoff write ...`.

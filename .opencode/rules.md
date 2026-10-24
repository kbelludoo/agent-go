# OpenCode Rules: AI-Memory Cross-Agent Wiki

- **Fonte de Verdade**: As decisões técnicas e conhecimento acumulado ficam em `.ai-memory/`.
- **Handoff Automático**: Leia `.ai-memory/handoff.json` para prosseguir tarefas iniciadas por outros agentes.
- **Indexação**: Após alterar arquivos em `.ai-memory/`, execute `ai-memory index`.
- **MB-VM First**: Execute `go-tools mbvm .` para inspecionar a arquitetura inteira em ~10k tokens sem ler 200k tokens de arquivos .go.

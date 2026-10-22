# Registro de Decisões de Arquitetura (ADR)

#adr #decisions

## ADR-001: Memória Baseada em Markdown Git-Backed
- **Decisão**: Usar Markdown no estilo [[architecture]] como fonte da verdade, com SQLite FTS5 como índice derivado descartável.
- **Motivo**: Permite leitura direta por desenvolvedores, versionamento pelo Git e handoff limpo entre Antigravity, Cursor, OpenCode e Agent-Go.

## ADR-002: Assembly AVX2 para Dot Product e Hamming
- **Decisão**: Implementar operações de 1-bit e INT8 em Assembly x86-64 manual.
- **Motivo**: Executa em ~55 nanossegundos na VPS sem sobrecarregar a CPU.

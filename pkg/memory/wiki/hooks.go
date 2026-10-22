package wiki

import (
	"os"
	"path/filepath"
)

// InstallHooks generates configuration and instructions for Antigravity, Cursor, and OpenCode.
func InstallHooks(projectRoot string) ([]string, error) {
	if projectRoot == "" {
		projectRoot = "."
	}

	var installed []string

	// 1. Antigravity Rule: .gemini/rules/ai-memory.md
	geminiDir := filepath.Join(projectRoot, ".gemini", "rules")
	if err := os.MkdirAll(geminiDir, 0755); err == nil {
		content := `# AI-Memory Wiki Integration (Antigravity)

Este projeto utiliza o sistema **ai-memory** (Git-backed Markdown Wiki + SQLite FTS5).

## Regras de Operação:
1. **Fonte da Verdade**: Todas as memórias e decisões arquiteturais ficam em ` + "`.ai-memory/`" + `.
2. **Início de Sessão**: Verifique se existe ` + "`.ai-memory/handoff.json`" + ` para recuperar o contexto do último agente (Cursor, OpenCode ou Agent-Go).
3. **Busca Rápida de Conhecimento**:
   - Execute ` + "`go run ./cmd/ai-memory search \"<termo>\"`" + ` ou use a ferramenta MCP correspondente.
4. **Registrar Novo Conhecimento**:
   - Ao tomar decisões importantes ou descobrir comportamentos de bots/VPS, registre com ` + "`go run ./cmd/ai-memory remember --title \"...\" \"...\"`" + ` ou edite diretamente as páginas em ` + "`.ai-memory/`" + `.
5. **Final de Sessão**:
   - Ao concluir uma etapa importante, salve o handoff: ` + "`go run ./cmd/ai-memory handoff write ...`" + `.
`
		target := filepath.Join(geminiDir, "ai-memory.md")
		if err := os.WriteFile(target, []byte(content), 0644); err == nil {
			installed = append(installed, target)
		}
	}

	// 2. Cursor Rule: .cursor/rules/ai-memory.mdc
	cursorDir := filepath.Join(projectRoot, ".cursor", "rules")
	if err := os.MkdirAll(cursorDir, 0755); err == nil {
		content := `---
description: AI-Memory Cross-Agent Wiki & Handoff Integration
globs: *
alwaysApply: true
---

# AI-Memory (Cursor Agent Instructions)

You are operating within a project that uses **ai-memory** (Git-backed Markdown Wiki with SQLite FTS5).

## Guidelines:
1. **Read Active Context**: Check ` + "`.ai-memory/handoff.json`" + ` to pick up exactly where other agents (Antigravity, OpenCode, Agent-Go) left off.
2. **Knowledge Wiki**: Key project architecture, stack constraints, and decisions are stored as Markdown in ` + "`.ai-memory/`" + `.
3. **Fast Memory Search**: Run ` + "`ai-memory search \"<query>\"`" + ` or ` + "`go run ./cmd/ai-memory search \"<query>\"`" + ` to search past decisions and bot knowledge.
4. **Handoff On Task Complete**: Before concluding significant milestones, update ` + "`.ai-memory/handoff.json`" + ` or run ` + "`ai-memory handoff write`" + ` so the next agent has full context.
`
		target := filepath.Join(cursorDir, "ai-memory.mdc")
		if err := os.WriteFile(target, []byte(content), 0644); err == nil {
			installed = append(installed, target)
		}

		// Cursor MCP server definition
		mcpPath := filepath.Join(filepath.Dir(cursorDir), "mcp.json")
		if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
			mcpContent := `{
  "mcpServers": {
    "ai-memory": {
      "command": "go",
      "args": ["run", "./cmd/ai-memory", "mcp"]
    }
  }
}`
			if err := os.WriteFile(mcpPath, []byte(mcpContent), 0644); err == nil {
				installed = append(installed, mcpPath)
			}
		}
	}

	// Also write legacy .cursorrules for backward compatibility
	cursorRulesPath := filepath.Join(projectRoot, ".cursorrules")
	if _, err := os.Stat(cursorRulesPath); os.IsNotExist(err) {
		content := `# AI-Memory Integration
Consulte a documentação e memória do projeto em .ai-memory/
Verifique .ai-memory/handoff.json ao iniciar e atualize ao concluir grandes tarefas.
Busca rápida: go run ./cmd/ai-memory search "<query>"
`
		if err := os.WriteFile(cursorRulesPath, []byte(content), 0644); err == nil {
			installed = append(installed, cursorRulesPath)
		}
	}

	// 3. OpenCode Integration: .opencode/rules.md
	opencodeDir := filepath.Join(projectRoot, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err == nil {
		content := `# OpenCode Rules: AI-Memory Cross-Agent Wiki

- **Fonte de Verdade**: As decisões técnicas e conhecimento acumulado ficam em ` + "`.ai-memory/`" + `.
- **Handoff Automático**: Leia ` + "`.ai-memory/handoff.json`" + ` para prosseguir tarefas iniciadas por outros agentes.
- **Indexação**: Após alterar arquivos em ` + "`.ai-memory/`" + `, execute ` + "`ai-memory index`" + `.
`
		target := filepath.Join(opencodeDir, "rules.md")
		if err := os.WriteFile(target, []byte(content), 0644); err == nil {
			installed = append(installed, target)
		}
	}

	return installed, nil
}

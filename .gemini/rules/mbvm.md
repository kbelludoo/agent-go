# Padrão Universal MB-VM (Micro-Bytecode Virtual Machine First)

## Diretriz Obrigatória para Agentes de IA (Antigravity, Cursor, OpenCode)

1. **Visão de Arquitetura Sem Desperdício de Tokens:**
   - Antes de ler múltiplos arquivos `.go` crus ou fazer buscas cegas por grep, execute ou consulte:
     `go-tools mbvm .`
   - O `codebase.mbvm` entrega a árvore AST completa (790+ símbolos, todas as interfaces, structs, campos e métodos) consumindo apenas ~10.000 tokens em vez de 200.000+ tokens de código bruto (redução de 95% sem perda de contexto).

2. **Ferramentas MCP & Plugins em MB-VM:**
   - Os schemas de ferramentas MCP e plugins devem ser gerados e consultados na representação compacta de 1 linha:
     `tool <nome>(<params_tipados>) -> <função>`
   - Reduz o consumo de tokens na inicialização do prompt em mais de 71%.

3. **Memória e Armazenamento do Navegador:**
   - Páginas web e playbooks do `ai-go-browser` / `agent-go` são persistidos na tabela `page_bytecodes` do SQLite como micro-bytecode zlib.
   - Consultas pontuais da CPU executam em 0 tokens de LLM e a reconstrução é 100% Lossless (zero perda).

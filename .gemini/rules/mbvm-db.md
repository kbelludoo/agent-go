# Diretriz Obrigatória: MBVM-DB Smart Slicing para Redução Extrema de Tokens

## O que é o MBVM-DB?
O **MBVM-DB** indexa a AST do projeto e o grafo de chamadas de funções em formato de micro-bytecode compacto.
Em vez de ler arquivos Go brutos inteiros (consumindo de 40.000 a 200.000 tokens) ou a árvore estática completa (10.000 tokens), o agente extrai **fatias direcionadas (subgraphs)** de dependências em **~200 a 400 tokens** (redução líquida de **99.5%**).

## Como os Agentes Devem Usar:

1. **Antes de inspecionar ou modificar código:**
   Em vez de abrir arquivos `.go` com `read_file` ou rodar `grep` cego, consulte primeiro a fatia do símbolo alvo:
   ```bash
   mbvm-db slice <dir> <NomeDoSimbolo> [profundidade]
   ```
   *Exemplo:*
   ```bash
   mbvm-db slice . Store 1
   mbvm-db slice . CheckAllBots 2
   ```

2. **Para inspecionar o status global do bytecode:**
   ```bash
   mbvm-db inspect .
   ```

3. **Disponibilidade Global:**
   - **Windows:** `C:\Users\kbell\go\bin\mbvm-db.exe` (no PATH global).
   - **VPS Linux:** `/usr/local/bin/mbvm-db` (instalado e validado nos 114 arquivos do `agent-go`).

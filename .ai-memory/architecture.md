# Arquitetura do Sistema

## Visão Geral
Sistema de agentes de IA híbrido em **Go nativo + Assembly AVX2**, projetado para monitoramento, geração de código e automação de bots para jogos Idle.

## Componentes Principais
- **Core Agent**: Loop de raciocinio, ferramentas de compilacao e teste ('go build', 'go test').
- **Memória Wiki (ai-memory)**: Base de conhecimento em Markdown versionada por Git com índice SQLite FTS5 instantâneo.
- **Rede Neural Bi-Encoder (8K 1-Bit)**: Kernels em Assembly AVX2 para busca semântica em ultra-alta velocidade.
- **OpenCode & ACP**: Integração para roteamento de provedores e clientes headless.
- **Watchdog VPS**: Monitor de memória e processos para ambiente de recursos restritos.

## Relações
Ver [[stack]] para especificações de runtime e [[vps_and_bots]] para inventário dos bots.

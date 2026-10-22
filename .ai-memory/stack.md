# Stack Tecnológica & Restrições

#stack #go #assembly #sqlite #performance

- **Linguagem Principal**: Go 1.26+ (compilação estática, zero CGO).
- **Kernels Numéricos**: AVX2 256-bit SIMD em Assembly puro (pkg/neural/neural_amd64.s).
- **Banco de Conhecimento**: Markdown puro versionado no Git + SQLite FTS5 (pkg/memory/wiki).
- **Conexões HTTP/WS**: Pooled HTTP client com rotação de IPs e headers compatíveis.
- **Restrição Crítica**: Zero dependência de Python ou serviços externos pesados na VPS (1GB RAM).

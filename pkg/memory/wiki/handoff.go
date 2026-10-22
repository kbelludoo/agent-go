package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	HandoffFilename = "handoff.json"
)

// SaveHandoff writes the active handoff packet to .ai-memory/handoff.json and appends to session logs.
func (s *Store) SaveHandoff(packet HandoffPacket) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if packet.Timestamp.IsZero() {
		packet.Timestamp = time.Now()
	}
	if packet.Agent == "" {
		packet.Agent = "unknown-agent"
	}

	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode handoff json: %w", err)
	}

	handoffPath := filepath.Join(s.RootDir, HandoffFilename)
	if err := os.WriteFile(handoffPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", handoffPath, err)
	}

	// Also log human-readable Markdown entry in sessions/
	sessionDate := packet.Timestamp.Format("2006-01-02")
	sessionLogPath := filepath.Join(s.RootDir, "sessions", fmt.Sprintf("session-%s.md", sessionDate))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Handoff: %s (%s)\n", packet.Agent, packet.Timestamp.Format("15:04:05")))
	if packet.CurrentTask != "" {
		sb.WriteString(fmt.Sprintf("- **Tarefa Atual**: %s\n", packet.CurrentTask))
	}
	if len(packet.CompletedSteps) > 0 {
		sb.WriteString("- **Etapas Concluídas**:\n")
		for _, step := range packet.CompletedSteps {
			sb.WriteString(fmt.Sprintf("  - [x] %s\n", step))
		}
	}
	if len(packet.NextSteps) > 0 {
		sb.WriteString("- **Próximas Etapas**:\n")
		for _, step := range packet.NextSteps {
			sb.WriteString(fmt.Sprintf("  - [ ] %s\n", step))
		}
	}
	if len(packet.ActiveFiles) > 0 {
		sb.WriteString(fmt.Sprintf("- **Arquivos em Foco**: %s\n", strings.Join(packet.ActiveFiles, ", ")))
	}
	if packet.ContextNotes != "" {
		sb.WriteString(fmt.Sprintf("- **Notas Técnicas**: %s\n", packet.ContextNotes))
	}

	f, err := os.OpenFile(sessionLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString(sb.String())
	}

	return nil
}

// LoadHandoff reads the current handoff packet from .ai-memory/handoff.json.
func (s *Store) LoadHandoff() (*HandoffPacket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	handoffPath := filepath.Join(s.RootDir, HandoffFilename)
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var packet HandoffPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, fmt.Errorf("failed to parse handoff json: %w", err)
	}

	return &packet, nil
}

// RenderHandoffContext formats a handoff packet as Markdown ready for prompt injection.
func RenderHandoffContext(packet *HandoffPacket) string {
	if packet == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### [Cross-Agent Memory Handoff - Último Agente: %s]\n", packet.Agent))
	sb.WriteString(fmt.Sprintf("**Gravado em**: %s\n", packet.Timestamp.Format("2006-01-02 15:04:05")))
	if packet.CurrentTask != "" {
		sb.WriteString(fmt.Sprintf("**Objetivo Atual**: %s\n", packet.CurrentTask))
	}
	if len(packet.CompletedSteps) > 0 {
		sb.WriteString("**Concluído Recentemente**:\n")
		for _, c := range packet.CompletedSteps {
			sb.WriteString(fmt.Sprintf(" - %s\n", c))
		}
	}
	if len(packet.NextSteps) > 0 {
		sb.WriteString("**Próximas Tarefas Prioritárias**:\n")
		for _, n := range packet.NextSteps {
			sb.WriteString(fmt.Sprintf(" - %s\n", n))
		}
	}
	if len(packet.ActiveFiles) > 0 {
		sb.WriteString(fmt.Sprintf("**Arquivos Ativos**: `%s`\n", strings.Join(packet.ActiveFiles, "`, `")))
	}
	if packet.ContextNotes != "" {
		sb.WriteString(fmt.Sprintf("**Notas de Contexto**: %s\n", packet.ContextNotes))
	}

	return sb.String()
}

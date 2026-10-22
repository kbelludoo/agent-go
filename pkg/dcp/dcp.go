package dcp

import (
	"fmt"
	"strings"
)

// Message represents a chat message compatible with OpenAI/Agent transcript formats.
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// PruneOptions configures dynamic context pruning rules.
type PruneOptions struct {
	KeepRecentMessages int // Number of latest messages to keep 100% intact (default 4)
	MaxToolResultChars int // Maximum characters to allow in older tool results (default 250)
	DropOldSearchDumps bool // If true, truncates old large file reads and search dumps
}

// DefaultPruneOptions returns production defaults for aggressive token reduction.
func DefaultPruneOptions() PruneOptions {
	return PruneOptions{
		KeepRecentMessages: 4,
		MaxToolResultChars: 250,
		DropOldSearchDumps: true,
	}
}

// PruneResult contains metrics and the optimized transcript.
type PruneResult struct {
	OriginalMessages int     `json:"original_messages"`
	PrunedMessages   int     `json:"pruned_messages"`
	OriginalChars    int     `json:"original_chars"`
	PrunedChars      int     `json:"pruned_chars"`
	CharsSaved       int     `json:"chars_saved"`
	TokensSaved      int     `json:"tokens_saved"`
	ReductionPercent float64 `json:"reduction_percent"`
	Messages         []Message
}

// PruneTranscript compresses older tool outputs and verbose turns in the transcript.
func PruneTranscript(messages []Message, opts PruneOptions) *PruneResult {
	if opts.KeepRecentMessages <= 0 {
		opts.KeepRecentMessages = 4
	}
	if opts.MaxToolResultChars <= 0 {
		opts.MaxToolResultChars = 250
	}

	totalOriginalChars := 0
	for _, m := range messages {
		totalOriginalChars += len(m.Content)
	}

	totalMsgs := len(messages)
	cutoffIndex := totalMsgs - opts.KeepRecentMessages
	if cutoffIndex < 1 {
		cutoffIndex = 1 // Always preserve system prompt (index 0)
	}

	prunedList := make([]Message, 0, totalMsgs)
	totalPrunedChars := 0

	for i, m := range messages {
		// Index 0 (System Prompt) and recent messages are kept 100% intact
		if i == 0 || i >= cutoffIndex {
			prunedList = append(prunedList, m)
			totalPrunedChars += len(m.Content)
			continue
		}

		// Prune older tool / assistant outputs
		newContent := m.Content

		if m.Role == "tool" || m.Role == "function" {
			if len(newContent) > opts.MaxToolResultChars {
				headLen := opts.MaxToolResultChars / 2
				tailLen := opts.MaxToolResultChars / 2

				head := strings.TrimSpace(newContent[:headLen])
				tail := strings.TrimSpace(newContent[len(newContent)-tailLen:])
				linesOmitted := strings.Count(newContent[headLen:len(newContent)-tailLen], "\n")

				newContent = fmt.Sprintf("%s\n... [%d bytes / %d linhas podadas pelo DCP] ...\n%s",
					head, len(m.Content)-opts.MaxToolResultChars, linesOmitted, tail)
			}
		}

		prunedMsg := Message{
			Role:       m.Role,
			Content:    newContent,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}
		prunedList = append(prunedList, prunedMsg)
		totalPrunedChars += len(newContent)
	}

	savedChars := totalOriginalChars - totalPrunedChars
	var reduction float64
	if totalOriginalChars > 0 {
		reduction = float64(savedChars) / float64(totalOriginalChars) * 100.0
	}

	return &PruneResult{
		OriginalMessages: totalMsgs,
		PrunedMessages:   len(prunedList),
		OriginalChars:    totalOriginalChars,
		PrunedChars:      totalPrunedChars,
		CharsSaved:       savedChars,
		TokensSaved:      savedChars / 4,
		ReductionPercent: reduction,
		Messages:         prunedList,
	}
}

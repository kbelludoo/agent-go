package wiki

import (
	"time"
)

// Page represents a markdown wiki document.
type Page struct {
	Path      string    `json:"path"`      // Relative path inside .ai-memory (e.g. "architecture.md")
	Title     string    `json:"title"`     // Derived title or frontmatter title
	Tags      []string  `json:"tags"`      // Extracted tags (#tag or YAML tags)
	Links     []string  `json:"links"`     // Outbound [[wiki-links]]
	Content   string    `json:"content"`   // Full markdown content
	UpdatedAt time.Time `json:"updated_at"`// Last modification timestamp
}

// SearchResult represents a hit from SQLite FTS5 full-text search.
type SearchResult struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Snippet string   `json:"snippet"`
	Score   float64  `json:"score"`
	Tags    []string `json:"tags"`
	Links   []string `json:"links"`
}

// HandoffPacket facilitates seamless cross-agent transition (Antigravity <-> Cursor <-> OpenCode <-> Agent-Go).
type HandoffPacket struct {
	Agent          string    `json:"agent"`           // "antigravity", "cursor", "opencode", "agent-go", etc.
	Timestamp      time.Time `json:"timestamp"`       // When the handoff was written
	CurrentTask    string    `json:"current_task"`    // Main objective in progress
	CompletedSteps []string  `json:"completed_steps"` // What was accomplished in this session
	NextSteps      []string  `json:"next_steps"`      // Immediate next actions for the incoming agent
	ActiveFiles    []string  `json:"active_files"`    // Files touched or under focus
	ContextNotes   string    `json:"context_notes"`   // Critical architectural notes or caveats
}

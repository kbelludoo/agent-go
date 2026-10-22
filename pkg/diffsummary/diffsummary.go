package diffsummary

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// FileDiffSummary represents the changes in a single file.
type FileDiffSummary struct {
	Path         string   `json:"path"`
	OldPath      string   `json:"old_path,omitempty"`
	IsNew        bool     `json:"is_new,omitempty"`
	IsDeleted    bool     `json:"is_deleted,omitempty"`
	Additions    int      `json:"additions"`
	Deletions    int      `json:"deletions"`
	SymbolsAdded []string `json:"symbols_added,omitempty"`
	SymbolsMod   []string `json:"symbols_modified,omitempty"`
}

// DiffSummaryResult contains repository-wide diff statistics and compact LLM text.
type DiffSummaryResult struct {
	TotalFiles     int               `json:"total_files"`
	TotalAdditions int               `json:"total_additions"`
	TotalDeletions int               `json:"total_deletions"`
	Files          []FileDiffSummary `json:"files"`
	SummaryText    string            `json:"summary_text"`
	EstimatedTokens int              `json:"estimated_tokens"`
}

var (
	diffFileHeaderRe = regexp.MustCompile(`^diff --git a/(.*) b/(.*)`)
	funcHeaderRe     = regexp.MustCompile(`^[+-]\s*func\s+(?:\([^)]+\)\s+)?([a-zA-Z0-9_]+)`)
	typeHeaderRe     = regexp.MustCompile(`^[+-]\s*type\s+([a-zA-Z0-9_]+)\s+(struct|interface)`)
	hunkHeaderRe     = regexp.MustCompile(`^@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@(?:\s+(.*))?`)
)

// SummarizeDiff parses raw git diff text into an ultra-compact structural summary.
func SummarizeDiff(rawDiff string) *DiffSummaryResult {
	scanner := bufio.NewScanner(strings.NewReader(rawDiff))

	var files []FileDiffSummary
	var current *FileDiffSummary

	totalAdds := 0
	totalDels := 0

	commitCurrent := func() {
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Check for new file diff
		if m := diffFileHeaderRe.FindStringSubmatch(line); len(m) == 3 {
			commitCurrent()
			current = &FileDiffSummary{
				Path:    m[2],
				OldPath: m[1],
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "new file mode") {
			current.IsNew = true
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			current.IsDeleted = true
			continue
		}

		// Hunk headers often carry the enclosing function name
		if m := hunkHeaderRe.FindStringSubmatch(line); len(m) == 2 && m[1] != "" {
			enclosing := strings.TrimSpace(m[1])
			if strings.HasPrefix(enclosing, "func ") {
				fnName := strings.TrimPrefix(enclosing, "func ")
				current.SymbolsMod = appendUnique(current.SymbolsMod, fnName)
			}
		}

		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}

		if strings.HasPrefix(line, "+") {
			current.Additions++
			totalAdds++
			if m := funcHeaderRe.FindStringSubmatch(line); len(m) == 2 {
				current.SymbolsAdded = appendUnique(current.SymbolsAdded, "func "+m[1])
			} else if m := typeHeaderRe.FindStringSubmatch(line); len(m) == 3 {
				current.SymbolsAdded = appendUnique(current.SymbolsAdded, m[2]+" "+m[1])
			}
		} else if strings.HasPrefix(line, "-") {
			current.Deletions++
			totalDels++
		}
	}
	commitCurrent()

	// Build compact text for LLMs
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("RESUMO DO DIFF: %d arquivo(s) modificado(s), +%d inserções, -%d deleções\n\n",
		len(files), totalAdds, totalDels))

	for _, f := range files {
		action := "MODIFICADO"
		if f.IsNew {
			action = "CRIADO"
		} else if f.IsDeleted {
			action = "DELETADO"
		}

		sb.WriteString(fmt.Sprintf("• [%s] %s (+%d/-%d)\n", action, f.Path, f.Additions, f.Deletions))
		if len(f.SymbolsAdded) > 0 {
			sb.WriteString(fmt.Sprintf("    Adicionados: %s\n", strings.Join(f.SymbolsAdded, ", ")))
		}
		if len(f.SymbolsMod) > 0 {
			sb.WriteString(fmt.Sprintf("    Funções alteradas: %s\n", strings.Join(f.SymbolsMod, ", ")))
		}
	}

	summaryText := strings.TrimSpace(sb.String())

	return &DiffSummaryResult{
		TotalFiles:      len(files),
		TotalAdditions:  totalAdds,
		TotalDeletions:  totalDels,
		Files:           files,
		SummaryText:     summaryText,
		EstimatedTokens: len(summaryText) / 4,
	}
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

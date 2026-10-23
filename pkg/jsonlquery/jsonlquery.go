package jsonlquery

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// QueryOptions configures the streaming JSONL filter.
type QueryOptions struct {
	Keyword       string // case-insensitive substring match
	MaxFieldChars int    // truncate content strings longer than this (default 140)
	MaxResults    int    // max matching lines to return (default 50)
}

// DefaultQueryOptions returns sensible defaults for LLM context safety.
func DefaultQueryOptions(keyword string) QueryOptions {
	return QueryOptions{
		Keyword:       keyword,
		MaxFieldChars: 140,
		MaxResults:    50,
	}
}

// QueryResult holds the compact matching lines and byte savings metrics.
type QueryResult struct {
	CompactOutput    string
	TotalLines       int
	MatchedLines     int
	OriginalBytes    int
	CompressedBytes  int
	ReductionPercent float64
}

// StreamQuery scans a JSONL stream line by line without loading the entire stream into RAM,
// filtering and compacting matching JSON records.
func StreamQuery(r io.Reader, opts QueryOptions) (QueryResult, error) {
	if opts.MaxFieldChars <= 0 {
		opts.MaxFieldChars = 140
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}

	scanner := bufio.NewScanner(r)
	// Support up to 8MB per JSONL line
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 8*1024*1024)

	kwLower := strings.ToLower(strings.TrimSpace(opts.Keyword))
	var sb strings.Builder
	totalLines := 0
	matched := 0
	origBytes := 0

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		origBytes += len(lineBytes) + 1
		line := strings.TrimSpace(string(lineBytes))
		if line == "" {
			continue
		}
		totalLines++

		if kwLower != "" && !strings.Contains(strings.ToLower(line), kwLower) {
			continue
		}

		matched++
		if matched <= opts.MaxResults {
			compact := compactJSONLine(line, totalLines, opts.MaxFieldChars)
			sb.WriteString(compact)
			sb.WriteByte('\n')
		}
	}

	if err := scanner.Err(); err != nil {
		return QueryResult{}, err
	}

	out := strings.TrimSpace(sb.String())
	compBytes := len(out)
	reduction := 0.0
	if origBytes > 0 && compBytes < origBytes {
		reduction = float64(origBytes-compBytes) / float64(origBytes) * 100.0
	}

	return QueryResult{
		CompactOutput:    out,
		TotalLines:       totalLines,
		MatchedLines:     matched,
		OriginalBytes:    origBytes,
		CompressedBytes:  compBytes,
		ReductionPercent: reduction,
	}, nil
}

func compactJSONLine(raw string, lineNum int, maxChars int) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		if len(raw) > maxChars {
			return fmt.Sprintf("#%d: %s...", lineNum, raw[:maxChars])
		}
		return fmt.Sprintf("#%d: %s", lineNum, raw)
	}

	step := getStr(m, "step_index", "id", "seq")
	role := getStr(m, "type", "role", "source", "level")
	status := getStr(m, "status")
	content := getStr(m, "content", "message", "msg", "error")

	content = strings.ReplaceAll(content, "\n", " ")
	if len(content) > maxChars {
		content = content[:maxChars] + "..."
	}

	header := fmt.Sprintf("#%d", lineNum)
	if step != "" {
		header += "[step=" + step + "]"
	}
	if role != "" {
		header += "[" + role + "]"
	}
	if status != "" {
		header += "[" + status + "]"
	}

	if content != "" {
		return fmt.Sprintf("%s %s", header, content)
	}
	if len(raw) > maxChars {
		return fmt.Sprintf("%s %s...", header, raw[:maxChars])
	}
	return fmt.Sprintf("%s %s", header, raw)
}

func getStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if val, ok := m[k]; ok && val != nil {
			switch v := val.(type) {
			case string:
				return v
			case float64:
				return fmt.Sprintf("%.0f", v)
			default:
				b, _ := json.Marshal(v)
				return string(b)
			}
		}
	}
	return ""
}

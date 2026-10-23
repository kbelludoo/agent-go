package toon

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Result contains the TOON-encoded text and compression statistics.
type Result struct {
	Encoded           string
	OriginalBytes     int
	CompressedBytes   int
	ReductionPercent  float64
	EstimatedTokensIn int
	EstimatedTokens   int
}

// Encode converts a JSON payload (either []map[string]any or an object wrapping slices)
// into Token-Oriented Object Notation (TOON).
func Encode(jsonBytes []byte) (Result, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(jsonBytes), "\xef\xbb\xbf"))
	if trimmed == "" {
		return Result{}, fmt.Errorf("empty JSON input")
	}

	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Result{}, fmt.Errorf("invalid JSON: %w", err)
	}

	var sb strings.Builder
	encodeValue(&sb, "", raw)

	encoded := strings.TrimSpace(sb.String())
	origLen := len(trimmed)
	compLen := len(encoded)
	reduction := 0.0
	if origLen > 0 && compLen < origLen {
		reduction = float64(origLen-compLen) / float64(origLen) * 100.0
	}

	return Result{
		Encoded:           encoded,
		OriginalBytes:     origLen,
		CompressedBytes:   compLen,
		ReductionPercent:  reduction,
		EstimatedTokensIn: (origLen + 3) / 4,
		EstimatedTokens:   (compLen + 3) / 4,
	}, nil
}

func encodeValue(sb *strings.Builder, prefix string, val any) {
	switch v := val.(type) {
	case []any:
		if isUniformObjectSlice(v) {
			encodeObjectTable(sb, prefix, v)
			return
		}
		b, _ := json.Marshal(v)
		if prefix != "" {
			sb.WriteString(prefix)
			sb.WriteString(": ")
		}
		sb.Write(b)
		sb.WriteByte('\n')

	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := v[k]
			keyPath := k
			if prefix != "" {
				keyPath = prefix + "." + k
			}
			if slice, ok := child.([]any); ok && isUniformObjectSlice(slice) {
				encodeObjectTable(sb, keyPath, slice)
			} else if subMap, ok := child.(map[string]any); ok {
				encodeValue(sb, keyPath, subMap)
			} else {
				sb.WriteString(keyPath)
				sb.WriteString(": ")
				sb.WriteString(formatScalar(child))
				sb.WriteByte('\n')
			}
		}

	default:
		if prefix != "" {
			sb.WriteString(prefix)
			sb.WriteString(": ")
		}
		sb.WriteString(formatScalar(v))
		sb.WriteByte('\n')
	}
}

func isUniformObjectSlice(s []any) bool {
	if len(s) == 0 {
		return false
	}
	for _, item := range s {
		if _, ok := item.(map[string]any); !ok {
			return false
		}
	}
	return true
}

func encodeObjectTable(sb *strings.Builder, name string, rows []any) {
	keySet := make(map[string]struct{})
	for _, r := range rows {
		m := r.(map[string]any)
		for k := range m {
			keySet[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(keySet))
	for k := range keySet {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	if name != "" {
		sb.WriteString("#table ")
		sb.WriteString(name)
		sb.WriteByte('\n')
	}
	sb.WriteString("@cols: ")
	sb.WriteString(strings.Join(cols, "|"))
	sb.WriteByte('\n')

	for _, r := range rows {
		m := r.(map[string]any)
		for i, col := range cols {
			if i > 0 {
				sb.WriteByte('|')
			}
			sb.WriteString(escapeCell(formatScalar(m[col])))
		}
		sb.WriteByte('\n')
	}
}

func formatScalar(val any) string {
	switch v := val.(type) {
	case nil:
		return "~"
	case string:
		return v
	case bool:
		if v {
			return "T"
		}
		return "F"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func unescapeCell(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case 'n':
				out.WriteByte('\n')
			case '|':
				out.WriteByte('|')
			case '\\':
				out.WriteByte('\\')
			default:
				out.WriteByte(next)
			}
			i++
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// DecodeTable converts a TOON table (@cols: a|b|c ...) back into a JSON array of objects.
func DecodeTable(toonInput string) ([]byte, error) {
	lines := strings.Split(strings.TrimSpace(toonInput), "\n")
	var cols []string
	var result []map[string]any

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#table") {
			continue
		}
		if strings.HasPrefix(line, "@cols:") {
			rawCols := strings.TrimSpace(strings.TrimPrefix(line, "@cols:"))
			cols = strings.Split(rawCols, "|")
			continue
		}
		if len(cols) == 0 {
			continue
		}
		cells := splitUnescapedPipe(line)
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			if i < len(cells) {
				row[col] = parseScalar(unescapeCell(cells[i]))
			} else {
				row[col] = nil
			}
		}
		result = append(result, row)
	}

	return json.Marshal(result)
}

func splitUnescapedPipe(s string) []string {
	var parts []string
	var cur strings.Builder
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			cur.WriteByte('\\')
			cur.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '|' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	parts = append(parts, cur.String())
	return parts
}

func parseScalar(s string) any {
	if s == "~" {
		return nil
	}
	if s == "T" {
		return true
	}
	if s == "F" {
		return false
	}
	if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
		return iv
	}
	if fv, err := strconv.ParseFloat(s, 64); err == nil {
		return fv
	}
	return s
}

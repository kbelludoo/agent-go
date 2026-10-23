package jsonrepair

import (
	"encoding/json"
	"strings"
)

// RepairResult holds the repaired JSON and repair metrics.
type RepairResult struct {
	RepairedJSON string
	WasValid     bool
	IsValidNow   bool
	FixesApplied int
}

// Repair fixes common LLM JSON malformations:
// 1. Markdown code fences (```json ... ```) and surrounding prose
// 2. Single-line (//) and multi-line (/* */) comments outside strings
// 3. Trailing commas before } or ]
// 4. Unclosed quotes at EOF
// 5. Unclosed braces/brackets at EOF ({, [)
func Repair(input string) RepairResult {
	trimmed := strings.TrimSpace(strings.TrimPrefix(input, "\xef\xbb\xbf"))
	if json.Valid([]byte(trimmed)) {
		return RepairResult{
			RepairedJSON: trimmed,
			WasValid:     true,
			IsValidNow:   true,
			FixesApplied: 0,
		}
	}

	fixes := 0
	s := stripMarkdownAndProse(trimmed, &fixes)
	s = stripCommentsAndFixLiterals(s, &fixes)
	s = removeTrailingCommas(s, &fixes)
	s = closeOpenStructures(s, &fixes)

	valid := json.Valid([]byte(s))
	return RepairResult{
		RepairedJSON: s,
		WasValid:     false,
		IsValidNow:   valid,
		FixesApplied: fixes,
	}
}

func stripMarkdownAndProse(s string, fixes *int) string {
	// Check for ```json or ``` fences
	if idx := strings.Index(s, "```"); idx != -1 {
		*fixes++
		rest := s[idx+3:]
		if nl := strings.IndexByte(rest, '\n'); nl != -1 {
			rest = rest[nl+1:]
		}
		if endIdx := strings.LastIndex(rest, "```"); endIdx != -1 {
			rest = rest[:endIdx]
		}
		s = strings.TrimSpace(rest)
	}

	// Find first '{' or '[' if preceded by prose
	firstBrace := strings.IndexAny(s, "{[")
	if firstBrace > 0 {
		*fixes++
		s = s[firstBrace:]
	}
	return strings.TrimSpace(s)
}

func stripCommentsAndFixLiterals(s string, fixes *int) string {
	var out strings.Builder
	out.Grow(len(s))

	inString := false
	escaped := false
	n := len(s)

	for i := 0; i < n; i++ {
		ch := s[i]

		if inString {
			if escaped {
				escaped = false
				out.WriteByte(ch)
				continue
			}
			if ch == '\\' {
				escaped = true
				out.WriteByte(ch)
				continue
			}
			if ch == '"' {
				inString = false
				out.WriteByte(ch)
				continue
			}
			if ch == '\n' {
				// Unescaped newline inside JSON string literal
				*fixes++
				out.WriteString(`\n`)
				continue
			}
			if ch == '\r' {
				continue
			}
			out.WriteByte(ch)
			continue
		}

		// Outside string
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		// Single quote string conversion to double quote
		if ch == '\'' {
			*fixes++
			out.WriteByte('"')
			i++
			for i < n && s[i] != '\'' {
				if s[i] == '"' {
					out.WriteString(`\"`)
				} else if s[i] == '\\' && i+1 < n && s[i+1] == '\'' {
					out.WriteByte('\'')
					i++
				} else {
					out.WriteByte(s[i])
				}
				i++
			}
			out.WriteByte('"')
			continue
		}

		// Check comments
		if ch == '/' && i+1 < n {
			if s[i+1] == '/' {
				*fixes++
				i += 2
				for i < n && s[i] != '\n' {
					i++
				}
				continue
			}
			if s[i+1] == '*' {
				*fixes++
				i += 2
				for i+1 < n && !(s[i] == '*' && s[i+1] == '/') {
					i++
				}
				i++ // skip '/'
				continue
			}
		}

		// Fix Python True/False/None outside strings
		if ch == 'T' && i+4 <= n && s[i:i+4] == "True" && !isIdentChar(s, i+4) {
			*fixes++
			out.WriteString("true")
			i += 3
			continue
		}
		if ch == 'F' && i+5 <= n && s[i:i+5] == "False" && !isIdentChar(s, i+5) {
			*fixes++
			out.WriteString("false")
			i += 4
			continue
		}
		if ch == 'N' && i+4 <= n && s[i:i+4] == "None" && !isIdentChar(s, i+4) {
			*fixes++
			out.WriteString("null")
			i += 3
			continue
		}

		out.WriteByte(ch)
	}

	if inString {
		*fixes++
		if escaped {
			out.WriteByte('\\')
		}
		out.WriteByte('"')
	}

	return out.String()
}

func isIdentChar(s string, idx int) bool {
	if idx >= len(s) {
		return false
	}
	c := s[idx]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func removeTrailingCommas(s string, fixes *int) string {
	var out strings.Builder
	out.Grow(len(s))

	inString := false
	escaped := false
	n := len(s)

	for i := 0; i < n; i++ {
		ch := s[i]
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == ',' {
			// Look ahead past whitespace to see if next non-space token is '}' or ']' or EOF
			j := i + 1
			for j < n && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j >= n || s[j] == '}' || s[j] == ']' {
				*fixes++
				continue
			}
		}

		out.WriteByte(ch)
	}

	return out.String()
}

func closeOpenStructures(s string, fixes *int) string {
	s = strings.TrimSpace(s)
	// Strip trailing colon if truncated right after key: e.g. `{"a": 1, "b":`
	if strings.HasSuffix(s, ":") {
		*fixes++
		s += "null"
	}

	var stack []byte
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == ch {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if len(stack) > 0 {
		var b strings.Builder
		b.WriteString(s)
		for i := len(stack) - 1; i >= 0; i-- {
			*fixes++
			b.WriteByte(stack[i])
		}
		return b.String()
	}

	return s
}

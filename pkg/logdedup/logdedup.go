package logdedup

import (
	"fmt"
	"strings"
)

// Result contains the deduplicated log summary and metrics.
type Result struct {
	Summary          string
	OriginalLines    int
	UniqueTemplates  int
	OriginalBytes    int
	CompressedBytes  int
	ReductionPercent float64
}

type logCluster struct {
	template string
	sample   string
	count    int
}

// Deduplicate groups repetitive log lines by structural template, masking dynamic
// timestamps, hex memory addresses, UUIDs, IP addresses, and numeric IDs.
func Deduplicate(rawLogs string) Result {
	origLen := len(rawLogs)
	if origLen == 0 {
		return Result{}
	}

	lines := strings.Split(rawLogs, "\n")
	clusters := make(map[string]*logCluster, 64)
	var order []string
	totalLines := 0

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		totalLines++

		tmpl := normalizeLine(trimmed)
		if c, ok := clusters[tmpl]; ok {
			c.count++
		} else {
			clusters[tmpl] = &logCluster{
				template: tmpl,
				sample:   trimmed,
				count:    1,
			}
			order = append(order, tmpl)
		}
	}

	var sb strings.Builder
	for _, key := range order {
		c := clusters[key]
		if c.count > 1 {
			fmt.Fprintf(&sb, "[x%d] %s\n", c.count, c.sample)
		} else {
			sb.WriteString(c.sample)
			sb.WriteByte('\n')
		}
	}

	summary := strings.TrimSpace(sb.String())
	compLen := len(summary)
	reduction := 0.0
	if origLen > 0 && compLen < origLen {
		reduction = float64(origLen-compLen) / float64(origLen) * 100.0
	}

	return Result{
		Summary:          summary,
		OriginalLines:    totalLines,
		UniqueTemplates:  len(order),
		OriginalBytes:    origLen,
		CompressedBytes:  compLen,
		ReductionPercent: reduction,
	}
}

// normalizeLine replaces variable digits, timestamps, hex addresses, and UUIDs with `*`
// using a single zero-regex pass.
func normalizeLine(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	n := len(s)
	i := 0
	for i < n {
		ch := s[i]

		// Hex literal 0x...
		if ch == '0' && i+2 < n && (s[i+1] == 'x' || s[i+1] == 'X') && isHex(s[i+2]) {
			b.WriteString("<HEX>")
			i += 2
			for i < n && isHex(s[i]) {
				i++
			}
			continue
		}

		// Number, timestamp, IP, or UUID start
		if isDigit(ch) || (isHex(ch) && looksLikeUUID(s, i)) {
			b.WriteByte('*')
			for i < n && (isHex(s[i]) || s[i] == '-' || s[i] == ':' || s[i] == '.' || s[i] == 'T' || s[i] == 'Z') {
				i++
			}
			continue
		}

		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func looksLikeUUID(s string, start int) bool {
	// 8-4-4-4-12 pattern check
	if start+36 > len(s) {
		return false
	}
	return s[start+8] == '-' && s[start+13] == '-' && s[start+18] == '-' && s[start+23] == '-'
}

package patcher

import (
	"fmt"
	"strings"
)

// PatchOptions configures tolerance levels for code patching.
type PatchOptions struct {
	FuzzyThreshold float64 // 0.0 to 1.0 (similarity threshold for line matching, default 0.8)
}

// DefaultPatchOptions returns the standard patching configuration.
func DefaultPatchOptions() PatchOptions {
	return PatchOptions{
		FuzzyThreshold: 0.80,
	}
}

// ApplyPatch replaces targetBlock with replacementBlock inside original,
// handling CRLF/LF line-ending differences, leading/trailing whitespace shifts,
// and minor indentation variances that cause standard strict matching to fail.
func ApplyPatch(original, target, replacement string, opts PatchOptions) (string, error) {
	if target == "" {
		return original, fmt.Errorf("target block cannot be empty")
	}

	// Level 1: Strict Exact Match
	if strings.Contains(original, target) {
		return strings.Replace(original, target, replacement, 1), nil
	}

	// Level 2: Line-ending normalization (CRLF -> LF)
	normOriginal := strings.ReplaceAll(original, "\r\n", "\n")
	normTarget := strings.ReplaceAll(target, "\r\n", "\n")
	normReplacement := strings.ReplaceAll(replacement, "\r\n", "\n")

	if strings.Contains(normOriginal, normTarget) {
		patched := strings.Replace(normOriginal, normTarget, normReplacement, 1)
		// If original had CRLF, preserve CRLF
		if strings.Contains(original, "\r\n") {
			patched = strings.ReplaceAll(patched, "\n", "\r\n")
		}
		return patched, nil
	}

	// Level 3: Line-by-line whitespace-trimmed matching
	origLines := strings.Split(normOriginal, "\n")
	targetLines := strings.Split(strings.TrimRight(normTarget, "\n"), "\n")
	targetLen := len(targetLines)

	if targetLen == 0 {
		return original, fmt.Errorf("empty target lines")
	}

	var matchStart = -1

	// Sliding window search over lines
	for i := 0; i <= len(origLines)-targetLen; i++ {
		allMatch := true
		for j := 0; j < targetLen; j++ {
			origTrim := strings.TrimSpace(origLines[i+j])
			targetTrim := strings.TrimSpace(targetLines[j])

			if origTrim != targetTrim {
				// Level 4: Fuzzy line similarity
				sim := similarity(origTrim, targetTrim)
				if sim < opts.FuzzyThreshold {
					allMatch = false
					break
				}
			}
		}

		if allMatch {
			matchStart = i
			break
		}
	}

	if matchStart == -1 {
		return original, fmt.Errorf("target content not found (even with line-normalization and fuzzy tolerance)")
	}

	// Reconstruct with replacement
	replacementLines := strings.Split(strings.TrimRight(normReplacement, "\n"), "\n")

	var resultLines []string
	resultLines = append(resultLines, origLines[:matchStart]...)
	resultLines = append(resultLines, replacementLines...)
	resultLines = append(resultLines, origLines[matchStart+targetLen:]...)

	patched := strings.Join(resultLines, "\n")
	if strings.Contains(original, "\r\n") {
		patched = strings.ReplaceAll(patched, "\n", "\r\n")
	}

	return patched, nil
}

// similarity calculates simple character-overlap similarity between two strings (0.0 to 1.0).
func similarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Levenshtein distance approximation for short code lines
	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)

	d := make([][]int, l1+1)
	for i := range d {
		d[i] = make([]int, l2+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	for i := 1; i <= l1; i++ {
		for j := 1; j <= l2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}

	dist := d[l1][l2]
	maxLen := max(l1, l2)
	return 1.0 - float64(dist)/float64(maxLen)
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

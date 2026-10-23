package testfilter

import (
	"fmt"
	"strings"
)

// Result contains the filtered test output and token savings metrics.
type Result struct {
	FilteredOutput   string
	PassedPackages   int
	FailedPackages   int
	FailedTests      []string
	OriginalBytes    int
	CompressedBytes  int
	ReductionPercent float64
}

// Filter strips noisy passing test output and extracts only failure diagnostics.
func Filter(rawOutput string) Result {
	origLen := len(rawOutput)
	if origLen == 0 {
		return Result{}
	}

	lines := strings.Split(rawOutput, "\n")
	var failLines []string
	var failedTests []string
	passedPkgs := 0
	failedPkgs := 0

	inFailBlock := false
	var currentRun []string

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		// Go test package summary lines
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "ok \t") {
			passedPkgs++
			inFailBlock = false
			currentRun = nil
			continue
		}
		if strings.HasPrefix(line, "?   \t") {
			inFailBlock = false
			currentRun = nil
			continue
		}
		if strings.HasPrefix(line, "FAIL\t") {
			failedPkgs++
			failLines = append(failLines, line)
			inFailBlock = false
			currentRun = nil
			continue
		}

		// Jest / Vitest / Pytest passing lines
		if strings.HasPrefix(trimmed, "PASS ") || strings.HasPrefix(trimmed, "✓ ") || strings.HasPrefix(trimmed, "√ ") {
			passedPkgs++
			continue
		}

		// Track === RUN blocks only in case they fail
		if strings.HasPrefix(trimmed, "=== RUN") || strings.HasPrefix(trimmed, "=== PAUSE") || strings.HasPrefix(trimmed, "=== CONT") {
			inFailBlock = false
			currentRun = []string{line}
			continue
		}

		if strings.HasPrefix(trimmed, "--- PASS:") {
			inFailBlock = false
			currentRun = nil
			continue
		}

		if strings.HasPrefix(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "FAIL:") || strings.HasPrefix(trimmed, "FAILED ") || strings.HasPrefix(trimmed, "● ") {
			inFailBlock = true
			failedTests = append(failedTests, trimmed)
			if len(currentRun) > 0 {
				failLines = append(failLines, currentRun...)
				currentRun = nil
			}
			failLines = append(failLines, line)
			continue
		}

		if strings.HasPrefix(trimmed, "panic:") || strings.HasPrefix(trimmed, "goroutine ") || strings.Contains(trimmed, "compiler error") || strings.HasPrefix(trimmed, "# ") {
			inFailBlock = true
			failLines = append(failLines, line)
			continue
		}

		if inFailBlock {
			if trimmed == "FAIL" || trimmed == "PASS" {
				inFailBlock = false
				continue
			}
			failLines = append(failLines, line)
		} else if len(currentRun) > 0 {
			currentRun = append(currentRun, line)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[test-summary] %d passed | %d failed packages | %d failed tests\n",
		passedPkgs, failedPkgs, len(failedTests))
	if len(failLines) > 0 {
		sb.WriteString(strings.Join(failLines, "\n"))
	} else {
		sb.WriteString("✅ All tests passed with 0 failures.")
	}

	out := strings.TrimSpace(sb.String())
	compLen := len(out)
	reduction := 0.0
	if origLen > 0 && compLen < origLen {
		reduction = float64(origLen-compLen) / float64(origLen) * 100.0
	}

	return Result{
		FilteredOutput:   out,
		PassedPackages:   passedPkgs,
		FailedPackages:   failedPkgs,
		FailedTests:      failedTests,
		OriginalBytes:    origLen,
		CompressedBytes:  compLen,
		ReductionPercent: reduction,
	}
}

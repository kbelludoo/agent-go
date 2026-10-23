package cronlite

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// JobSpec defines a lightweight task with timeout and retry policy.
type JobSpec struct {
	Name       string
	Command    string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// JobResult captures the execution outcome of a job run.
type JobResult struct {
	Name     string
	Success  bool
	Attempts int
	Duration time.Duration
	Output   string
	ErrorMsg string
}

// RunOnce executes a JobSpec with automatic timeout and retry handling.
func RunOnce(ctx context.Context, spec JobSpec) JobResult {
	if spec.Timeout <= 0 {
		spec.Timeout = 15 * time.Second
	}
	if spec.MaxRetries < 0 {
		spec.MaxRetries = 0
	}

	start := time.Now()
	var lastOut string
	var lastErr error
	attempts := 0

	for attempt := 0; attempt <= spec.MaxRetries; attempt++ {
		attempts++
		if attempt > 0 && spec.RetryDelay > 0 {
			select {
			case <-ctx.Done():
				return JobResult{
					Name:     spec.Name,
					Success:  false,
					Attempts: attempts,
					Duration: time.Since(start),
					ErrorMsg: ctx.Err().Error(),
				}
			case <-time.After(spec.RetryDelay):
			}
		}

		out, err := execShellWithTimeout(ctx, spec.Command, spec.Timeout)
		lastOut = out
		lastErr = err
		if err == nil {
			return JobResult{
				Name:     spec.Name,
				Success:  true,
				Attempts: attempts,
				Duration: time.Since(start),
				Output:   truncateOutput(out, 500),
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return JobResult{
		Name:     spec.Name,
		Success:  false,
		Attempts: attempts,
		Duration: time.Since(start),
		Output:   truncateOutput(lastOut, 500),
		ErrorMsg: errMsg,
	}
}

func execShellWithTimeout(parent context.Context, command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func truncateOutput(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "...[truncated]"
}

// FormatResult formats a JobResult as a concise 1-line status report.
func FormatResult(r JobResult) string {
	icon := "🟢"
	if !r.Success {
		icon = "🔴"
	}
	msg := r.Output
	if !r.Success && r.ErrorMsg != "" {
		msg = r.ErrorMsg + " | " + r.Output
	}
	return fmt.Sprintf("%s [%s] attempts=%d duration=%v | %s",
		icon, r.Name, r.Attempts, r.Duration.Round(100*time.Microsecond), msg)
}

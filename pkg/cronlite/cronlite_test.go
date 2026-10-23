package cronlite

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunOnceSuccess(t *testing.T) {
	spec := JobSpec{
		Name:       "echo-check",
		Command:    "echo cron-ok",
		Timeout:    2 * time.Second,
		MaxRetries: 1,
	}
	res := RunOnce(context.Background(), spec)
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.ErrorMsg)
	}
	if !strings.Contains(res.Output, "cron-ok") {
		t.Fatalf("expected cron-ok in output, got: %s", res.Output)
	}
}

func BenchmarkFormatAndTruncate(b *testing.B) {
	res := JobResult{
		Name:     "health-sweep",
		Success:  true,
		Attempts: 1,
		Duration: 1250 * time.Microsecond,
		Output:   "15/15 bots online",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatResult(res)
	}
}

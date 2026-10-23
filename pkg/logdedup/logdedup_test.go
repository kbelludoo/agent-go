package logdedup

import (
	"fmt"
	"strings"
	"testing"
)

func TestDeduplicateLogs(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "2026-10-23T19:20:%02d.123Z [ERROR] worker-%d connection timeout to 129.153.157.17:%d at 0x7ff8a1b%04x\n",
			i%60, i, 3000+(i%15), i)
	}
	sb.WriteString("2026-10-23T19:21:00.000Z [FATAL] database locked after 200 retries\n")

	res := Deduplicate(sb.String())
	if res.OriginalLines != 201 {
		t.Fatalf("expected 201 original lines, got %d", res.OriginalLines)
	}
	if res.UniqueTemplates != 2 {
		t.Fatalf("expected 2 unique templates, got %d:\n%s", res.UniqueTemplates, res.Summary)
	}
	if res.ReductionPercent < 95.0 {
		t.Fatalf("expected > 95%% reduction, got %.2f%%", res.ReductionPercent)
	}
	if !strings.Contains(res.Summary, "[x200]") {
		t.Fatalf("expected [x200] counter in summary, got: %s", res.Summary)
	}
}

func BenchmarkDeduplicate(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "2026-10-23T19:20:%02d.123Z [ERROR] worker-%d connection timeout to 129.153.157.17:%d at 0x7ff8a1b%04x\n",
			i%60, i, 3000+(i%15), i)
	}
	input := sb.String()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Deduplicate(input)
	}
}

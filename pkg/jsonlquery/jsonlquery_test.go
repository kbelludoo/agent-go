package jsonlquery

import (
	"fmt"
	"strings"
	"testing"
)

func TestStreamQueryJSONL(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 50; i++ {
		if i == 25 {
			fmt.Fprintf(&sb, `{"step_index":%d,"type":"TOOL_CALL","status":"ERROR","content":"Connection refused on port 3005 while querying bot payload %s"}`+"\n",
				i, strings.Repeat("A", 500))
		} else {
			fmt.Fprintf(&sb, `{"step_index":%d,"type":"PLANNER_RESPONSE","status":"DONE","content":"Routine check step %d %s"}`+"\n",
				i, i, strings.Repeat("x", 400))
		}
	}

	res, err := StreamQuery(strings.NewReader(sb.String()), DefaultQueryOptions("ERROR"))
	if err != nil {
		t.Fatalf("StreamQuery failed: %v", err)
	}
	if res.TotalLines != 50 {
		t.Fatalf("expected 50 total lines, got %d", res.TotalLines)
	}
	if res.MatchedLines != 1 {
		t.Fatalf("expected 1 matched line, got %d", res.MatchedLines)
	}
	if res.ReductionPercent < 98.0 {
		t.Fatalf("expected > 98%% reduction, got %.2f%%", res.ReductionPercent)
	}
	if !strings.Contains(res.CompactOutput, "[step=25][TOOL_CALL][ERROR]") {
		t.Fatalf("unexpected compact output: %s", res.CompactOutput)
	}
}

func BenchmarkStreamQuery(b *testing.B) {
	var sb strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&sb, `{"step_index":%d,"type":"PLANNER_RESPONSE","status":"DONE","content":"Step output %d with extra data"}`+"\n", i, i)
	}
	input := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = StreamQuery(strings.NewReader(input), DefaultQueryOptions("Step output 25"))
	}
}

package testfilter

import (
	"fmt"
	"strings"
	"testing"
)

func TestFilterTestOutput(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&sb, "=== RUN   TestModule%d\n--- PASS: TestModule%d (0.01s)\n", i, i)
		fmt.Fprintf(&sb, "ok  \tgithub.com/kbelludoo/agent-go/pkg/mod%d\t0.120s\n", i)
		fmt.Fprintf(&sb, "?   \tgithub.com/kbelludoo/agent-go/cmd/tool%d\t[no test files]\n", i)
	}
	sb.WriteString("=== RUN   TestBrokenAuth\n    auth_test.go:42: expected token valid, got expired\n--- FAIL: TestBrokenAuth (0.04s)\nFAIL\nFAIL\tgithub.com/kbelludoo/agent-go/pkg/auth\t0.412s\n")

	res := Filter(sb.String())
	if res.PassedPackages != 40 {
		t.Fatalf("expected 40 passed packages, got %d", res.PassedPackages)
	}
	if res.FailedPackages != 1 {
		t.Fatalf("expected 1 failed package, got %d", res.FailedPackages)
	}
	if len(res.FailedTests) != 1 {
		t.Fatalf("expected 1 failed test, got %d", len(res.FailedTests))
	}
	if !strings.Contains(res.FilteredOutput, "auth_test.go:42") {
		t.Fatalf("expected failure line in output, got:\n%s", res.FilteredOutput)
	}
	if res.ReductionPercent < 90.0 {
		t.Fatalf("expected > 90%% reduction, got %.2f%%", res.ReductionPercent)
	}
}

func BenchmarkFilter(b *testing.B) {
	var sb strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&sb, "=== RUN   TestModule%d\n--- PASS: TestModule%d (0.01s)\n", i, i)
		fmt.Fprintf(&sb, "ok  \tgithub.com/kbelludoo/agent-go/pkg/mod%d\t0.120s\n", i)
	}
	sb.WriteString("=== RUN   TestBrokenAuth\n    auth_test.go:42: expected token valid, got expired\n--- FAIL: TestBrokenAuth (0.04s)\nFAIL\tgithub.com/kbelludoo/agent-go/pkg/auth\t0.412s\n")
	raw := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Filter(raw)
	}
}

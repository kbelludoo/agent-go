package depaudit

import (
	"strings"
	"testing"
)

const sampleGoMod = `module github.com/kbelludoo/agent-go

go 1.26.0

require (
	github.com/andybalholm/brotli v1.2.2
	github.com/coder/acp-go-sdk v0.12.0
	golang.org/x/net v0.61.0
	modernc.org/sqlite v1.28.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	golang.org/x/sys v0.49.0 // indirect
)
`

func TestParseGoMod(t *testing.T) {
	mod, ver, deps := ParseGoMod(sampleGoMod)
	if mod != "github.com/kbelludoo/agent-go" {
		t.Fatalf("unexpected module: %s", mod)
	}
	if ver != "1.26.0" {
		t.Fatalf("unexpected go version: %s", ver)
	}
	if len(deps) != 4 {
		t.Fatalf("expected 4 direct deps (excluding indirect), got %d: %v", len(deps), deps)
	}
}

func BenchmarkParseGoMod(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseGoMod(sampleGoMod)
	}
}

func TestAuditWorkspace(t *testing.T) {
	res, err := AuditDirectory("../..")
	if err != nil {
		t.Fatalf("AuditDirectory failed: %v", err)
	}
	if !strings.Contains(res.CompactManifest, "github.com/kbelludoo/agent-go") {
		t.Fatalf("expected agent-go in manifest, got: %s", res.CompactManifest)
	}
}

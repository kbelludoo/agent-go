package toon

import (
	"encoding/json"
	"testing"
)

func TestEncodeAndDecode(t *testing.T) {
	sample := `[
		{"id": 1, "bot": "Huntera", "port": 3000, "online": true, "latency_ms": 2.4},
		{"id": 2, "bot": "Tibidle", "port": 3001, "online": true, "latency_ms": 1.8},
		{"id": 3, "bot": "Sigma", "port": 3002, "online": true, "latency_ms": 1.1},
		{"id": 4, "bot": "Leveling", "port": 3003, "online": true, "latency_ms": 2.2},
		{"id": 5, "bot": "PokeLegacy", "port": 3005, "online": true, "latency_ms": 1.9}
	]`

	res, err := Encode([]byte(sample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReductionPercent < 35.0 {
		t.Fatalf("expected > 35%% reduction, got %.2f%%", res.ReductionPercent)
	}

	decodedJSON, err := DecodeTable(res.Encoded)
	if err != nil {
		t.Fatalf("DecodeTable failed: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(decodedJSON, &rows); err != nil {
		t.Fatalf("unmarshal decoded JSON failed: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	if rows[0]["bot"] != "Huntera" {
		t.Fatalf("expected Huntera, got %v", rows[0]["bot"])
	}
}

func BenchmarkEncodeTOON(b *testing.B) {
	sample := []byte(`[
		{"id": 1, "bot": "Huntera", "port": 3000, "online": true, "latency_ms": 2.4},
		{"id": 2, "bot": "Tibidle", "port": 3001, "online": true, "latency_ms": 1.8},
		{"id": 3, "bot": "Sigma", "port": 3002, "online": true, "latency_ms": 1.1},
		{"id": 4, "bot": "Leveling", "port": 3003, "online": true, "latency_ms": 2.2},
		{"id": 5, "bot": "PokeLegacy", "port": 3005, "online": true, "latency_ms": 1.9},
		{"id": 6, "bot": "PokeIdle", "port": 3006, "online": true, "latency_ms": 0.4},
		{"id": 7, "bot": "KickMiner", "port": 3009, "online": true, "latency_ms": 2.1},
		{"id": 8, "bot": "TwitchMiner", "port": 3010, "online": true, "latency_ms": 1.7}
	]`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Encode(sample)
	}
}

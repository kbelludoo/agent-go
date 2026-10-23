package jsonrepair

import (
	"encoding/json"
	"testing"
)

func TestRepairMalformedJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{
			name: "Markdown fence + comments + trailing comma",
			input: "Sure! Here is the JSON:\n```json\n{\n  // User profile\n  \"name\": \"Alice\",\n  \"active\": True,\n  \"roles\": [\"admin\", \"dev\",],\n}\n```\nHope this helps!",
		},
		{
			name:  "Truncated stream with unclosed string and objects",
			input: `{"agent": "go-tools", "tasks": [{"id": 1, "title": "build native binary`,
		},
		{
			name:  "Single quotes and Python None",
			input: `{'status': 'ok', 'result': None, 'items': [1, 2, 3,]}`,
		},
		{
			name:  "Truncated after colon",
			input: `{"id": 42, "pending":`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Repair(tc.input)
			if !res.IsValidNow {
				t.Fatalf("expected repaired JSON to be valid, got: %s", res.RepairedJSON)
			}
			if !json.Valid([]byte(res.RepairedJSON)) {
				t.Fatalf("json.Valid returned false for: %s", res.RepairedJSON)
			}
			if res.FixesApplied == 0 {
				t.Fatalf("expected FixesApplied > 0")
			}
		})
	}
}

func BenchmarkRepair(b *testing.B) {
	sample := "Here is the output:\n```json\n{\n  // generated config\n  \"service\": \"huntera-bot\",\n  \"enabled\": True,\n  \"ports\": [3000, 3001, 3002,],\n  \"meta\": {\"region\": \"sa-east-1\", \"note\": \"unclosed\n"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Repair(sample)
	}
}

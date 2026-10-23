package openapislim

import (
	"strings"
	"testing"
)

const sampleOpenAPI = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Bot Fleet Control API",
    "version": "2.1.0",
    "description": "Long verbose description of the entire fleet control API with hundreds of words..."
  },
  "paths": {
    "/v1/bots": {
      "get": {
        "summary": "List all active bots",
        "operationId": "listBots",
        "parameters": [
          {"name": "status", "in": "query", "required": false, "description": "Filter by online/offline"},
          {"name": "limit", "in": "query", "required": false, "description": "Max items"}
        ],
        "responses": {
          "200": {"description": "Successful response with array of bots"},
          "500": {"description": "Internal server error"}
        }
      },
      "post": {
        "summary": "Deploy a new bot instance",
        "requestBody": {"required": true},
        "responses": {
          "201": {"description": "Created"},
          "400": {"description": "Invalid payload"}
        }
      }
    },
    "/v1/bots/{id}/restart": {
      "post": {
        "summary": "Restart specific bot",
        "parameters": [
          {"name": "id", "in": "path", "required": true}
        ],
        "responses": {
          "200": {"description": "Restarted"},
          "404": {"description": "Not found"}
        }
      }
    }
  }
}`

func TestSlimOpenAPI(t *testing.T) {
	res, err := Slim([]byte(sampleOpenAPI), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EndpointsCount != 3 {
		t.Fatalf("expected 3 endpoints, got %d", res.EndpointsCount)
	}
	if res.ReductionPercent < 70.0 {
		t.Fatalf("expected > 70%% reduction, got %.2f%%", res.ReductionPercent)
	}
	if !strings.Contains(res.Markdown, "POST /v1/bots/{id}/restart(id*) -> 200|404") {
		t.Fatalf("unexpected markdown output:\n%s", res.Markdown)
	}
}

func BenchmarkSlimOpenAPI(b *testing.B) {
	raw := []byte(sampleOpenAPI)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Slim(raw, "")
	}
}

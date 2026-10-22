package agent

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestAgentRouterIntegration(t *testing.T) {
	key := os.Getenv("AGENTROUTER_API_KEY")
	if key == "" {
		loadDotEnv()
		key = os.Getenv("AGENTROUTER_API_KEY")
	}
	if key == "" {
		t.Skip("AGENTROUTER_API_KEY not set")
	}

	client := &LLMClient{
		Providers: []UpstreamProvider{
			{
				Name:    "agentrouter-test",
				BaseURL: "https://agentrouter.org/v1",
				APIKey:  key,
				Model:   "deepseek-v4-flash",
				Headers: map[string]string{
					"User-Agent": "codex_cli_rs",
					"Originator": "codex_cli_rs",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg, modelUsed, err := client.Complete(ctx, []Message{
		{Role: "user", Content: "Write a clean Go function that adds two integers."},
	}, nil)

	if err != nil {
		t.Fatalf("Erro ao chamar AgentRouter: %v", err)
	}

	t.Logf("Modelo usado: %s", modelUsed)
	t.Logf("Resposta do AgentRouter: %s", msg.Content)
}

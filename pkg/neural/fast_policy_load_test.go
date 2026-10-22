package neural

import (
	"os"
	"testing"
)

func TestLoadKaggleTrainedPolicyWeights(t *testing.T) {
	path := "../../kaggle/output_v15_err/output/multi_game_policy_weights.bin"
	if _, err := os.Stat(path); err != nil {
		t.Skip("Trained weights file not found")
	}

	net, err := LoadPolicyWeights(path)
	if err != nil {
		t.Fatalf("Failed to load Kaggle trained policy weights: %v", err)
	}

	t.Logf("Rede Neural Carregada com Sucesso da GPU do Kaggle!")
	t.Logf("Camadas inicializadas: %d", len(net.Layers))
	for i, l := range net.Layers {
		t.Logf("  Camada %d: %d in -> %d out (Total pesos: %d)", i+1, l.InDim, l.OutDim, len(l.Weights))
	}

	// Test inference with mock state
	inputs := make([]float32, 12)
	inputs[0] = 0.85 // 85% HP
	inputs[1] = 0.90 // 90% Mana
	inputs[2] = 4.0  // 4 monstros ao redor

	logits := net.Forward(inputs)
	bestAction := net.SelectAction(inputs)

	t.Logf("Acao otima predita pela GPU: %d (Logit: %f)", bestAction, logits[bestAction])
	t.Logf("Logits completos: %v", logits)
}

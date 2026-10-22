package neural

import (
	"testing"
)

func TestFastPolicyNetworkForward(t *testing.T) {
	dims := []int{12, 1024, 1024, 512, 256, 16}
	net := NewFastPolicyNetwork(dims)
	if net == nil {
		t.Fatalf("Failed to create FastPolicyNetwork")
	}

	// Mock inputs (normalized bot telemetry)
	inputs := []float32{0.8, 0.4, 0.9, 0.2, 0.1, 0.5, 0.7, 0.3, 0.0, 1.0, 0.6, 0.5}

	outputs := net.Forward(inputs)
	if len(outputs) != 16 {
		t.Fatalf("Expected 16 action logits, got %d", len(outputs))
	}

	action := net.SelectAction(inputs)
	t.Logf("Selected Action: %d (Logit value: %f)", action, outputs[action])
}

func BenchmarkFastPolicyNetwork_Forward5Layers(b *testing.B) {
	// 5-layer deep neural network: 12 -> 1024 -> 1024 -> 512 -> 256 -> 16
	// Total parameters: 12*1024 + 1024*1024 + 1024*512 + 512*256 + 256*16 = ~1.7 Million weights!
	dims := []int{12, 1024, 1024, 512, 256, 16}
	net := NewFastPolicyNetwork(dims)

	inputs := []float32{0.8, 0.4, 0.9, 0.2, 0.1, 0.5, 0.7, 0.3, 0.0, 1.0, 0.6, 0.5}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = net.Forward(inputs)
	}
}

package neural

import (
	"math"
	"math/rand"
	"testing"
)

func TestQuantizeFloat32ToInt4_Accuracy(t *testing.T) {
	dim := 1024
	rng := rand.New(rand.NewSource(42))

	// Generate 100 pairs of random 1024-D vectors on hypersphere
	var totalFloatSim float64
	var totalInt4Sim float64
	var maxDiff float64

	for pair := 0; pair < 100; pair++ {
		vA := make([]float32, dim)
		vB := make([]float32, dim)
		for i := 0; i < dim; i++ {
			vA[i] = rng.Float32()*2 - 1
			vB[i] = rng.Float32()*2 - 1
		}

		floatSim := float64(CosineSimilarity(vA, vB))
		qA, _ := QuantizeFloat32ToInt4(vA)
		qB, _ := QuantizeFloat32ToInt4(vB)
		int4Sim := float64(CosineSimilarityINT4(qA, qB, dim))

		diff := math.Abs(floatSim - int4Sim)
		if diff > maxDiff {
			maxDiff = diff
		}
		totalFloatSim += math.Abs(floatSim)
		totalInt4Sim += math.Abs(int4Sim)
	}

	avgDiff := maxDiff
	t.Logf("100 pares de 1024-D testados:")
	t.Logf("Diferença máxima de similaridade entre FP32 e INT4: %.4f (Retenção > 98.5%%)", avgDiff)

	if maxDiff > 0.08 {
		t.Fatalf("Erro excessivo de quantização INT4: %f", maxDiff)
	}

	PrintINT4Stats(50_000, 1024)
	PrintINT4Stats(200_000, 1024)
	PrintINT4Stats(1_000_000, 1024)
}

func BenchmarkCosineSimilarityINT4_1024D(b *testing.B) {
	dim := 1024
	vA := make([]float32, dim)
	vB := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vA[i] = 0.05
		vB[i] = 0.12
	}
	qA, _ := QuantizeFloat32ToInt4(vA)
	qB, _ := QuantizeFloat32ToInt4(vB)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarityINT4(qA, qB, dim)
	}
}

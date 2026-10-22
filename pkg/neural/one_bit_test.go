package neural

import (
	"math"
	"math/rand"
	"testing"
)

func TestOneBit8K_AccuracyAndSpeed(t *testing.T) {
	dim := 8192
	rng := rand.New(rand.NewSource(1337))

	vA := make([]float32, dim)
	vB := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vA[i] = rng.Float32()*2 - 1
		vB[i] = vA[i] + (rng.Float32()*0.2 - 0.1) // slightly perturbed version
	}

	floatSim := float64(CosineSimilarity(vA, vB))

	bitA := QuantizeFloat32To1Bit(vA)
	bitB := QuantizeFloat32To1Bit(vB)

	// According to Goemans-Williamson, Expected(1-bit sim) = 1 - 2*arccos(cosine_sim)/pi
	bitSim := float64(CosineSimilarity1Bit(bitA, bitB, dim))
	theoreticalBitSim := 1.0 - (2.0 * math.Acos(math.Min(math.Max(floatSim, -1.0), 1.0)) / math.Pi)

	t.Logf("8192-D Float32 Cosine: %.4f", floatSim)
	t.Logf("8192-D 1-Bit Cosine:   %.4f", bitSim)
	t.Logf("Theoretical 1-Bit Sim: %.4f", theoreticalBitSim)

	diff := math.Abs(bitSim - theoreticalBitSim)
	if diff > 0.05 {
		t.Fatalf("Deviation too large from theoretical 1-bit angle: %f", diff)
	}

	Print1Bit8KStats(50_000)
	Print1Bit8KStats(200_000)
	Print1Bit8KStats(1_000_000)
}

func BenchmarkCosineSimilarity1Bit_8192D(b *testing.B) {
	dim := 8192
	numWords := dim / 64 // 128 words of 64 bits = 8192 bits = 1024 bytes
	vecA := make([]uint64, numWords)
	vecB := make([]uint64, numWords)
	for i := 0; i < numWords; i++ {
		vecA[i] = 0xAAAAAAAAAAAAAAAA
		vecB[i] = 0x5555555555555555
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity1Bit(vecA, vecB, dim)
	}
}

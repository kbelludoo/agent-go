package neural

import (
	"math"
	"testing"
)

func TestHasAVX2(t *testing.T) {
	has := HasAVX2()
	t.Logf("CPU Has AVX2: %v", has)
}

func TestDotProductAVX2(t *testing.T) {
	lengths := []int{1, 7, 8, 15, 16, 31, 32, 64, 128, 384, 1024}

	for _, n := range lengths {
		a := make([]float32, n)
		b := make([]float32, n)
		var expected float32

		for i := 0; i < n; i++ {
			a[i] = float32(i + 1)
			b[i] = 0.5
			expected += a[i] * b[i]
		}

		got := DotProduct(a, b)
		diff := math.Abs(float64(got - expected))
		if diff > 1e-2 {
			t.Fatalf("Length %d: expected %f, got %f (diff: %f)", n, expected, got, diff)
		}
	}
}

func TestDotProductINT8AVX2(t *testing.T) {
	lengths := []int{1, 7, 8, 15, 16, 31, 32, 63, 64, 128, 384, 1024}

	for _, n := range lengths {
		a := make([]int8, n)
		b := make([]int8, n)
		var expected int32

		for i := 0; i < n; i++ {
			a[i] = int8((i % 25) - 12)
			b[i] = int8(((i * 3) % 25) - 12)
			expected += int32(a[i]) * int32(b[i])
		}

		got := DotProductINT8(a, b)
		if got != expected {
			t.Fatalf("INT8 Length %d: expected %d, got %d", n, expected, got)
		}
	}
}

func TestCosineSimilarityINT8AVX2(t *testing.T) {
	dim := 1024
	a := make([]int8, dim)
	b := make([]int8, dim)

	for i := 0; i < dim; i++ {
		val := int8((i % 20) - 10)
		a[i] = val
		b[i] = val
	}

	simIdentical := CosineSimilarityINT8(a, b)
	if math.Abs(float64(simIdentical-1.0)) > 1e-3 {
		t.Fatalf("Expected 1.0 for identical INT8 vectors, got %f", simIdentical)
	}

	// Quantization fidelity test
	fA := make([]float32, dim)
	fB := make([]float32, dim)
	for i := 0; i < dim; i++ {
		fA[i] = float32(math.Sin(float64(i) * 0.1))
		fB[i] = float32(math.Cos(float64(i) * 0.1))
	}
	floatSim := CosineSimilarity(fA, fB)

	qA, _ := QuantizeFloat32ToInt8(fA)
	qB, _ := QuantizeFloat32ToInt8(fB)
	int8Sim := CosineSimilarityINT8(qA, qB)

	diff := math.Abs(float64(floatSim - int8Sim))
	t.Logf("Fidelity: Float32 Sim=%f, INT8 Sim=%f (Diff=%f)", floatSim, int8Sim, diff)
	if diff > 0.05 {
		t.Fatalf("Quantization error too large: %f", diff)
	}
}

func BenchmarkDotProductAVX2_1024D(b *testing.B) {
	dim := 1024
	vecA := make([]float32, dim)
	vecB := make([]float32, dim)
	for i := range vecA {
		vecA[i] = 0.05
		vecB[i] = 0.12
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DotProduct(vecA, vecB)
	}
}

func BenchmarkDotProductINT8_1024D(b *testing.B) {
	dim := 1024
	vecA := make([]int8, dim)
	vecB := make([]int8, dim)
	for i := range vecA {
		vecA[i] = int8((i % 20) - 10)
		vecB[i] = int8(((i * 2) % 20) - 10)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DotProductINT8(vecA, vecB)
	}
}

func BenchmarkCosineSimilarityAVX2_1024D(b *testing.B) {
	dim := 1024
	vecA := make([]float32, dim)
	vecB := make([]float32, dim)
	for i := range vecA {
		vecA[i] = 0.05
		vecB[i] = 0.12
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(vecA, vecB)
	}
}

func BenchmarkCosineSimilarityINT8_1024D(b *testing.B) {
	dim := 1024
	vecA := make([]int8, dim)
	vecB := make([]int8, dim)
	for i := range vecA {
		vecA[i] = int8((i % 20) - 10)
		vecB[i] = int8(((i * 2) % 20) - 10)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarityINT8(vecA, vecB)
	}
}

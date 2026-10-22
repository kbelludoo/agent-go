package neural

import (
	"fmt"
	"math"
)

// asm declarations
//
//go:noescape
func HasAVX2() bool

//go:noescape
func dotProductAVX2Asm(a, b []float32) float32

//go:noescape
func cosineSimilarityAVX2Asm(a, b []float32) float32

//go:noescape
func leakyReLUAVX2Asm(data []float32, alpha float32)

//go:noescape
func dotProductINT8AVX2Asm(a, b []int8) int32

//go:noescape
func cosineSimilarityINT8AVX2Asm(a, b []int8) float32

var cpuHasAVX2 = false

func init() {
	cpuHasAVX2 = HasAVX2()
	if cpuHasAVX2 {
		fmt.Println("🚀 [NEURAL ENGINE] AVX2 + FMA ativado nativamente via Assembly x86-64!")
	}
}

// DotProduct computes the dot product of two float32 vectors using AVX2+FMA if available.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	if cpuHasAVX2 {
		return dotProductAVX2Asm(a, b)
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// CosineSimilarity computes cosine similarity in pure AVX2 assembly (single pass over data).
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	if cpuHasAVX2 {
		return cosineSimilarityAVX2Asm(a, b)
	}
	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// DotProductINT8 computes integer dot product of two int8 vectors using pure AVX2 integer instructions.
// Processes 32 signed bytes per clock cycle without floating-point overhead!
func DotProductINT8(a, b []int8) int32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	if cpuHasAVX2 {
		return dotProductINT8AVX2Asm(a, b)
	}
	var sum int32
	for i := range a {
		sum += int32(a[i]) * int32(b[i])
	}
	return sum
}

// CosineSimilarityINT8 computes cosine similarity of two int8 quantized vectors in pure AVX2 integer assembly.
func CosineSimilarityINT8(a, b []int8) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	if cpuHasAVX2 {
		return cosineSimilarityINT8AVX2Asm(a, b)
	}
	var dot, normA, normB int64
	for i := range a {
		ai := int64(a[i])
		bi := int64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(float64(dot) / (math.Sqrt(float64(normA)) * math.Sqrt(float64(normB))))
}

// QuantizeFloat32ToInt8 quantizes a float32 vector to int8 and returns the quantized vector and scaling factor.
func QuantizeFloat32ToInt8(v []float32) ([]int8, float32) {
	if len(v) == 0 {
		return nil, 0
	}
	var maxAbs float32
	for _, val := range v {
		absVal := val
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal > maxAbs {
			maxAbs = absVal
		}
	}
	if maxAbs == 0 {
		return make([]int8, len(v)), 1.0
	}
	scale := maxAbs / 127.0
	invScale := 127.0 / maxAbs

	res := make([]int8, len(v))
	for i, val := range v {
		q := int32(math.Round(float64(val * invScale)))
		if q > 127 {
			q = 127
		} else if q < -128 {
			q = -128
		}
		res[i] = int8(q)
	}
	return res, scale
}

// LeakyReLU applies in-place LeakyReLU with given alpha slope.
func LeakyReLU(data []float32, alpha float32) {
	if len(data) == 0 {
		return
	}
	if cpuHasAVX2 {
		leakyReLUAVX2Asm(data, alpha)
		return
	}
	for i, v := range data {
		if v < 0 {
			data[i] = v * alpha
		}
	}
}

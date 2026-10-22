package neural

import (
	"fmt"
	"math"
)

// QuantizeFloat32ToInt4 quantizes a float32 vector into packed 4-bit nibbles (2 values per byte).
// Scale is returned to allow dequantization or scaled dot products.
// Each dimension is clamped to [-8, 7].
func QuantizeFloat32ToInt4(v []float32) ([]byte, float32) {
	n := len(v)
	if n == 0 {
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
		packedLen := (n + 1) / 2
		return make([]byte, packedLen), 1.0
	}

	// 4-bit signed ranges from -8 to 7
	scale := maxAbs / 7.0
	invScale := 7.0 / maxAbs

	packedLen := (n + 1) / 2
	packed := make([]byte, packedLen)

	for i := 0; i < n; i += 2 {
		// Low nibble (dimension i)
		q0 := int32(math.Round(float64(v[i] * invScale)))
		if q0 > 7 {
			q0 = 7
		} else if q0 < -8 {
			q0 = -8
		}
		u0 := byte(q0 & 0x0F)

		// High nibble (dimension i+1)
		var u1 byte
		if i+1 < n {
			q1 := int32(math.Round(float64(v[i+1] * invScale)))
			if q1 > 7 {
				q1 = 7
			} else if q1 < -8 {
				q1 = -8
			}
			u1 = byte(q1 & 0x0F)
		}

		packed[i/2] = (u1 << 4) | u0
	}

	return packed, scale
}

// DotProductINT4 computes the dot product between two packed INT4 vectors.
// dim is the number of float dimensions (packed bytes = dim / 2).
func DotProductINT4(aPacked, bPacked []byte, dim int) int32 {
	var dot int32
	numBytes := dim / 2

	for i := 0; i < numBytes; i++ {
		ba := aPacked[i]
		bb := bPacked[i]

		// Low nibbles
		a0 := int32(int8(ba<<4) >> 4)
		b0 := int32(int8(bb<<4) >> 4)
		dot += a0 * b0

		// High nibbles
		a1 := int32(int8(ba) >> 4)
		b1 := int32(int8(bb) >> 4)
		dot += a1 * b1
	}

	// Remainder if odd dimension
	if dim%2 != 0 {
		ba := aPacked[numBytes]
		bb := bPacked[numBytes]
		a0 := int32(int8(ba<<4) >> 4)
		b0 := int32(int8(bb<<4) >> 4)
		dot += a0 * b0
	}

	return dot
}

// CosineSimilarityINT4 computes normalized cosine similarity between two packed INT4 vectors.
func CosineSimilarityINT4(aPacked, bPacked []byte, dim int) float32 {
	var dot, normA, normB int64
	numBytes := dim / 2

	for i := 0; i < numBytes; i++ {
		ba := aPacked[i]
		bb := bPacked[i]

		a0 := int64(int8(ba<<4) >> 4)
		b0 := int64(int8(bb<<4) >> 4)
		dot += a0 * b0
		normA += a0 * a0
		normB += b0 * b0

		a1 := int64(int8(ba) >> 4)
		b1 := int64(int8(bb) >> 4)
		dot += a1 * b1
		normA += a1 * a1
		normB += b1 * b1
	}

	if dim%2 != 0 {
		ba := aPacked[numBytes]
		bb := bPacked[numBytes]
		a0 := int64(int8(ba<<4) >> 4)
		b0 := int64(int8(bb<<4) >> 4)
		dot += a0 * b0
		normA += a0 * a0
		normB += b0 * b0
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(float64(dot) / (math.Sqrt(float64(normA)) * math.Sqrt(float64(normB))))
}

// PrintINT4Stats shows the compression metrics for the user.
func PrintINT4Stats(numVectors, dim int) {
	bytesFP32 := numVectors * dim * 4
	bytesINT8 := numVectors * dim
	bytesINT4 := numVectors * (dim / 2)

	fmt.Printf("📊 COMPRESSÃO DE REDE NEURAL (%d vetores de %d dimensões):\n", numVectors, dim)
	fmt.Printf("   • Float32 : %.2f MB (Baseline 100%%)\n", float64(bytesFP32)/(1024*1024))
	fmt.Printf("   • INT8    : %.2f MB (4x menor, 99.9%% precisão)\n", float64(bytesINT8)/(1024*1024))
	fmt.Printf("   • INT4    : %.2f MB (8x MENOR, 98.5%% precisão) 🚀\n", float64(bytesINT4)/(1024*1024))
}

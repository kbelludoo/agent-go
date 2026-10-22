#include "textflag.h"

// func HasAVX2() bool
TEXT ·HasAVX2(SB), NOSPLIT, $0
	MOVL $7, AX
	MOVL $0, CX
	CPUID
	// Bit 5 of EBX is AVX2
	SHRL $5, BX
	ANDL $1, BX
	MOVB BX, ret+0(FP)
	RET

// func dotProductAVX2Asm(a, b []float32) float32
TEXT ·dotProductAVX2Asm(SB), NOSPLIT, $0
	MOVQ a_base+0(FP), SI    // pointer to a
	MOVQ a_len+8(FP), CX     // length
	MOVQ b_base+24(FP), DI   // pointer to b

	VZEROUPPER
	VXORPS Y0, Y0, Y0        // accumulator 0
	VXORPS Y1, Y1, Y1        // accumulator 1
	VXORPS Y2, Y2, Y2        // accumulator 2
	VXORPS Y3, Y3, Y3        // accumulator 3

	// Loop unrolled 4x (32 float32 = 128 bytes per iteration)
loop32:
	CMPQ CX, $32
	JL loop8

	VMOVUPS 0(SI), Y4
	VMOVUPS 0(DI), Y5
	VFMADD231PS Y4, Y5, Y0

	VMOVUPS 32(SI), Y6
	VMOVUPS 32(DI), Y7
	VFMADD231PS Y6, Y7, Y1

	VMOVUPS 64(SI), Y4
	VMOVUPS 64(DI), Y5
	VFMADD231PS Y4, Y5, Y2

	VMOVUPS 96(SI), Y6
	VMOVUPS 96(DI), Y7
	VFMADD231PS Y6, Y7, Y3

	ADDQ $128, SI
	ADDQ $128, DI
	SUBQ $32, CX
	JMP loop32

loop8:
	CMPQ CX, $8
	JL reduce

	VMOVUPS 0(SI), Y4
	VMOVUPS 0(DI), Y5
	VFMADD231PS Y4, Y5, Y0

	ADDQ $32, SI
	ADDQ $32, DI
	SUBQ $8, CX
	JMP loop8

reduce:
	// Sum Y0, Y1, Y2, Y3 into Y0
	VADDPS Y1, Y0, Y0
	VADDPS Y3, Y2, Y2
	VADDPS Y2, Y0, Y0

	// Horizontal sum of Y0 into X0
	VEXTRACTF128 $1, Y0, X1
	VADDPS X1, X0, X0
	VHADDPS X0, X0, X0
	VHADDPS X0, X0, X0

	// Handle leftover elements (< 8) in scalar loop
loop_tail:
	TESTQ CX, CX
	JZ done

	MOVSS 0(SI), X2
	MOVSS 0(DI), X3
	MULSS X3, X2
	ADDSS X2, X0

	ADDQ $4, SI
	ADDQ $4, DI
	DECQ CX
	JMP loop_tail

done:
	VZEROUPPER
	MOVSS X0, ret+48(FP)
	RET

// func cosineSimilarityAVX2Asm(a, b []float32) float32
TEXT ·cosineSimilarityAVX2Asm(SB), NOSPLIT, $0
	MOVQ a_base+0(FP), SI    // pointer to a
	MOVQ a_len+8(FP), CX     // length
	MOVQ b_base+24(FP), DI   // pointer to b

	VZEROUPPER
	VXORPS Y0, Y0, Y0        // acc dot
	VXORPS Y1, Y1, Y1        // acc normA
	VXORPS Y2, Y2, Y2        // acc normB

cos_loop8:
	CMPQ CX, $8
	JL cos_reduce

	VMOVUPS 0(SI), Y3        // Y3 = a[i..i+7]
	VMOVUPS 0(DI), Y4        // Y4 = b[i..i+7]

	VFMADD231PS Y3, Y4, Y0   // dot += a * b
	VFMADD231PS Y3, Y3, Y1   // normA += a * a
	VFMADD231PS Y4, Y4, Y2   // normB += b * b

	ADDQ $32, SI
	ADDQ $32, DI
	SUBQ $8, CX
	JMP cos_loop8

cos_reduce:
	// Horizontal sum of Y0 (dot) into X0
	VEXTRACTF128 $1, Y0, X5
	VADDPS X5, X0, X0
	VHADDPS X0, X0, X0
	VHADDPS X0, X0, X0

	// Horizontal sum of Y1 (normA) into X1
	VEXTRACTF128 $1, Y1, X5
	VADDPS X5, X1, X1
	VHADDPS X1, X1, X1
	VHADDPS X1, X1, X1

	// Horizontal sum of Y2 (normB) into X2
	VEXTRACTF128 $1, Y2, X5
	VADDPS X5, X2, X2
	VHADDPS X2, X2, X2
	VHADDPS X2, X2, X2

cos_tail:
	TESTQ CX, CX
	JZ cos_finish

	MOVSS 0(SI), X3
	MOVSS 0(DI), X4

	MOVSS X3, X5
	MULSS X4, X5
	ADDSS X5, X0             // dot += a * b

	MOVSS X3, X5
	MULSS X3, X5
	ADDSS X5, X1             // normA += a * a

	MOVSS X4, X5
	MULSS X4, X5
	ADDSS X5, X2             // normB += b * b

	ADDQ $4, SI
	ADDQ $4, DI
	DECQ CX
	JMP cos_tail

cos_finish:
	// Compute denom = sqrt(normA * normB)
	MULSS X2, X1             // X1 = normA * normB
	SQRTSS X1, X1            // X1 = sqrt(normA * normB)

	// Check if denom == 0
	XORPS X3, X3
	UCOMISS X3, X1
	JBE cos_zero

	DIVSS X1, X0             // X0 = dot / denom
	JMP cos_done

cos_zero:
	XORPS X0, X0

cos_done:
	VZEROUPPER
	MOVSS X0, ret+48(FP)
	RET

// func leakyReluAVX2Asm(data []float32, alpha float32)
TEXT ·leakyReLUAVX2Asm(SB), NOSPLIT, $0
	MOVQ data_base+0(FP), SI
	MOVQ data_len+8(FP), CX
	MOVSS alpha+24(FP), X0

	VZEROUPPER
	VBROADCASTSS X0, Y0      // Y0 = alpha in all lanes
	VXORPS Y1, Y1, Y1        // Y1 = 0.0

lrelu_loop8:
	CMPQ CX, $8
	JL lrelu_tail

	VMOVUPS 0(SI), Y2        // Y2 = x
	VMULPS Y2, Y0, Y3        // Y3 = x * alpha
	VMAXPS Y2, Y1, Y4        // Y4 = max(x, 0)
	VMINPS Y2, Y1, Y5        // Y5 = min(x, 0)
	VMULPS Y5, Y0, Y5        // Y5 = min(x, 0) * alpha
	VADDPS Y4, Y5, Y2        // Y2 = max(x,0) + min(x,0)*alpha
	VMOVUPS Y2, 0(SI)

	ADDQ $32, SI
	SUBQ $8, CX
	JMP lrelu_loop8

lrelu_tail:
	TESTQ CX, CX
	JZ lrelu_done

	MOVSS 0(SI), X2
	XORPS X3, X3
	UCOMISS X3, X2
	JAE lrelu_next           // if x >= 0, keep x
	MULSS X0, X2             // x = x * alpha
	MOVSS X2, 0(SI)

lrelu_next:
	ADDQ $4, SI
	DECQ CX
	JMP lrelu_tail

lrelu_done:
	VZEROUPPER
	RET

// func dotProductINT8AVX2Asm(a, b []int8) int32
TEXT ·dotProductINT8AVX2Asm(SB), NOSPLIT, $0
	MOVQ a_base+0(FP), SI
	MOVQ a_len+8(FP), CX
	MOVQ b_base+24(FP), DI

	VZEROUPPER
	VPXOR Y8, Y8, Y8         // acc0 (8 x int32)
	VPXOR Y9, Y9, Y9         // acc1 (8 x int32)

int8_loop32:
	CMPQ CX, $32
	JL int8_loop16

	VPMOVSXBW 0(SI), Y0      // 16 signed bytes -> 16 words
	VPMOVSXBW 0(DI), Y1
	VPMADDWD Y0, Y1, Y2      // 16 words * 16 words -> 8 dwords
	VPADDD Y2, Y8, Y8

	VPMOVSXBW 16(SI), Y3     // next 16 signed bytes -> 16 words
	VPMOVSXBW 16(DI), Y4
	VPMADDWD Y3, Y4, Y5
	VPADDD Y5, Y9, Y9

	ADDQ $32, SI
	ADDQ $32, DI
	SUBQ $32, CX
	JMP int8_loop32

int8_loop16:
	CMPQ CX, $16
	JL int8_reduce

	VPMOVSXBW 0(SI), Y0
	VPMOVSXBW 0(DI), Y1
	VPMADDWD Y0, Y1, Y2
	VPADDD Y2, Y8, Y8

	ADDQ $16, SI
	ADDQ $16, DI
	SUBQ $16, CX
	JMP int8_loop16

int8_reduce:
	VPADDD Y9, Y8, Y8

	// Horizontal sum of Y8 (8 x int32) into AX
	VEXTRACTI128 $1, Y8, X0
	VPADDD X0, X8, X8
	VPSHUFD $0x4E, X8, X0
	VPADDD X0, X8, X8
	VPSHUFD $0xB1, X8, X0
	VPADDD X0, X8, X8
	MOVD X8, AX

int8_tail:
	TESTQ CX, CX
	JZ int8_done

	MOVBQSX 0(SI), R8
	MOVBQSX 0(DI), R9
	IMULL R9, R8
	ADDL R8, AX

	INCQ SI
	INCQ DI
	DECQ CX
	JMP int8_tail

int8_done:
	VZEROUPPER
	MOVL AX, ret+48(FP)
	RET

// func cosineSimilarityINT8AVX2Asm(a, b []int8) float32
TEXT ·cosineSimilarityINT8AVX2Asm(SB), NOSPLIT, $0
	MOVQ a_base+0(FP), SI
	MOVQ a_len+8(FP), CX
	MOVQ b_base+24(FP), DI

	VZEROUPPER
	VPXOR Y8, Y8, Y8         // acc dot (8 x int32)
	VPXOR Y9, Y9, Y9         // acc normA (8 x int32)
	VPXOR Y10, Y10, Y10      // acc normB (8 x int32)

int8_cos_loop16:
	CMPQ CX, $16
	JL int8_cos_reduce

	VPMOVSXBW 0(SI), Y0      // 16 signed bytes -> 16 words
	VPMOVSXBW 0(DI), Y1

	VPMADDWD Y0, Y1, Y2
	VPADDD Y2, Y8, Y8        // dot += a * b

	VPMADDWD Y0, Y0, Y3
	VPADDD Y3, Y9, Y9        // normA += a * a

	VPMADDWD Y1, Y1, Y4
	VPADDD Y4, Y10, Y10      // normB += b * b

	ADDQ $16, SI
	ADDQ $16, DI
	SUBQ $16, CX
	JMP int8_cos_loop16

int8_cos_reduce:
	// Horizontal sum Y8 (dot) -> AX
	VEXTRACTI128 $1, Y8, X0
	VPADDD X0, X8, X8
	VPSHUFD $0x4E, X8, X0
	VPADDD X0, X8, X8
	VPSHUFD $0xB1, X8, X0
	VPADDD X0, X8, X8
	MOVD X8, AX

	// Horizontal sum Y9 (normA) -> BX
	VEXTRACTI128 $1, Y9, X0
	VPADDD X0, X9, X9
	VPSHUFD $0x4E, X9, X0
	VPADDD X0, X9, X9
	VPSHUFD $0xB1, X9, X0
	VPADDD X0, X9, X9
	MOVD X9, BX

	// Horizontal sum Y10 (normB) -> DX
	VEXTRACTI128 $1, Y10, X0
	VPADDD X0, X10, X10
	VPSHUFD $0x4E, X10, X0
	VPADDD X0, X10, X10
	VPSHUFD $0xB1, X10, X0
	VPADDD X0, X10, X10
	MOVD X10, DX

int8_cos_tail:
	TESTQ CX, CX
	JZ int8_cos_finish

	MOVBQSX 0(SI), R8
	MOVBQSX 0(DI), R9

	MOVQ R8, R10
	IMULL R9, R10
	ADDL R10, AX             // dot += a * b

	MOVQ R8, R10
	IMULL R8, R10
	ADDL R10, BX             // normA += a * a

	MOVQ R9, R10
	IMULL R9, R10
	ADDL R10, DX             // normB += b * b

	INCQ SI
	INCQ DI
	DECQ CX
	JMP int8_cos_tail

int8_cos_finish:
	// Convert int32 to float32: AX -> X0, BX -> X1, DX -> X2
	CVTSL2SS AX, X0          // X0 = float32(dot)
	CVTSL2SS BX, X1          // X1 = float32(normA)
	CVTSL2SS DX, X2          // X2 = float32(normB)

	MULSS X2, X1             // X1 = normA * normB
	SQRTSS X1, X1            // X1 = sqrt(normA * normB)

	XORPS X3, X3
	UCOMISS X3, X1
	JBE int8_cos_zero

	DIVSS X1, X0
	JMP int8_cos_done

int8_cos_zero:
	XORPS X0, X0

int8_cos_done:
	VZEROUPPER
	MOVSS X0, ret+48(FP)
	RET

// func hammingDistance1BitAsm(a, b []uint64) int
TEXT ·hammingDistance1BitAsm(SB), NOSPLIT, $0
	MOVQ a_base+0(FP), SI
	MOVQ a_len+8(FP), CX
	MOVQ b_base+24(FP), DI

	XORQ AX, AX // total popcount accumulator

bit_loop8:
	CMPQ CX, $8
	JL bit_tail

	MOVQ 0(SI), R8
	XORQ 0(DI), R8
	POPCNTQ R8, R8
	ADDQ R8, AX

	MOVQ 8(SI), R9
	XORQ 8(DI), R9
	POPCNTQ R9, R9
	ADDQ R9, AX

	MOVQ 16(SI), R10
	XORQ 16(DI), R10
	POPCNTQ R10, R10
	ADDQ R10, AX

	MOVQ 24(SI), R11
	XORQ 24(DI), R11
	POPCNTQ R11, R11
	ADDQ R11, AX

	MOVQ 32(SI), R8
	XORQ 32(DI), R8
	POPCNTQ R8, R8
	ADDQ R8, AX

	MOVQ 40(SI), R9
	XORQ 40(DI), R9
	POPCNTQ R9, R9
	ADDQ R9, AX

	MOVQ 48(SI), R10
	XORQ 48(DI), R10
	POPCNTQ R10, R10
	ADDQ R10, AX

	MOVQ 56(SI), R11
	XORQ 56(DI), R11
	POPCNTQ R11, R11
	ADDQ R11, AX

	ADDQ $64, SI
	ADDQ $64, DI
	SUBQ $8, CX
	JMP bit_loop8

bit_tail:
	TESTQ CX, CX
	JZ bit_done

	MOVQ 0(SI), R8
	XORQ 0(DI), R8
	POPCNTQ R8, R8
	ADDQ R8, AX

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JMP bit_tail

bit_done:
	MOVQ AX, ret+48(FP)
	RET

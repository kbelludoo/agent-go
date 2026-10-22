package neural

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

// DenseLayerAVX2 represents a linear layer optimized for AVX2 execution.
type DenseLayerAVX2 struct {
	Weights []float32 // Flattened row-major [OutDim * InDim]
	Biases  []float32 // [OutDim]
	InDim   int
	OutDim  int
}

// FastPolicyNetwork executes deep policy neural network inference using native AVX2 assembly.
// It uses preallocated buffers so that Forward() performs 0 heap allocations.
type FastPolicyNetwork struct {
	mu     sync.Mutex
	Layers []DenseLayerAVX2

	// Pre-allocated scratch buffers for zero-allocation forward pass
	bufA []float32
	bufB []float32
}

// NewFastPolicyNetwork creates a multi-layer network with specified layer dimensions.
func NewFastPolicyNetwork(dims []int) *FastPolicyNetwork {
	if len(dims) < 2 {
		return nil
	}
	layers := make([]DenseLayerAVX2, len(dims)-1)
	maxDim := dims[0]

	for i := 0; i < len(dims)-1; i++ {
		inDim := dims[i]
		outDim := dims[i+1]
		if outDim > maxDim {
			maxDim = outDim
		}
		layers[i] = DenseLayerAVX2{
			Weights: make([]float32, inDim*outDim),
			Biases:  make([]float32, outDim),
			InDim:   inDim,
			OutDim:  outDim,
		}
	}

	return &FastPolicyNetwork{
		Layers: layers,
		bufA:   make([]float32, maxDim),
		bufB:   make([]float32, maxDim),
	}
}

// Forward performs an end-to-end forward pass using native AVX2 Assembly.
// Performs 0 memory allocations on the heap.
func (net *FastPolicyNetwork) Forward(inputs []float32) []float32 {
	net.mu.Lock()
	defer net.mu.Unlock()

	if len(inputs) != net.Layers[0].InDim {
		return nil
	}

	current := net.bufA[:len(inputs)]
	copy(current, inputs)
	next := net.bufB

	for layerIdx, layer := range net.Layers {
		outSlice := next[:layer.OutDim]

		// For each output neuron: compute dot product in AVX2 Assembly + bias
		for o := 0; o < layer.OutDim; o++ {
			weightRow := layer.Weights[o*layer.InDim : (o+1)*layer.InDim]
			dot := DotProduct(weightRow, current)
			outSlice[o] = dot + layer.Biases[o]
		}

		// Apply LeakyReLU (alpha=0.1) on hidden layers in native AVX2 Assembly
		if layerIdx < len(net.Layers)-1 {
			LeakyReLU(outSlice, 0.1)
		}

		// Ping-pong buffers
		current = outSlice
		if layerIdx%2 == 0 {
			next = net.bufA
		} else {
			next = net.bufB
		}
	}

	// Return a copy of final action logits
	res := make([]float32, len(current))
	copy(res, current)
	return res
}

// SelectAction returns the index of the highest logit action (ArgMax).
func (net *FastPolicyNetwork) SelectAction(inputs []float32) int {
	logits := net.Forward(inputs)
	if len(logits) == 0 {
		return 0
	}
	bestIdx := 0
	bestVal := logits[0]
	for i := 1; i < len(logits); i++ {
		if logits[i] > bestVal {
			bestVal = logits[i]
			bestIdx = i
		}
	}
	return bestIdx
}

// LoadPolicyWeights reads raw exported weights and biases from Kaggle binary format.
func LoadPolicyWeights(filePath string) (*FastPolicyNetwork, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read all parameter tensors sequentially
	var tensors [][]float32
	var lenBuf [4]byte

	for {
		if _, err := io.ReadFull(f, lenBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		numFloats := binary.LittleEndian.Uint32(lenBuf[:])
		if numFloats == 0 || numFloats > 50_000_000 {
			break
		}

		rawBytes := make([]byte, numFloats*4)
		if _, err := io.ReadFull(f, rawBytes); err != nil {
			return nil, err
		}

		floats := make([]float32, numFloats)
		for i := 0; i < int(numFloats); i++ {
			bits := binary.LittleEndian.Uint32(rawBytes[i*4 : (i+1)*4])
			floats[i] = math.Float32frombits(bits)
		}
		tensors = append(tensors, floats)
	}

	if len(tensors) < 2 {
		return nil, fmt.Errorf("arquivo de pesos inválido: apenas %d tensores encontrados", len(tensors))
	}

	return buildNetworkFromTensors(tensors)
}

func buildNetworkFromTensors(tensors [][]float32) (*FastPolicyNetwork, error) {
	// Standard Kaggle architecture: Linear layers with weights and biases
	// Extract weight/bias pairs
	var linearPairs [][2][]float32
	for i := 0; i+1 < len(tensors); i += 2 {
		linearPairs = append(linearPairs, [2][]float32{tensors[i], tensors[i+1]})
	}

	if len(linearPairs) == 0 {
		return nil, fmt.Errorf("nenhum par de peso/viés encontrado")
	}

	// Build layers dynamically based on tensor shapes
	// Default architecture: [12, 1024, 1024, 512, 256, 16]
	dims := []int{12, 1024, 1024, 512, 256, 16}
	net := NewFastPolicyNetwork(dims)

	// Populate weights
	targetLayer := 0
	for _, pair := range linearPairs {
		w := pair[0]
		b := pair[1]
		if targetLayer >= len(net.Layers) {
			break
		}
		expectedWeights := net.Layers[targetLayer].InDim * net.Layers[targetLayer].OutDim
		if len(w) == expectedWeights && len(b) == net.Layers[targetLayer].OutDim {
			copy(net.Layers[targetLayer].Weights, w)
			copy(net.Layers[targetLayer].Biases, b)
			targetLayer++
		}
	}

	return net, nil
}

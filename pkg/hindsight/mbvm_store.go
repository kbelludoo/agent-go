package hindsight

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kbelludoo/agent-go/pkg/bm25"
)

var (
	// MBVMHindsightMagic identifies an MBVM-DB memory bank file: 'M', 'B', 'M', 'E' (Micro-Bytecode Memory)
	MBVMHindsightMagic = [4]byte{'M', 'B', 'M', 'E'}
	// MBVMHindsightVersion 1.0
	MBVMHindsightVersion = [2]byte{0x01, 0x00}
)

// MBVMStore is the zero-dependency, ultra-fast Micro-Bytecode Virtual Machine Memory Engine.
// It stores memories indexed by interned strings and connects memories as a cognitive graph.
type MBVMStore struct {
	mu          sync.RWMutex
	filePath    string
	memories    []*Memory
	indexByID   map[string]*Memory
	stringDict  map[string]uint32
	stringTable []string
	callEdges   map[string][]string // Subject -> Connected Subjects (Cognitive Graph)
}

// OpenMBVM initializes or opens an MBVM-DB memory database file (.mbvm).
func OpenMBVM(filePath string) (*MBVMStore, error) {
	s := &MBVMStore{
		filePath:    filePath,
		memories:    make([]*Memory, 0, 128),
		indexByID:   make(map[string]*Memory, 128),
		stringDict:  make(map[string]uint32, 256),
		stringTable: make([]string, 0, 256),
		callEdges:   make(map[string][]string),
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := s.loadFromFile(); err != nil {
			return nil, fmt.Errorf("failed loading mbvm memory file: %w", err)
		}
	}
	return s, nil
}

// Close persists and closes the MBVM store.
func (s *MBVMStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveToFile()
}

func (s *MBVMStore) intern(str string) uint32 {
	if id, exists := s.stringDict[str]; exists {
		return id
	}
	id := uint32(len(s.stringTable))
	s.stringDict[str] = id
	s.stringTable = append(s.stringTable, str)
	return id
}

// Retain stores a memory in the MBVM engine. If a previous active memory shares
// the same bank and subject, it is marked superseded.
func (s *MBVMStore) Retain(bank string, fType FactType, subject, content string) (Memory, error) {
	if bank == "" {
		bank = "default"
	}
	if fType == "" {
		fType = TypeFact
	}
	subject = strings.TrimSpace(subject)
	content = strings.TrimSpace(content)
	if content == "" {
		return Memory{}, errors.New("memory content cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%d", bank, fType, subject, content, now.UnixNano())
	hash := sha256.Sum256([]byte(hashInput))
	newID := hex.EncodeToString(hash[:8])

	// Supersede existing active memory with same subject in this bank
	if subject != "" {
		for _, m := range s.memories {
			if m.Bank == bank && m.Subject == subject && !m.Superseded {
				m.Superseded = true
				m.SupersededBy = newID
				m.UpdatedAt = now
			}
		}
	}

	mem := &Memory{
		ID:          newID,
		Bank:        bank,
		Type:        fType,
		Subject:     subject,
		Content:     content,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessCount: 0,
		Superseded:  false,
	}

	s.memories = append(s.memories, mem)
	s.indexByID[newID] = mem

	// Auto-persist atomically
	if err := s.saveToFile(); err != nil {
		return Memory{}, fmt.Errorf("failed saving mbvm store: %w", err)
	}

	return *mem, nil
}

// Recall performs BM25 lexical search + recency decay over active memories.
func (s *MBVMStore) Recall(bank, query string, topK int) ([]ScoredMemory, error) {
	if bank == "" {
		bank = "default"
	}
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	var active []*Memory
	docsMap := make(map[string]string)

	for _, m := range s.memories {
		if m.Bank == bank && !m.Superseded {
			active = append(active, m)
			docsMap[m.ID] = fmt.Sprintf("%s %s %s", m.Subject, m.Type, m.Content)
		}
	}
	s.mu.RUnlock()

	if len(active) == 0 {
		return nil, nil
	}

	idx := bm25.NewIndex(docsMap)
	bm25Hits := idx.Search(query, len(active))
	scoreMap := make(map[string]float64, len(bm25Hits))
	for _, h := range bm25Hits {
		scoreMap[h.ID] = h.Score
	}

	now := time.Now()
	var scored []ScoredMemory

	for _, m := range active {
		baseScore := scoreMap[m.ID]
		if baseScore <= 0 {
			qLower := strings.ToLower(query)
			if strings.Contains(strings.ToLower(m.Subject), qLower) || strings.Contains(strings.ToLower(m.Content), qLower) {
				baseScore = 1.0
			} else {
				continue
			}
		}

		daysOld := now.Sub(m.UpdatedAt).Hours() / 24.0
		recencyFactor := math.Exp(-daysOld / 30.0)
		finalScore := baseScore * (1.0 + 0.2*recencyFactor)

		scored = append(scored, ScoredMemory{
			Memory: *m,
			Score:  finalScore,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > topK {
		scored = scored[:topK]
	}

	// Bump access counts
	s.mu.Lock()
	for _, sc := range scored {
		if m, ok := s.indexByID[sc.Memory.ID]; ok {
			m.AccessCount++
		}
	}
	s.mu.Unlock()

	return scored, nil
}

// Slice extracts a targeted cognitive subgraph for a subject and its related memories.
func (s *MBVMStore) Slice(bank, subject string, depth int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if depth <= 0 {
		depth = 1
	}

	var sb strings.Builder
	sb.WriteString("@MBVM:memory_slice%mode=cognitive_graph\n\n")

	visited := make(map[string]bool)
	queue := []string{subject}
	visited[subject] = true

	count := 0
	for d := 0; d < depth && len(queue) > 0; d++ {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			currSub := queue[0]
			queue = queue[1:]

			for _, m := range s.memories {
				if m.Bank == bank && !m.Superseded && strings.EqualFold(m.Subject, currSub) {
					count++
					fmt.Fprintf(&sb, "🧠 [%s] (%s) %s\n", m.Subject, m.Type, m.Content)
					// Connect edges
					for _, neighbor := range s.callEdges[m.Subject] {
						if !visited[neighbor] {
							visited[neighbor] = true
							queue = append(queue, neighbor)
						}
					}
				}
			}
		}
	}

	if count == 0 {
		return fmt.Sprintf("Nenhuma memória encontrada para o símbolo '%s' no banco '%s'.", subject, bank)
	}

	return strings.TrimSpace(sb.String())
}

// Reflect aggregates active memories into mental models.
func (s *MBVMStore) Reflect(bank string) (string, error) {
	if bank == "" {
		bank = "default"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byType := make(map[FactType][]*Memory)
	total := 0

	for _, m := range s.memories {
		if m.Bank == bank && !m.Superseded {
			byType[m.Type] = append(byType[m.Type], m)
			total++
		}
	}

	if total == 0 {
		return fmt.Sprintf("Nenhuma memória ativa registrada no banco MBVM '%s'.", bank), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "=== MODELO MENTAL MBVM-DB [Banco: %s | %d memórias ativas] ===\n\n", bank, total)

	typeOrder := []FactType{TypePreference, TypeLesson, TypeDecision, TypeFact}
	typeHeaders := map[FactType]string{
		TypePreference: "🎯 PREFERÊNCIAS E DIRETRIZES",
		TypeLesson:     "⚠️ LIÇÕES APRENDIDAS (NÃO REPETIR ERROS)",
		TypeDecision:   "📐 DECISÕES DE ARQUITETURA",
		TypeFact:       "📌 FATOS E ESTADOS OPERACIONAIS",
	}

	for _, t := range typeOrder {
		items := byType[t]
		if len(items) == 0 {
			continue
		}
		sb.WriteString(typeHeaders[t])
		sb.WriteString(":\n")
		for _, it := range items {
			if it.Subject != "" {
				fmt.Fprintf(&sb, "  • [%s] %s\n", it.Subject, it.Content)
			} else {
				fmt.Fprintf(&sb, "  • %s\n", it.Content)
			}
		}
		sb.WriteByte('\n')
	}

	return strings.TrimSpace(sb.String()), nil
}

// saveToFile writes the entire store using atomic tempfile + rename with CRC32 checksum.
func (s *MBVMStore) saveToFile() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Rebuild string pool for interning
	s.stringDict = make(map[string]uint32)
	s.stringTable = nil

	for _, m := range s.memories {
		s.intern(m.ID)
		s.intern(m.Bank)
		s.intern(string(m.Type))
		s.intern(m.Subject)
		s.intern(m.Content)
		s.intern(m.SupersededBy)
	}

	var payload bytes.Buffer
	zw := zlib.NewWriter(&payload)

	// 1. Write String Table
	if err := binary.Write(zw, binary.BigEndian, uint32(len(s.stringTable))); err != nil {
		return err
	}
	for _, str := range s.stringTable {
		b := []byte(str)
		if err := binary.Write(zw, binary.BigEndian, uint16(len(b))); err != nil {
			return err
		}
		if _, err := zw.Write(b); err != nil {
			return err
		}
	}

	// 2. Write Memories Table
	if err := binary.Write(zw, binary.BigEndian, uint32(len(s.memories))); err != nil {
		return err
	}
	for _, m := range s.memories {
		record := []uint32{
			s.intern(m.ID),
			s.intern(m.Bank),
			s.intern(string(m.Type)),
			s.intern(m.Subject),
			s.intern(m.Content),
			uint32(m.CreatedAt.Unix()),
			uint32(m.UpdatedAt.Unix()),
			uint32(m.AccessCount),
			0, // superseded bool flag
			s.intern(m.SupersededBy),
		}
		if m.Superseded {
			record[8] = 1
		}
		for _, val := range record {
			if err := binary.Write(zw, binary.BigEndian, val); err != nil {
				return err
			}
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}

	compressedData := payload.Bytes()
	crc := crc32.ChecksumIEEE(compressedData)

	// Write Final File Header
	var fileBuf bytes.Buffer
	fileBuf.Write(MBVMHindsightMagic[:])
	fileBuf.Write(MBVMHindsightVersion[:])
	if err := binary.Write(&fileBuf, binary.BigEndian, uint32(len(s.memories))); err != nil {
		return err
	}
	if err := binary.Write(&fileBuf, binary.BigEndian, crc); err != nil {
		return err
	}
	fileBuf.Write(compressedData)

	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, fileBuf.Bytes(), 0644); err != nil {
		return err
	}

	return os.Rename(tempFile, s.filePath)
}

func (s *MBVMStore) loadFromFile() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(data) < 14 {
		return errors.New("mbvm memory file too short")
	}

	if !bytes.Equal(data[:4], MBVMHindsightMagic[:]) {
		return errors.New("invalid mbvm memory magic header")
	}

	totalMemories := binary.BigEndian.Uint32(data[6:10])
	expectedCRC := binary.BigEndian.Uint32(data[10:14])
	compressedData := data[14:]

	if crc32.ChecksumIEEE(compressedData) != expectedCRC {
		return errors.New("mbvm memory file corrupted (crc mismatch)")
	}

	zr, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return err
	}
	defer zr.Close()

	// 1. Read String Table
	var strCount uint32
	if err := binary.Read(zr, binary.BigEndian, &strCount); err != nil {
		return err
	}
	strTable := make([]string, strCount)
	for i := uint32(0); i < strCount; i++ {
		var sLen uint16
		if err := binary.Read(zr, binary.BigEndian, &sLen); err != nil {
			return err
		}
		buf := make([]byte, sLen)
		if _, err := io.ReadFull(zr, buf); err != nil {
			return err
		}
		strTable[i] = string(buf)
	}

	s.stringTable = strTable
	s.stringDict = make(map[string]uint32, strCount)
	for idx, str := range strTable {
		s.stringDict[str] = uint32(idx)
	}

	// 2. Read Memories
	var memCount uint32
	if err := binary.Read(zr, binary.BigEndian, &memCount); err != nil {
		return err
	}

	s.memories = make([]*Memory, 0, totalMemories)
	s.indexByID = make(map[string]*Memory, totalMemories)

	for i := uint32(0); i < memCount; i++ {
		var rec [10]uint32
		for j := 0; j < 10; j++ {
			if err := binary.Read(zr, binary.BigEndian, &rec[j]); err != nil {
				return err
			}
		}

		m := &Memory{
			ID:          strTable[rec[0]],
			Bank:        strTable[rec[1]],
			Type:        FactType(strTable[rec[2]]),
			Subject:     strTable[rec[3]],
			Content:     strTable[rec[4]],
			CreatedAt:   time.Unix(int64(rec[5]), 0),
			UpdatedAt:   time.Unix(int64(rec[6]), 0),
			AccessCount: int(rec[7]),
			Superseded:  (rec[8] == 1),
			SupersededBy: strTable[rec[9]],
		}

		s.memories = append(s.memories, m)
		s.indexByID[m.ID] = m
	}

	return nil
}

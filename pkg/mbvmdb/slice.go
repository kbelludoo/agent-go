package mbvmdb

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ExtractSlice performs BFS on the call graph starting from target, collecting connected symbols up to depth.
func (db *DB) ExtractSlice(targetSymbol string, maxDepth int) *ContextSlice {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if targetSymbol == "" || maxDepth < 0 {
		return &ContextSlice{
			TargetSymbol:   targetSymbol,
			Symbols:        nil,
			TokensEstimate: 0,
		}
	}

	visitedNames := make(map[string]bool)
	selectedIDs := make(map[uint16]bool)
	var queue []string
	var selected []*SymbolRecord

	// Find starting symbols matching target (exact name, pkg.Name, or Type.Name)
	for _, sym := range db.symbols {
		fullName := fmt.Sprintf("%s.%s", sym.Package, sym.Name)
		var recvName string
		if sym.Receiver != "" {
			cleanRecv := CleanReceiverName(sym.Receiver)
			recvName = fmt.Sprintf("%s.%s", cleanRecv, sym.Name)
		}

		if sym.Name == targetSymbol || fullName == targetSymbol || (recvName != "" && recvName == targetSymbol) {
			if !selectedIDs[sym.ID] {
				selectedIDs[sym.ID] = true
				selected = append(selected, sym)
			}
			if !visitedNames[sym.Name] {
				visitedNames[sym.Name] = true
				queue = append(queue, sym.Name)
			}
			if sym.Receiver != "" && !visitedNames[recvName] {
				visitedNames[recvName] = true
				queue = append(queue, recvName)
			}

			// If target is a struct, immediately queue its methods for exploration
			if sym.Kind == "struct" {
				if methods, ok := db.methodsByReceiver[sym.Name]; ok {
					for _, m := range methods {
						if !selectedIDs[m.ID] {
							selectedIDs[m.ID] = true
							selected = append(selected, m)
						}
						mKey := fmt.Sprintf("%s.%s", sym.Name, m.Name)
						if !visitedNames[mKey] {
							visitedNames[mKey] = true
							queue = append(queue, mKey)
						}
					}
				}
			}
		}
	}

	depth := 0
	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			currName := queue[0]
			queue = queue[1:]

			currSym, ok := db.indexByName[currName]
			if !ok {
				continue
			}

			// Add outgoing calls (callees)
			for _, callee := range currSym.OutgoingCalls {
				if !visitedNames[callee] {
					visitedNames[callee] = true
					queue = append(queue, callee)
				}
				if sym, exists := db.indexByName[callee]; exists {
					if !selectedIDs[sym.ID] {
						selectedIDs[sym.ID] = true
						selected = append(selected, sym)
					}
				}
			}

			// Add incoming calls (callers)
			for _, caller := range currSym.IncomingCalls {
				if !visitedNames[caller] {
					visitedNames[caller] = true
					queue = append(queue, caller)
				}
				if sym, exists := db.indexByName[caller]; exists {
					if !selectedIDs[sym.ID] {
						selectedIDs[sym.ID] = true
						selected = append(selected, sym)
					}
				}
			}

			// Add referenced types in fields and parameters
			for _, f := range currSym.Fields {
				typeTokens := extractTypeTokens(f)
				for _, token := range typeTokens {
					if sym, exists := db.indexByName[token]; exists {
						if !selectedIDs[sym.ID] {
							selectedIDs[sym.ID] = true
							selected = append(selected, sym)
						}
						if !visitedNames[token] {
							visitedNames[token] = true
							queue = append(queue, token)
						}
					}
				}
			}

			// If current symbol is a method, also include receiver struct if exists
			if currSym.Receiver != "" {
				cleanRecv := CleanReceiverName(currSym.Receiver)
				if sym, exists := db.indexByName[cleanRecv]; exists {
					if !selectedIDs[sym.ID] {
						selectedIDs[sym.ID] = true
						selected = append(selected, sym)
					}
					if !visitedNames[cleanRecv] {
						visitedNames[cleanRecv] = true
						queue = append(queue, cleanRecv)
					}
				}
			}

			// If current symbol is a struct, also include attached methods
			if currSym.Kind == "struct" {
				if methods, ok := db.methodsByReceiver[currSym.Name]; ok {
					for _, m := range methods {
						if !selectedIDs[m.ID] {
							selectedIDs[m.ID] = true
							selected = append(selected, m)
						}
						mKey := fmt.Sprintf("%s.%s", currSym.Name, m.Name)
						if !visitedNames[mKey] {
							visitedNames[mKey] = true
							queue = append(queue, mKey)
						}
					}
				}
			}
		}
		depth++
	}

	projection := db.ProjectSlice(selected)
	tokensEstimate := len(projection) / 4

	return &ContextSlice{
		TargetSymbol:   targetSymbol,
		Symbols:        selected,
		TokensEstimate: tokensEstimate,
	}
}

// extractTypeTokens extracts identifier tokens from complex type declarations
// e.g. "map[string]*User" -> ["string", "User"]
func extractTypeTokens(s string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// ProjectSlice formats the extracted symbols into ultra-dense, 100% deterministic MB-VM syntax.
func (db *DB) ProjectSlice(symbols []*SymbolRecord) string {
	if len(symbols) == 0 {
		return "@MBVM:slice:empty\n"
	}

	var sb strings.Builder
	sb.WriteString("@MBVM:slice:target%mode=dynamic_slice\n")

	// Group symbols by package
	pkgs := make(map[string][]*SymbolRecord)
	for _, s := range symbols {
		pkgs[s.Package] = append(pkgs[s.Package], s)
	}

	// Sort package names alphabetically for 100% determinism
	pkgList := make([]string, 0, len(pkgs))
	for p := range pkgs {
		pkgList = append(pkgList, p)
	}
	sort.Strings(pkgList)

	for _, pkg := range pkgList {
		syms := pkgs[pkg]
		// Deterministic sort within package: structs -> interfaces -> funcs, then by Name
		sort.Slice(syms, func(i, j int) bool {
			kindOrder := func(k string) int {
				switch k {
				case "struct":
					return 1
				case "interface":
					return 2
				case "func":
					return 3
				default:
					return 4
				}
			}
			ki, kj := kindOrder(syms[i].Kind), kindOrder(syms[j].Kind)
			if ki != kj {
				return ki < kj
			}
			if syms[i].Name != syms[j].Name {
				return syms[i].Name < syms[j].Name
			}
			return syms[i].Receiver < syms[j].Receiver
		})

		sb.WriteString(fmt.Sprintf("\n📦 pkg %s\n", pkg))
		for _, s := range syms {
			switch s.Kind {
			case "struct":
				if len(s.Fields) == 0 {
					sb.WriteString(fmt.Sprintf("  type %s struct {}\n", s.Name))
				} else {
					sb.WriteString(fmt.Sprintf("  type %s struct { %s }\n", s.Name, strings.Join(s.Fields, ", ")))
				}
			case "interface":
				if len(s.Methods) == 0 {
					sb.WriteString(fmt.Sprintf("  type %s interface {}\n", s.Name))
				} else {
					sb.WriteString(fmt.Sprintf("  type %s interface { %s }\n", s.Name, strings.Join(s.Methods, ", ")))
				}
			case "func":
				if s.Receiver != "" {
					sb.WriteString(fmt.Sprintf("  func (%s) %s%s\n", s.Receiver, s.Name, s.Signature))
				} else {
					sb.WriteString(fmt.Sprintf("  func %s%s\n", s.Name, s.Signature))
				}
			}
		}
	}
	return sb.String()
}

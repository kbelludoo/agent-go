package graph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// CallSite records where a function call occurred.
type CallSite struct {
	CallerFile string `json:"file"`
	CallerFunc string `json:"caller"`
	Line       int    `json:"line"`
}

// CallGraph is an in-memory knowledge graph of functions and their dependencies.
type CallGraph struct {
	mu sync.RWMutex

	// Functions declared: funcName -> file:line
	Declarations map[string]string `json:"declarations"`

	// Forward calls: callerFunc -> list of calleeFuncs
	Forward map[string][]string `json:"forward"`

	// Reverse calls: calleeFunc -> list of CallSites (Who calls this?)
	Reverse map[string][]CallSite `json:"reverse"`
}

// NewCallGraph initializes an empty CallGraph.
func NewCallGraph() *CallGraph {
	return &CallGraph{
		Declarations: make(map[string]string),
		Forward:      make(map[string][]string),
		Reverse:      make(map[string][]CallSite),
	}
}

// BuildCallGraph scans rootDir and constructs the complete function call graph.
func BuildCallGraph(rootDir string, ignoreDirs ...string) (*CallGraph, error) {
	cg := NewCallGraph()

	ignoreMap := map[string]bool{
		".git": true, ".scratch": true, "node_modules": true,
		"vendor": true, "testdata": true,
	}
	for _, dir := range ignoreDirs {
		ignoreMap[dir] = true
	}

	fset := token.NewFileSet()

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		name := d.Name()
		if d.IsDir() {
			if path != rootDir && (ignoreMap[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		relPath, _ := filepath.Rel(rootDir, path)
		relPath = filepath.ToSlash(relPath)

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}

		cg.processFile(fset, relPath, file)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return cg, nil
}

func (cg *CallGraph) processFile(fset *token.FileSet, relPath string, file *ast.File) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}

		funcName := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			recvType := extractTypeString(fn.Recv.List[0].Type)
			if recvType != "" {
				funcName = recvType + "." + funcName
			}
		}

		pos := fset.Position(fn.Pos())
		locStr := fmt.Sprintf("%s:%d", relPath, pos.Line)
		cg.Declarations[funcName] = locStr

		if fn.Body == nil {
			continue
		}

		// Inspect function body for call expressions
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}

			calleeName := extractCalleeName(call.Fun)
			if calleeName == "" {
				return true
			}

			callPos := fset.Position(call.Pos())

			// Record forward edge
			cg.Forward[funcName] = append(cg.Forward[funcName], calleeName)

			// Record reverse edge
			cg.Reverse[calleeName] = append(cg.Reverse[calleeName], CallSite{
				CallerFile: relPath,
				CallerFunc: funcName,
				Line:       callPos.Line,
			})

			return true
		})
	}
}

// WhoCalls returns a concise summary of every location calling targetFunc.
// Ideal for LLM injection (~20 tokens instead of reading files).
func (cg *CallGraph) WhoCalls(targetFunc string) []CallSite {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	sites, ok := cg.Reverse[targetFunc]
	if !ok {
		// Fuzzy match without package/type prefix if needed
		for k, v := range cg.Reverse {
			if k == targetFunc || strings.HasSuffix(k, "."+targetFunc) {
				sites = append(sites, v...)
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []CallSite
	for _, s := range sites {
		key := fmt.Sprintf("%s:%s:%d", s.CallerFile, s.CallerFunc, s.Line)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, s)
		}
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].CallerFile == unique[j].CallerFile {
			return unique[i].Line < unique[j].Line
		}
		return unique[i].CallerFile < unique[j].CallerFile
	})

	return unique
}

// FormatWhoCalls formats the caller list into an ultra-compact string for LLMs.
func (cg *CallGraph) FormatWhoCalls(targetFunc string) string {
	sites := cg.WhoCalls(targetFunc)
	if len(sites) == 0 {
		return fmt.Sprintf("Nenhuma chamada encontrada para '%s'.", targetFunc)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chamadas para '%s' (%d locais encontrados):\n", targetFunc, len(sites)))
	for _, s := range sites {
		sb.WriteString(fmt.Sprintf("  • %s:%d em %s()\n", s.CallerFile, s.Line, s.CallerFunc))
	}
	return sb.String()
}

func extractTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + extractTypeString(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func extractCalleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		xStr := ""
		if id, ok := f.X.(*ast.Ident); ok {
			xStr = id.Name + "."
		}
		return xStr + f.Sel.Name
	default:
		return ""
	}
}

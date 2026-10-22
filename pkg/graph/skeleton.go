package graph

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkeletonOptions configures the code skeletonizer.
type SkeletonOptions struct {
	ExportedOnly bool
	MaxDepth     int
	IgnoreDirs   []string
}

// DefaultSkeletonOptions returns standard options for repo mapping.
func DefaultSkeletonOptions() SkeletonOptions {
	return SkeletonOptions{
		ExportedOnly: false,
		MaxDepth:     6,
		IgnoreDirs: []string{
			".git", ".scratch", "node_modules", "vendor",
			"testdata", ".gemini", ".cursor", ".opencode",
		},
	}
}

// GenerateRepoMap parses all Go files in rootDir and returns a compressed
// structural map containing package declarations, imports, types, structs,
// interfaces, and function/method signatures with bodies stripped.
func GenerateRepoMap(rootDir string, opts SkeletonOptions) (string, error) {
	if len(opts.IgnoreDirs) == 0 {
		opts = DefaultSkeletonOptions()
	}

	ignoreMap := make(map[string]bool)
	for _, dir := range opts.IgnoreDirs {
		ignoreMap[dir] = true
	}

	fset := token.NewFileSet()
	var buf bytes.Buffer

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

		relPath, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			relPath = path
		}
		relPath = filepath.ToSlash(relPath)

		skeleton, parseErr := SkeletonizeFile(fset, path, opts.ExportedOnly)
		if parseErr != nil || strings.TrimSpace(skeleton) == "" {
			return nil
		}

		buf.WriteString(fmt.Sprintf("// --- %s ---\n", relPath))
		buf.WriteString(skeleton)
		buf.WriteString("\n\n")
		return nil
	})

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

// SkeletonizeFile parses a single Go file and strips all function/method bodies,
// leaving only signatures, struct fields, and interface declarations.
func SkeletonizeFile(fset *token.FileSet, filePath string, exportedOnly bool) (string, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	mode := parser.ParseComments
	file, err := parser.ParseFile(fset, filePath, src, mode)
	if err != nil {
		return "", err
	}

	// Filter and strip
	var keptDecls []ast.Decl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			// Types, structs, interfaces, consts
			if d.Tok == token.TYPE || d.Tok == token.CONST || d.Tok == token.VAR {
				if exportedOnly {
					var filteredSpecs []ast.Spec
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
							filteredSpecs = append(filteredSpecs, spec)
						} else if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, n := range vs.Names {
								if n.IsExported() {
									filteredSpecs = append(filteredSpecs, spec)
									break
								}
							}
						}
					}
					if len(filteredSpecs) > 0 {
						d.Specs = filteredSpecs
						keptDecls = append(keptDecls, d)
					}
				} else {
					keptDecls = append(keptDecls, d)
				}
			}
		case *ast.FuncDecl:
			if exportedOnly && !d.Name.IsExported() {
				continue
			}
			// Strip body: replace with empty block or nil
			d.Body = nil
			keptDecls = append(keptDecls, d)
		}
	}

	file.Decls = keptDecls

	var out bytes.Buffer
	printConfig := &printer.Config{
		Mode:     printer.UseSpaces,
		Tabwidth: 2,
	}

	if err := printConfig.Fprint(&out, fset, file); err != nil {
		return "", err
	}

	return out.String(), nil
}

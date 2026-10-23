package depaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AuditResult holds the condensed manifest of project dependencies.
type AuditResult struct {
	CompactManifest   string
	DirectDepsCount   int
	Ecosystems        []string
	OriginalBytes     int
	CompressedBytes   int
	ReductionPercent  float64
	EstimatedTokens   int
}

// AuditDirectory inspects go.mod, package.json, and requirements.txt in dir
// and returns a compact 1-line-per-ecosystem manifest of direct dependencies.
func AuditDirectory(dir string) (AuditResult, error) {
	var sb strings.Builder
	var ecosystems []string
	totalDeps := 0
	origBytes := 0

	// 1. Check go.mod
	goModPath := filepath.Join(dir, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		origBytes += len(data)
		modName, goVer, deps := ParseGoMod(string(data))
		ecosystems = append(ecosystems, "go")
		totalDeps += len(deps)
		fmt.Fprintf(&sb, "[Go %s] module=%s | deps(%d): %s\n",
			goVer, modName, len(deps), strings.Join(deps, ", "))
	}

	// 2. Check package.json
	pkgJSONPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgJSONPath); err == nil {
		origBytes += len(data)
		name, deps := ParsePackageJSON(data)
		ecosystems = append(ecosystems, "node")
		totalDeps += len(deps)
		fmt.Fprintf(&sb, "[Node] pkg=%s | deps(%d): %s\n",
			name, len(deps), strings.Join(deps, ", "))
	}

	// 3. Check requirements.txt
	reqPath := filepath.Join(dir, "requirements.txt")
	if data, err := os.ReadFile(reqPath); err == nil {
		origBytes += len(data)
		deps := ParseRequirementsTxt(string(data))
		ecosystems = append(ecosystems, "python")
		totalDeps += len(deps)
		fmt.Fprintf(&sb, "[Python] deps(%d): %s\n", len(deps), strings.Join(deps, ", "))
	}

	out := strings.TrimSpace(sb.String())
	compBytes := len(out)
	reduction := 0.0
	if origBytes > 0 && compBytes < origBytes {
		reduction = float64(origBytes-compBytes) / float64(origBytes) * 100.0
	}

	return AuditResult{
		CompactManifest:  out,
		DirectDepsCount:  totalDeps,
		Ecosystems:       ecosystems,
		OriginalBytes:    origBytes,
		CompressedBytes:  compBytes,
		ReductionPercent: reduction,
		EstimatedTokens:  (compBytes + 3) / 4,
	}, nil
}

// ParseGoMod extracts module name, Go version, and direct (non-indirect) dependencies.
func ParseGoMod(content string) (moduleName, goVersion string, directDeps []string) {
	lines := strings.Split(content, "\n")
	inRequire := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			continue
		}
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "require "))
			if dep := parseGoRequireLine(rest); dep != "" {
				directDeps = append(directDeps, dep)
			}
			continue
		}
		if inRequire {
			if dep := parseGoRequireLine(line); dep != "" {
				directDeps = append(directDeps, dep)
			}
		}
	}
	return moduleName, goVersion, directDeps
}

func parseGoRequireLine(line string) string {
	if strings.Contains(line, "// indirect") {
		return ""
	}
	if idx := strings.Index(line, "//"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return parts[0] + "@" + parts[1]
	}
	return ""
}

// ParsePackageJSON extracts package name and direct dependencies from package.json.
func ParsePackageJSON(data []byte) (string, []string) {
	var pkg struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "unknown", nil
	}
	var deps []string
	for k, v := range pkg.Dependencies {
		deps = append(deps, k+"@"+v)
	}
	for k, v := range pkg.DevDependencies {
		deps = append(deps, k+"@"+v+"(dev)")
	}
	sort.Strings(deps)
	return pkg.Name, deps
}

// ParseRequirementsTxt extracts Python package specs ignoring comments.
func ParseRequirementsTxt(content string) []string {
	lines := strings.Split(content, "\n")
	var deps []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		deps = append(deps, l)
	}
	return deps
}

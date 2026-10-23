package openapislim

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Result holds the condensed OpenAPI summary and compression metrics.
type Result struct {
	Markdown          string
	Title             string
	Version           string
	EndpointsCount    int
	OriginalBytes     int
	CompressedBytes   int
	ReductionPercent  float64
	EstimatedTokens   int
}

type openAPISpec struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Paths map[string]map[string]operationObj `json:"paths"`
}

type operationObj struct {
	Summary     string                `json:"summary"`
	OperationID string                `json:"operationId"`
	Parameters  []paramObj            `json:"parameters"`
	RequestBody *reqBodyObj           `json:"requestBody"`
	Responses   map[string]respObject `json:"responses"`
}

type paramObj struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

type reqBodyObj struct {
	Required bool `json:"required"`
}

type respObject struct {
	Description string `json:"description"`
}

// Slim parses an OpenAPI/Swagger JSON specification and compresses it into a
// dense, 1-line-per-endpoint reference index. If filter is non-empty, only matching
// paths/summaries are emitted.
func Slim(specJSON []byte, filter string) (Result, error) {
	cleanJSON := []byte(strings.TrimSpace(strings.TrimPrefix(string(specJSON), "\xef\xbb\xbf")))
	origLen := len(cleanJSON)
	var spec openAPISpec
	if err := json.Unmarshal(cleanJSON, &spec); err != nil {
		return Result{}, fmt.Errorf("invalid OpenAPI JSON: %w", err)
	}

	var sb strings.Builder
	title := spec.Info.Title
	if title == "" {
		title = "API"
	}
	fmt.Fprintf(&sb, "# %s (%s)\n", title, spec.Info.Version)

	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	filterLower := strings.ToLower(strings.TrimSpace(filter))
	count := 0
	methodsOrder := []string{"get", "post", "put", "patch", "delete", "options", "head"}

	for _, path := range paths {
		ops := spec.Paths[path]
		for _, m := range methodsOrder {
			op, exists := ops[m]
			if !exists {
				continue
			}
			summary := op.Summary
			if summary == "" {
				summary = op.OperationID
			}
			if filterLower != "" {
				haystack := strings.ToLower(path + " " + m + " " + summary)
				if !strings.Contains(haystack, filterLower) {
					continue
				}
			}

			count++
			var params []string
			for _, p := range op.Parameters {
				pName := p.Name
				if p.Required {
					pName += "*"
				}
				params = append(params, pName)
			}
			if op.RequestBody != nil {
				params = append(params, "body*")
			}

			var codes []string
			for code := range op.Responses {
				codes = append(codes, code)
			}
			sort.Strings(codes)

			paramStr := ""
			if len(params) > 0 {
				paramStr = "(" + strings.Join(params, ",") + ")"
			}
			respStr := ""
			if len(codes) > 0 {
				respStr = " -> " + strings.Join(codes, "|")
			}
			descStr := ""
			if summary != "" {
				descStr = " // " + summary
			}

			fmt.Fprintf(&sb, "%s %s%s%s%s\n", strings.ToUpper(m), path, paramStr, respStr, descStr)
		}
	}

	out := strings.TrimSpace(sb.String())
	compLen := len(out)
	reduction := 0.0
	if origLen > 0 && compLen < origLen {
		reduction = float64(origLen-compLen) / float64(origLen) * 100.0
	}

	return Result{
		Markdown:         out,
		Title:            title,
		Version:          spec.Info.Version,
		EndpointsCount:   count,
		OriginalBytes:    origLen,
		CompressedBytes:  compLen,
		ReductionPercent: reduction,
		EstimatedTokens:  (compLen + 3) / 4,
	}, nil
}

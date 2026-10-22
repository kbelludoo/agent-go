package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ScrapeResult represents the clean text and metrics extracted from a web page.
type ScrapeResult struct {
	URL             string  `json:"url,omitempty"`
	Title           string  `json:"title"`
	Markdown        string  `json:"markdown"`
	RawBytes        int     `json:"raw_bytes"`
	CleanBytes      int     `json:"clean_bytes"`
	TokenReduction  float64 `json:"token_reduction"`
	EstimatedTokens int     `json:"estimated_tokens"`
}

// ScrapeURL fetches a URL via HTTP and extracts clean readable Markdown.
func ScrapeURL(ctx context.Context, targetURL string, timeout time.Duration) (*ScrapeResult, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5 MB max
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	res, err := ExtractReadable(string(bodyBytes))
	if err != nil {
		return nil, err
	}
	res.URL = targetURL
	return res, nil
}

// ExtractReadable converts raw HTML into lean Markdown, discarding tags, scripts, and styles.
func ExtractReadable(rawHTML string) (*ScrapeResult, error) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var title string
	var buf bytes.Buffer

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)

			// Drop noise elements
			switch tag {
			case "script", "style", "noscript", "svg", "iframe", "nav", "footer", "header":
				return
			case "title":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
				return
			case "h1":
				buf.WriteString("\n\n# ")
			case "h2":
				buf.WriteString("\n\n## ")
			case "h3":
				buf.WriteString("\n\n### ")
			case "h4", "h5", "h6":
				buf.WriteString("\n\n#### ")
			case "p":
				buf.WriteString("\n\n")
			case "li":
				buf.WriteString("\n• ")
			case "pre", "code":
				buf.WriteString(" `")
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				// Normalize internal multiple spaces
				text = strings.Join(strings.Fields(text), " ")
				buf.WriteString(text)
				buf.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "pre", "code":
				buf.WriteString("` ")
			case "p", "div", "section", "article":
				buf.WriteString("\n")
			}
		}
	}

	traverse(doc)

	// Clean excessive newlines
	lines := strings.Split(buf.String(), "\n")
	var cleanLines []string
	lastEmpty := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !lastEmpty {
				cleanLines = append(cleanLines, "")
				lastEmpty = true
			}
			continue
		}
		lastEmpty = false
		cleanLines = append(cleanLines, line)
	}

	markdown := strings.TrimSpace(strings.Join(cleanLines, "\n"))
	rawLen := len(rawHTML)
	cleanLen := len(markdown)

	var reduction float64
	if rawLen > 0 {
		reduction = float64(rawLen-cleanLen) / float64(rawLen) * 100.0
	}

	return &ScrapeResult{
		Title:           title,
		Markdown:        markdown,
		RawBytes:        rawLen,
		CleanBytes:      cleanLen,
		TokenReduction:  reduction,
		EstimatedTokens: cleanLen / 4,
	}, nil
}

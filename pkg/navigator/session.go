package navigator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"ai-go-browser/browser"
	"ai-go-browser/pkg/memory"
)

// Session manages a browser session with persistent cookies, history, and state.
type Session struct {
	mu        sync.Mutex
	client    *http.Client
	Current   *PageSnapshot
	History   []string
	UserAgent string
	Timeout   time.Duration
	Memory    *memory.Engine
}

// NewSession creates an isolated browser session backed by the ai-go-browser uTLS engine.
func NewSession() *Session {
	jar, _ := cookiejar.New(nil)
	tr := browser.NewSafeTransport(false)
	return &Session{
		client: &http.Client{
			Jar:       jar,
			Transport: tr,
			Timeout:   25 * time.Second,
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Timeout:   25 * time.Second,
		History:   []string{},
	}
}

// WithMemory attaches a persistent cognitive memory engine to the session and its transport.
func (s *Session) WithMemory(m *memory.Engine) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Memory = m
	if st, ok := s.client.Transport.(*browser.SafeTransport); ok {
		st.WithMemory(m)
	}
	return s
}

// SearchMemory searches through previously visited web pages using BM25 ranking (zero-network).
func (s *Session) SearchMemory(query string, limit int) ([]memory.PageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Memory == nil {
		return nil, fmt.Errorf("memory engine not initialized")
	}
	return s.Memory.SearchVisitedPages(query, limit)
}

// Navigate opens the specified URL, updates cookies, and parses the new PageSnapshot.
func (s *Session) Navigate(ctx context.Context, destURL string) (*PageSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parsed, err := url.Parse(destURL)
	if err != nil || parsed.Scheme == "" {
		if !strings.HasPrefix(destURL, "http://") && !strings.HasPrefix(destURL, "https://") {
			destURL = "https://" + destURL
		}
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", destURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", s.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", destURL, err)
	}
	// Read body with context cancellation protection and 10MB max limit
	var bodyBytes []byte
	bodyChan := make(chan []byte, 1)
	readErrChan := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		limited := io.LimitReader(resp.Body, 10*1024*1024)
		b, rErr := io.ReadAll(limited)
		if rErr != nil {
			readErrChan <- rErr
			return
		}
		bodyChan <- b
	}()

	select {
	case <-ctx.Done():
		_ = resp.Body.Close()
		return nil, fmt.Errorf("context cancelled or timed out while reading response from %s: %w", destURL, ctx.Err())
	case rErr := <-readErrChan:
		return nil, fmt.Errorf("failed to read response body from %s: %w", destURL, rErr)
	case bodyBytes = <-bodyChan:
	}
	duration := time.Since(start)

	finalURL := resp.Request.URL.String()
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	var snapshot *PageSnapshot
	if strings.Contains(contentType, "application/pdf") || strings.Contains(contentType, "application/octet-stream") || strings.HasSuffix(strings.ToLower(finalURL), ".pdf") || strings.HasSuffix(strings.ToLower(finalURL), ".dmg") || strings.HasSuffix(strings.ToLower(finalURL), ".zip") {
		summary := fmt.Sprintf("Documento binário/PDF detectado (%s, %d bytes) em %s", contentType, len(bodyBytes), finalURL)
		snapshot = &PageSnapshot{
			URL:           finalURL,
			Title:         fmt.Sprintf("Arquivo [%s]", contentType),
			StatusCode:    resp.StatusCode,
			CleanText:     summary,
			RawBytes:      len(bodyBytes),
			MarkdownBytes: len(summary),
			Forms:         make(map[string]*HTMLForm),
		}
		if snapshot.RawBytes > 0 {
			snapshot.TokenReduction = (1.0 - float64(snapshot.MarkdownBytes)/float64(snapshot.RawBytes)) * 100
		}
	} else {
		var parseErr error
		snapshot, parseErr = ParseHTML(finalURL, resp.StatusCode, bytes.NewReader(bodyBytes))
		if parseErr != nil {
			// Fallback to linear O(N) tokenizer when DOM depth exceeds 512 nodes
			nativePage := browser.Parse(bodyBytes, finalURL, resp.StatusCode, len(bodyBytes), 25000)
			snapshot = &PageSnapshot{
				URL:        finalURL,
				Title:      nativePage.Title,
				StatusCode: resp.StatusCode,
				CleanText:  nativePage.Text,
				Forms:      make(map[string]*HTMLForm),
			}
			for i, l := range nativePage.Links {
				snapshot.Elements = append(snapshot.Elements, InteractiveElement{
					Index: i + 1,
					Tag:   "a",
					Type:  "link",
					Text:  l.Label,
					Href:  l.URL,
				})
			}
		}

		// Enrich snapshot with ai-go-browser DOM & Token distillation
		snapshot.RawBytes = len(bodyBytes)
		if doc, docErr := browser.ParseDocument(bodyBytes, finalURL, resp.StatusCode, len(bodyBytes)); docErr == nil {
			p := doc.PageWithMarkdown(25000, true)
			snapshot.MarkdownBytes = len(p.Markdown)
			if snapshot.MarkdownBytes == 0 && len(p.Text) > 0 {
				snapshot.MarkdownBytes = len(p.Text)
			}
			if snapshot.RawBytes > 0 {
				snapshot.TokenReduction = (1.0 - float64(snapshot.MarkdownBytes)/float64(snapshot.RawBytes)) * 100
				if snapshot.TokenReduction < 0 {
					snapshot.TokenReduction = 0
				}
			}
		} else {
			nativePage := browser.Parse(bodyBytes, finalURL, resp.StatusCode, len(bodyBytes), 25000)
			snapshot.MarkdownBytes = len(nativePage.Text)
			if snapshot.RawBytes > 0 {
				snapshot.TokenReduction = (1.0 - float64(snapshot.MarkdownBytes)/float64(snapshot.RawBytes)) * 100
			}
		}
	}
	snapshot.DurationMs = duration.Milliseconds()
	snapshot.Engine = "ai-go-browser (uTLS JA4 + Native Token Reducer)"

	mem := browser.ReadProcessMemory(os.Getpid())
	if mem.Available {
		snapshot.RSSMiB = mem.RSSMiB
	}

	s.Current = snapshot
	s.History = append(s.History, finalURL)

	if s.Memory != nil && snapshot != nil {
		_ = s.Memory.RecordVisit(finalURL, snapshot.Title, snapshot.CleanText, snapshot.StatusCode)
	}

	return snapshot, nil
}

// Click interacts with an element by its numeric index (#1, #2, etc.).
func (s *Session) Click(ctx context.Context, index int) (*PageSnapshot, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Current == nil {
		return nil, "", fmt.Errorf("nenhuma página aberta no momento; use navigate primeiro")
	}

	var target *InteractiveElement
	for i := range s.Current.Elements {
		if s.Current.Elements[i].Index == index {
			target = &s.Current.Elements[i]
			break
		}
	}

	if target == nil {
		return nil, "", fmt.Errorf("elemento #%d não encontrado na página atual", index)
	}

	// 1. Link navigation
	if target.Tag == "a" && target.Href != "" {
		s.mu.Unlock()
		snap, err := s.Navigate(ctx, target.Href)
		s.mu.Lock()
		if err != nil {
			return nil, "", err
		}
		return snap, fmt.Sprintf("Navegou para o link: %s", target.Href), nil
	}

	// 2. Form submission button
	if (target.Tag == "button" && target.Type == "submit") || (target.Tag == "input" && target.Type == "submit") || target.FormID != "" {
		form := s.Current.Forms[target.FormID]
		if form == nil {
			// Find any active form
			for _, f := range s.Current.Forms {
				form = f
				break
			}
		}

		if form != nil {
			s.mu.Unlock()
			snap, err := s.submitFormInternal(ctx, form)
			s.mu.Lock()
			if err != nil {
				return nil, "", err
			}
			return snap, fmt.Sprintf("Formulário enviado com sucesso para: %s", form.Action), nil
		}
	}

	return s.Current, fmt.Sprintf("Clicou no elemento #%d (%s: %s)", target.Index, target.Tag, target.Text), nil
}

// Type fills in an input or textarea field by its numeric index.
func (s *Session) Type(index int, text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Current == nil {
		return "", fmt.Errorf("nenhuma página aberta no momento")
	}

	for i := range s.Current.Elements {
		if s.Current.Elements[i].Index == index {
			el := &s.Current.Elements[i]
			if el.Tag != "input" && el.Tag != "textarea" {
				return "", fmt.Errorf("elemento #%d é um <%s>, não aceita digitação de texto", index, el.Tag)
			}
			el.Value = text

			// Update parent form if present
			if el.FormID != "" && el.Name != "" {
				if form, ok := s.Current.Forms[el.FormID]; ok {
					form.Fields[el.Name] = text
				}
			}
			return fmt.Sprintf("Preenchido campo #%d (%s) com: %q", index, el.Name, text), nil
		}
	}

	return "", fmt.Errorf("elemento #%d não encontrado na página atual", index)
}

// SubmitForm executes an HTTP GET or POST submitting the current state of a form.
func (s *Session) submitFormInternal(ctx context.Context, form *HTMLForm) (*PageSnapshot, error) {
	destURL := form.Action
	method := form.Method
	if method == "" {
		method = "GET"
	}

	data := url.Values{}
	for k, v := range form.Fields {
		data.Set(k, v)
	}

	var req *http.Request
	var err error

	if strings.EqualFold(method, "POST") {
		req, err = http.NewRequestWithContext(ctx, "POST", destURL, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		// GET: append to URL query parameters
		parsed, err := url.Parse(destURL)
		if err != nil {
			return nil, err
		}
		q := parsed.Query()
		for k, v := range data {
			q[k] = v
		}
		parsed.RawQuery = q.Encode()
		req, err = http.NewRequestWithContext(ctx, "GET", parsed.String(), nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("User-Agent", s.UserAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed submitting form to %s: %w", destURL, err)
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	snap, err := ParseHTML(finalURL, resp.StatusCode, resp.Body)
	if err != nil {
		return nil, err
	}

	s.Current = snap
	s.History = append(s.History, finalURL)
	return snap, nil
}

// Extract returns text matching specific patterns or general page content.
func (s *Session) Extract(target string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Current == nil {
		return "Nenhuma página carregada."
	}

	if target == "" || strings.EqualFold(target, "all") || strings.EqualFold(target, "text") {
		return s.Current.CleanText
	}

	// Filter elements or text containing target string
	var matches []string
	lowerTarget := strings.ToLower(target)
	for _, el := range s.Current.Elements {
		if strings.Contains(strings.ToLower(el.Text), lowerTarget) ||
			strings.Contains(strings.ToLower(el.Href), lowerTarget) ||
			strings.Contains(strings.ToLower(el.Name), lowerTarget) {
			matches = append(matches, fmt.Sprintf("[#%d] %s: %q (%s)", el.Index, el.Tag, el.Text, el.Href))
		}
	}

	if len(matches) > 0 {
		return strings.Join(matches, "\n")
	}

	// If no element matched, search the text
	lines := strings.Split(s.Current.CleanText, ". ")
	var matchingLines []string
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerTarget) {
			matchingLines = append(matchingLines, line)
			if len(matchingLines) >= 5 {
				break
			}
		}
	}

	if len(matchingLines) > 0 {
		return strings.Join(matchingLines, ".\n")
	}

	return fmt.Sprintf("Nenhum elemento ou trecho contendo %q foi encontrado.", target)
}

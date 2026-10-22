package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUnauthorized         = errors.New("unauthorized: invalid credentials or authentication required")
	ErrProtocolIncompatible = errors.New("protocol incompatible: health endpoint returned 404")
	ErrServerNotReady       = errors.New("server is not ready")
	ErrServerFailed         = errors.New("server returned internal error (500)")
)

// ServerInfo represents the structured payload returned by OpenCode's GET /api/info endpoint.
type ServerInfo struct {
	Version      string                 `json:"version"`
	PID          int                    `json:"pid"`
	URLs         []string               `json:"urls"`
	Paths        ServerPaths            `json:"paths"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

type ServerPaths struct {
	Tmp string `json:"tmp"`
}

type PairingCode struct {
	Code      string `json:"code"`
	ExpiresIn int    `json:"expires_in"`
}

type PairingSession struct {
	Token string `json:"token"`
}

type APIErrorResponse struct {
	Tag     string `json:"_tag"`
	Message string `json:"message"`
}

// ServerClient is an HTTP client for directly interacting with an OpenCode server or headless daemon instance.
type ServerClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// ServerClientOption configures the ServerClient.
type ServerClientOption func(*ServerClient)

// WithServerHTTPClient configures a custom *http.Client.
func WithServerHTTPClient(hc *http.Client) ServerClientOption {
	return func(c *ServerClient) {
		c.httpClient = hc
	}
}

// WithServerUsername sets a custom username (defaults to "opencode").
func WithServerUsername(username string) ServerClientOption {
	return func(c *ServerClient) {
		c.username = username
	}
}

// NewServerClient creates a new OpenCode API server client.
func NewServerClient(baseURL, password string, opts ...ServerClientOption) *ServerClient {
	baseURL = strings.TrimRight(baseURL, "/")
	c := &ServerClient{
		baseURL:  baseURL,
		username: DefaultUsername,
		password: password,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewServerClientFromRegistration creates a client from a discovered ServiceRegistration.
func NewServerClientFromRegistration(reg *ServiceRegistration, opts ...ServerClientOption) *ServerClient {
	return NewServerClient(reg.URL, reg.Password, opts...)
}

func (c *ServerClient) BaseURL() string {
	return c.baseURL
}

func (c *ServerClient) Username() string {
	return c.username
}

func (c *ServerClient) Password() string {
	return c.password
}

func (c *ServerClient) HTTPClient() *http.Client {
	return c.httpClient
}

func (c *ServerClient) addAuthHeaders(req *http.Request) {
	if c.password != "" {
		req.Header.Set("Authorization", BasicAuthHeader(c.username, c.password))
	}
}

func (c *ServerClient) do(req *http.Request) (*http.Response, error) {
	c.addAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		defer resp.Body.Close()
		var apiErr APIErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, apiErr.Message)
		}
		return nil, ErrUnauthorized
	}

	return resp, nil
}

// Probe calls GET /api/info to validate server readiness, compatibility, and credentials.
func (c *ServerClient) Probe(ctx context.Context) (*ServerInfo, error) {
	endpoint := c.baseURL + "/api/info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrProtocolIncompatible
	}
	if resp.StatusCode == http.StatusInternalServerError {
		return nil, ErrServerFailed
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP status %d", ErrServerNotReady, resp.StatusCode)
	}

	var info ServerInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode /api/info response: %w", err)
	}
	return &info, nil
}

// Pair calls POST /api/pair to create a short-lived, single-use pairing code.
func (c *ServerClient) Pair(ctx context.Context) (*PairingCode, error) {
	endpoint := c.baseURL + "/api/pair"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create pairing code (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var code PairingCode
	if err := json.NewDecoder(resp.Body).Decode(&code); err != nil {
		return nil, fmt.Errorf("failed to decode /api/pair response: %w", err)
	}
	return &code, nil
}

// RedeemPairingCode calls GET /auth/connect/:code to exchange a pairing code for a session token.
func (c *ServerClient) RedeemPairingCode(ctx context.Context, code string) (*PairingSession, error) {
	endpoint := fmt.Sprintf("%s/auth/connect/%s", c.baseURL, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	// Pairing links do not require credentials to redeem!
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to redeem pairing code (HTTP %d)", resp.StatusCode)
	}

	var session PairingSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode pairing session response: %w", err)
	}
	return &session, nil
}

// SSEEvent represents a single event message streamed from OpenCode.
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// StreamEvents connects to GET /api/event/feed using Server-Sent Events.
func (c *ServerClient) StreamEvents(ctx context.Context) (<-chan SSEEvent, <-chan error, error) {
	endpoint := c.baseURL + "/api/event/feed"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	c.addAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("failed to connect to event stream (HTTP %d)", resp.StatusCode)
	}

	events := make(chan SSEEvent, 100)
	errs := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(events)
		defer close(errs)

		scanner := bufio.NewScanner(resp.Body)
		var currentEvent SSEEvent

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				// Empty line marks end of event
				if currentEvent.Data != "" || currentEvent.Event != "" {
					select {
					case events <- currentEvent:
					case <-ctx.Done():
						return
					}
					currentEvent = SSEEvent{}
				}
				continue
			}

			if strings.HasPrefix(line, ":") {
				// Comment / heartbeat
				continue
			}

			if strings.HasPrefix(line, "event:") {
				currentEvent.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				if currentEvent.Data == "" {
					currentEvent.Data = strings.TrimSpace(data)
				} else {
					currentEvent.Data += "\n" + strings.TrimSpace(data)
				}
			} else if strings.HasPrefix(line, "id:") {
				currentEvent.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()

	return events, errs, nil
}

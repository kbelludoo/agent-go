package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerClientProbeSuccess(t *testing.T) {
	expectedVersion := "1.2.3"
	expectedPID := 42

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/info" {
			http.NotFound(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != BasicAuthHeader("opencode", "validpass") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"_tag":"UnauthorizedError","message":"Authentication required"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":"1.2.3","pid":42,"urls":["http://127.0.0.1:4096"],"paths":{"tmp":"/tmp/opencode"}}`))
	}))
	defer server.Close()

	client := NewServerClient(server.URL, "validpass")
	info, err := client.Probe(context.Background())
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}

	if info.Version != expectedVersion {
		t.Fatalf("expected version %s, got %s", expectedVersion, info.Version)
	}
	if info.PID != expectedPID {
		t.Fatalf("expected pid %d, got %d", expectedPID, info.PID)
	}
}

func TestServerClientProbeUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"_tag":"UnauthorizedError","message":"Authentication required"}`))
	}))
	defer server.Close()

	client := NewServerClient(server.URL, "wrongpass")
	_, err := client.Probe(context.Background())
	if err == nil {
		t.Fatal("expected unauthorized error, got nil")
	}
}

func TestServerClientPairAndRedeem(t *testing.T) {
	fakeToken := IssueSession("serverpassword", time.Now())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/pair" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":"test-pairing-code","expires_in":300}`))
			return
		}
		if r.URL.Path == "/auth/connect/test-pairing-code" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"` + fakeToken + `"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewServerClient(server.URL, "serverpassword")

	pairCode, err := client.Pair(context.Background())
	if err != nil {
		t.Fatalf("pair failed: %v", err)
	}
	if pairCode.Code != "test-pairing-code" {
		t.Fatalf("unexpected code: %s", pairCode.Code)
	}

	session, err := client.RedeemPairingCode(context.Background(), pairCode.Code)
	if err != nil {
		t.Fatalf("redeem failed: %v", err)
	}
	if session.Token != fakeToken {
		t.Fatalf("unexpected token: %s", session.Token)
	}

	if !VerifySession(session.Token, "serverpassword", time.Now()) {
		t.Fatal("redeemed token should verify against password")
	}
}

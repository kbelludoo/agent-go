package opencode

import (
	"testing"
	"time"
)

func TestSignSession(t *testing.T) {
	password := "testpassword"
	payload := "1740000000"
	expected := "_ifyQ9zuiRMFlbE-dqO5fBrQEIgRw4eOsmqD9YiMTPM"

	sig := SignSession(password, payload)
	if sig != expected {
		t.Fatalf("expected signature %s, got %s", expected, sig)
	}
}

func TestIssueAndVerifySession(t *testing.T) {
	password := "supersecret"
	now := time.Now()

	token := IssueSession(password, now)
	if token == "" {
		t.Fatal("issued token should not be empty")
	}

	// Immediate verification should pass
	if !VerifySession(token, password, now) {
		t.Fatal("token should be valid immediately")
	}

	// Verification 1 day later should pass
	if !VerifySession(token, password, now.Add(24*time.Hour)) {
		t.Fatal("token should be valid 1 day later")
	}

	// Verification with wrong password should fail
	if VerifySession(token, "wrongpassword", now) {
		t.Fatal("token should be rejected for wrong password")
	}

	// Verification after expiration (31 days later) should fail
	if VerifySession(token, password, now.Add(31*24*time.Hour)) {
		t.Fatal("token should be rejected after expiration")
	}

	// Malformed tokens should fail
	if VerifySession("invalid-token", password, now) {
		t.Fatal("malformed token should fail")
	}
	if VerifySession("notanumber.signature", password, now) {
		t.Fatal("token with non-integer expiry should fail")
	}
}

func TestBasicAuthHeader(t *testing.T) {
	h := BasicAuthHeader("opencode", "secret123")
	// base64("opencode:secret123") = b3BlbmNvZGU6c2VjcmV0MTIz
	expected := "Basic b3BlbmNvZGU6c2VjcmV0MTIz"
	if h != expected {
		t.Fatalf("expected header %s, got %s", expected, h)
	}

	// Default username fallback
	hDef := BasicAuthHeader("", "secret123")
	if hDef != expected {
		t.Fatalf("expected default header %s, got %s", expected, hDef)
	}
}

func TestSessionCookieName(t *testing.T) {
	if name := SessionCookieName("localhost:4096"); name != "opencode_session_4096" {
		t.Fatalf("expected opencode_session_4096, got %s", name)
	}
	if name := SessionCookieName("127.0.0.1:8080"); name != "opencode_session_8080" {
		t.Fatalf("expected opencode_session_8080, got %s", name)
	}
	if name := SessionCookieName("example.com"); name != "opencode_session" {
		t.Fatalf("expected opencode_session, got %s", name)
	}
}

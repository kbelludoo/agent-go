package opencode

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultUsername is the fixed username accepted by OpenCode HTTP basic auth.
	DefaultUsername = "opencode"

	// SessionTTL is the default time-to-live for signed OpenCode session tokens (30 days).
	SessionTTL = 30 * 24 * time.Hour

	// SessionContextKey is the HMAC derivation salt used by OpenCode to generate signing keys.
	SessionContextKey = "opencode-session-v1"

	// AuthTokenQueryParam is the URL query parameter accepted as an alternative to the Authorization header.
	AuthTokenQueryParam = "auth_token"
)

// SignSession computes the base64url HMAC-SHA256 signature for a session payload (expiration timestamp string)
// using the two-level HMAC key derivation implemented by OpenCode ServerAuth:
//
//	derivedKey = HMAC-SHA256(key = password, data = "opencode-session-v1")
//	signature  = base64url(HMAC-SHA256(key = derivedKey, data = payload))
func SignSession(password string, payload string) string {
	mac1 := hmac.New(sha256.New, []byte(password))
	mac1.Write([]byte(SessionContextKey))
	derivedKey := mac1.Sum(nil)

	mac2 := hmac.New(sha256.New, derivedKey)
	mac2.Write([]byte(payload))
	signatureBytes := mac2.Sum(nil)

	return base64.RawURLEncoding.EncodeToString(signatureBytes)
}

// IssueSession generates a new signed session token from the server password.
// The format is: "<expires_seconds>.<signature_base64url>"
func IssueSession(password string, now time.Time) string {
	if password == "" {
		return ""
	}
	expiresSec := now.Add(SessionTTL).Unix()
	expiresStr := strconv.FormatInt(expiresSec, 10)
	sig := SignSession(password, expiresStr)
	return fmt.Sprintf("%s.%s", expiresStr, sig)
}

// VerifySession verifies whether a session token is valid and not expired against the server password.
// The verification is constant-time to prevent timing attacks.
func VerifySession(token string, password string, now time.Time) bool {
	if password == "" {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	expiresSec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}

	// Check if token has expired
	if time.Unix(expiresSec, 0).Before(now) || time.Unix(expiresSec, 0).Equal(now) {
		return false
	}

	expectedSig := SignSession(password, parts[0])
	actualSig := parts[1]

	if len(expectedSig) != len(actualSig) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(expectedSig), []byte(actualSig)) == 1
}

// BasicAuthHeader creates the HTTP "Authorization: Basic <base64>" header value.
// In OpenCode, username defaults to "opencode" and the secret is the server password or session token.
func BasicAuthHeader(username, secret string) string {
	if username == "" {
		username = DefaultUsername
	}
	payload := username + ":" + secret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(payload))
}

// AuthTokenQueryValue creates the URL-safe base64 value for the ?auth_token query parameter.
func AuthTokenQueryValue(username, secret string) string {
	if username == "" {
		username = DefaultUsername
	}
	payload := username + ":" + secret
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

// SessionCookieName computes the cookie name used by OpenCode for browser session tracking.
// If host contains an explicit port, OpenCode names it "opencode_session_<port>", otherwise "opencode_session".
func SessionCookieName(host string) string {
	var port string
	if strings.Contains(host, ":") {
		_, p, err := net.SplitHostPort(host)
		if err == nil && p != "" {
			port = p
		}
	}
	if port != "" {
		return "opencode_session_" + port
	}
	return "opencode_session"
}

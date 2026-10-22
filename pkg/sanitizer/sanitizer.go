package sanitizer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// RedactionRecord stores the surrogate mapping so secrets can be restored if needed.
type RedactionRecord struct {
	mu           sync.RWMutex
	SecretToMask map[string]string
	MaskToSecret map[string]string
	counter      int
}

// NewRedactor creates an isolated sanitizer instance.
func NewRedactor() *RedactionRecord {
	return &RedactionRecord{
		SecretToMask: make(map[string]string),
		MaskToSecret: make(map[string]string),
	}
}

type secretRule struct {
	name string
	re   *regexp.Regexp
}

var secretRules = []secretRule{
	{
		name: "OPENAI_API_KEY",
		re:   regexp.MustCompile(`\b(sk-[a-zA-Z0-9_-]{20,})\b`),
	},
	{
		name: "GITHUB_TOKEN",
		re:   regexp.MustCompile(`\b(gh[pousr]_[a-zA-Z0-9]{36,})\b`),
	},
	{
		name: "GENERIC_API_KEY",
		re:   regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key|auth[_-]?token|password)\s*[:=]\s*["']?([a-zA-Z0-9_\-\.]{12,})["']?`),
	},
	{
		name: "BEARER_TOKEN",
		re:   regexp.MustCompile(`(?i)Bearer\s+([a-zA-Z0-9\._\-]{20,})`),
	},
	{
		name: "PRIVATE_KEY",
		re:   regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`),
	},
}

// Redact replaces sensitive tokens and credentials with safe placeholders.
func (r *RedactionRecord) Redact(text string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := text

	for _, rule := range secretRules {
		switch rule.name {
		case "GENERIC_API_KEY":
			result = rule.re.ReplaceAllStringFunc(result, func(match string) string {
				sub := rule.re.FindStringSubmatch(match)
				if len(sub) == 3 {
					keyName := sub[1]
					secretVal := sub[2]
					mask := r.getOrCreateMask(secretVal, keyName)
					return fmt.Sprintf("%s=%s", keyName, mask)
				}
				return match
			})
		case "BEARER_TOKEN":
			result = rule.re.ReplaceAllStringFunc(result, func(match string) string {
				sub := rule.re.FindStringSubmatch(match)
				if len(sub) == 2 {
					secretVal := sub[1]
					mask := r.getOrCreateMask(secretVal, "BEARER_TOKEN")
					return fmt.Sprintf("Bearer %s", mask)
				}
				return match
			})
		default:
			result = rule.re.ReplaceAllStringFunc(result, func(match string) string {
				return r.getOrCreateMask(match, rule.name)
			})
		}
	}

	return result
}

// Restore substitutes placeholders back to the original secrets.
func (r *RedactionRecord) Restore(text string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := text
	for mask, original := range r.MaskToSecret {
		result = strings.ReplaceAll(result, mask, original)
	}
	return result
}

func (r *RedactionRecord) getOrCreateMask(secret, ruleName string) string {
	if mask, ok := r.SecretToMask[secret]; ok {
		return mask
	}
	r.counter++
	mask := fmt.Sprintf("[REDACTED_%s_%d]", strings.ToUpper(ruleName), r.counter)
	r.SecretToMask[secret] = mask
	r.MaskToSecret[mask] = secret
	return mask
}

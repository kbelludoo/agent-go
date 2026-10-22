package network

import (
	"net"
	"testing"
)

func TestParseTokenIPBindings_ExplicitPairs(t *testing.T) {
	tokensEnv := "tokenA@192.168.1.10,tokenB@192.168.1.11,tokenC@192.168.1.12"
	bindings := ParseTokenIPBindings(tokensEnv, "")

	if len(bindings) != 3 {
		t.Fatalf("esperado 3 bindings, obteve %d", len(bindings))
	}

	expected := []struct {
		tok string
		ip  string
	}{
		{"tokenA", "192.168.1.10"},
		{"tokenB", "192.168.1.11"},
		{"tokenC", "192.168.1.12"},
	}

	for i, exp := range expected {
		if bindings[i].Token != exp.tok {
			t.Errorf("[%d] Token esperado %s, obteve %s", i, exp.tok, bindings[i].Token)
		}
		if bindings[i].LocalIP.String() != exp.ip {
			t.Errorf("[%d] IP esperado %s, obteve %s", i, exp.ip, bindings[i].LocalIP.String())
		}
	}
}

func TestParseTokenIPBindings_SeparateLists(t *testing.T) {
	tokensEnv := "tok1,tok2,tok3"
	ipsEnv := "10.0.0.1,10.0.0.2,10.0.0.3"
	bindings := ParseTokenIPBindings(tokensEnv, ipsEnv)

	if len(bindings) != 3 {
		t.Fatalf("esperado 3 bindings, obteve %d", len(bindings))
	}

	// Verify strict 1:1 mapping (no duplicated IPs)
	seenIPs := make(map[string]bool)
	for _, b := range bindings {
		ip := b.LocalIP.String()
		if seenIPs[ip] {
			t.Fatalf("IP duplicado detectado: %s violou regra 1 token = 1 IP", ip)
		}
		seenIPs[ip] = true
	}
}

func TestParseTokenIPBindings_StrictIsolationNoReuse(t *testing.T) {
	// 4 tokens provided, but only 2 unique IPs and no spare local interfaces
	tokensEnv := "tok1,tok2,tok3,tok4"
	ipsEnv := "172.16.0.10,172.16.0.11"

	bindings := ParseTokenIPBindings(tokensEnv, ipsEnv)

	// Verify that each assigned binding has a unique IP
	seenIPs := make(map[string]bool)
	for _, b := range bindings {
		ip := b.LocalIP.String()
		if seenIPs[ip] {
			t.Fatalf("VIOLAÇÃO: O IP %s foi reutilizado por mais de um token!", ip)
		}
		seenIPs[ip] = true
	}

	// Verify that at most unique IPs were bound (never sharing an IP)
	for i := 0; i < len(bindings); i++ {
		for j := i + 1; j < len(bindings); j++ {
			if bindings[i].LocalIP.Equal(bindings[j].LocalIP) {
				t.Fatalf("Tokens %d e %d compartilham o mesmo IP %s!", bindings[i].Index, bindings[j].Index, bindings[i].LocalIP)
			}
		}
	}
}

func TestParseTokenIPBindings_RejectsDuplicateExplicitIP(t *testing.T) {
	// Two tokens attempting to use the same explicit IP
	tokensEnv := "tokA@10.0.0.5,tokB@10.0.0.5"
	bindings := ParseTokenIPBindings(tokensEnv, "")

	if len(bindings) != 1 {
		t.Fatalf("esperado rejeição do segundo token (mesmo IP), obteve %d bindings", len(bindings))
	}
	if bindings[0].Token != "tokA" {
		t.Errorf("esperado token tokA, obteve %s", bindings[0].Token)
	}
}

func TestNewDedicatedIPClient(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	client := NewDedicatedIPClient(ip, 0)
	if client == nil {
		t.Fatal("client é nil")
	}
	if client.Timeout <= 0 {
		t.Error("timeout padrão não configurado")
	}
}

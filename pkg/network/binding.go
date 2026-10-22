package network

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// TokenBinding represents an API token bound strictly to a dedicated egress local IP.
type TokenBinding struct {
	Index   int
	Token   string
	LocalIP net.IP
}

// ParseTokenIPBindings parses token-to-IP bindings enforcing strict 1:1 mapping:
// each token MUST have its own unique, dedicated IP address.
//
// Supported formats:
// 1. Explicit pair in tokensEnv: "token1@10.0.0.1,token2@10.0.0.2" or "token1:10.0.0.1,token2:10.0.0.2"
// 2. Separate lists: tokensEnv="token1,token2" and ipsEnv="10.0.0.1,10.0.0.2"
//
// If ipsEnv has fewer IPs than tokens, unassigned local network interface IPs are auto-assigned.
// If no unique IP is available for a token, it is rejected to ensure no IP is shared between tokens.
func ParseTokenIPBindings(tokensEnv, ipsEnv string) []TokenBinding {
	if strings.TrimSpace(tokensEnv) == "" {
		return nil
	}

	// 1. Parse configured IPs from ipsEnv
	var configuredIPs []net.IP
	if strings.TrimSpace(ipsEnv) != "" {
		for _, part := range strings.Split(ipsEnv, ",") {
			part = strings.TrimSpace(part)
			if ip := net.ParseIP(part); ip != nil {
				configuredIPs = append(configuredIPs, ip)
			}
		}
	}

	// 2. Discover local interface IPs for additional fallback pool
	var localPool []net.IP
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
					localPool = append(localPool, ip)
				}
			}
		}
	}

	usedIPs := make(map[string]bool)
	var bindings []TokenBinding

	rawTokens := strings.Split(tokensEnv, ",")
	cfgIndex := 0

	for i, raw := range rawTokens {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		var tok string
		var explicitIP net.IP

		// Check for "token@ip" format
		if atIdx := strings.LastIndex(raw, "@"); atIdx != -1 {
			tok = strings.TrimSpace(raw[:atIdx])
			candidate := strings.TrimSpace(raw[atIdx+1:])
			explicitIP = net.ParseIP(candidate)
		} else if colonIdx := strings.LastIndex(raw, ":"); colonIdx != -1 {
			// Check for "token:ip" format only if right side is a valid IP
			candidate := strings.TrimSpace(raw[colonIdx+1:])
			if ip := net.ParseIP(candidate); ip != nil {
				tok = strings.TrimSpace(raw[:colonIdx])
				explicitIP = ip
			} else {
				tok = raw
			}
		} else {
			tok = raw
		}

		if tok == "" {
			continue
		}

		var assignedIP net.IP

		if explicitIP != nil {
			ipStr := explicitIP.String()
			if usedIPs[ipStr] {
				log.Printf("[network] ⚠️ AVISO: Token %d ignorado: IP %s já está em uso por outro token (1 token = 1 IP estrito)", i+1, ipStr)
				continue
			}
			assignedIP = explicitIP
			usedIPs[ipStr] = true
		} else {
			// Find next unassigned IP from configuredIPs
			for cfgIndex < len(configuredIPs) {
				cand := configuredIPs[cfgIndex]
				cfgIndex++
				if !usedIPs[cand.String()] {
					assignedIP = cand
					usedIPs[cand.String()] = true
					break
				}
			}

			// If configured IPs exhausted, look in local interface pool
			if assignedIP == nil {
				for _, cand := range localPool {
					if !usedIPs[cand.String()] {
						assignedIP = cand
						usedIPs[cand.String()] = true
						break
					}
				}
			}
		}

		if assignedIP == nil {
			log.Printf("[network] ⚠️ AVISO: Token %d ignorado: nenhum IP exclusivo disponível (1 token = 1 IP estrito)", i+1)
			continue
		}

		bindings = append(bindings, TokenBinding{
			Index:   len(bindings) + 1,
			Token:   tok,
			LocalIP: assignedIP,
		})
	}

	return bindings
}

// NewDedicatedIPClient returns an http.Client whose dialer binds all connections
// strictly to the given local IP address.
func NewDedicatedIPClient(localIP net.IP, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if localIP != nil {
		dialer.LocalAddr = &net.TCPAddr{IP: localIP}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if localIP != nil {
				log.Printf("[dedicated-ip] 🌐 Requisição para %s enviada exclusivamente pelo IP local %s", addr, localIP.String())
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

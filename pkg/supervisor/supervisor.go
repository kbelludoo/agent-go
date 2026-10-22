package supervisor

import (
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

// BotTarget represents a monitored bot process or port.
type BotTarget struct {
	Port int    `json:"port"`
	Name string `json:"name"`
}

// BotHealthStatus represents the live health check result of a bot.
type BotHealthStatus struct {
	Port      int           `json:"port"`
	Name      string        `json:"name"`
	Online    bool          `json:"online"`
	Latency   time.Duration `json:"latency_ms"`
	CheckedAt time.Time     `json:"checked_at"`
}

// DefaultVPSBots returns the complete inventory of the 15 bots running on the VPS.
var DefaultVPSBots = []BotTarget{
	{Port: 3000, Name: "Huntera"},
	{Port: 3001, Name: "Tibidle"},
	{Port: 3002, Name: "Sigma"},
	{Port: 3003, Name: "Leveling"},
	{Port: 3005, Name: "PokeLegacy"},
	{Port: 3006, Name: "PokeIdle"},
	{Port: 3007, Name: "PokeIdleWorld"},
	{Port: 3009, Name: "Kick Miner"},
	{Port: 3010, Name: "Twitch Miner"},
	{Port: 3011, Name: "LoL Idle"},
	{Port: 3012, Name: "Lux Idle"},
	{Port: 3013, Name: "FarmBot Ricochet"},
	{Port: 3014, Name: "PokeTrue"},
	{Port: 3015, Name: "PokePixel"},
	{Port: 8090, Name: "Baiak Idle"},
}

// ProbePort performs a fast, non-blocking TCP probe to verify if a port is listening.
func ProbePort(host string, port int, timeout time.Duration) (bool, time.Duration) {
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}
	target := fmt.Sprintf("%s:%d", host, port)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	latency := time.Since(start)

	if err != nil {
		return false, 0
	}
	conn.Close()
	return true, latency
}

// CheckAllBots scans all targets concurrently using goroutines in < 50ms.
func CheckAllBots(host string, bots []BotTarget, timeout time.Duration) []BotHealthStatus {
	results := make([]BotHealthStatus, len(bots))
	var wg sync.WaitGroup

	for i, b := range bots {
		wg.Add(1)
		go func(idx int, target BotTarget) {
			defer wg.Done()
			online, lat := ProbePort(host, target.Port, timeout)
			results[idx] = BotHealthStatus{
				Port:      target.Port,
				Name:      target.Name,
				Online:    online,
				Latency:   lat,
				CheckedAt: time.Now(),
			}
		}(i, b)
	}

	wg.Wait()
	return results
}

// FormatHealthTable formats the bot status into an ultra-compact summary.
func FormatHealthTable(statuses []BotHealthStatus) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== GO-SUPERVISOR: STATUS DE %d BOTS ===\n", len(statuses)))

	onlineCount := 0
	for _, s := range statuses {
		icon := "🔴"
		latStr := "OFFLINE"
		if s.Online {
			icon = "🟢"
			latStr = fmt.Sprintf("%.2fms", float64(s.Latency.Microseconds())/1000.0)
			onlineCount++
		}
		sb.WriteString(fmt.Sprintf("%s :%-5d %-18s [%s]\n", icon, s.Port, s.Name, latStr))
	}

	sb.WriteString(fmt.Sprintf("\nResumo: %d/%d bots ativos online.\n", onlineCount, len(statuses)))
	return sb.String()
}

// GetMemoryStats returns real Go runtime memory in MB.
func GetMemoryStats() (allocMB, totalMB, sysMB float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	toMB := func(bytes uint64) float64 {
		return float64(bytes) / 1024.0 / 1024.0
	}

	return toMB(m.Alloc), toMB(m.TotalAlloc), toMB(m.Sys)
}

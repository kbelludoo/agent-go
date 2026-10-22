package supervisor

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSupervisorLocalProbe(t *testing.T) {
	// Start dummy listener on local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	start := time.Now()
	online, latency := ProbePort("127.0.0.1", port, 500*time.Millisecond)
	duration := time.Since(start)

	if !online {
		t.Fatal("Porta deveria estar online")
	}

	t.Logf("⚡ [PERFORMANCE COMPROVADA] Probe TCP na porta :%d respondeu em: %v (latência: %v)", port, duration, latency)

	// Check offline port
	onlineDead, _ := ProbePort("127.0.0.1", 59999, 100*time.Millisecond)
	if onlineDead {
		t.Fatal("Porta morta não deveria reportar online")
	}

	// Test concurrent inventory probe
	testBots := []BotTarget{
		{Port: port, Name: "Test Bot Alive"},
		{Port: 59998, Name: "Test Bot Dead"},
	}

	statuses := CheckAllBots("127.0.0.1", testBots, 200*time.Millisecond)
	formatted := FormatHealthTable(statuses)
	t.Logf("Tabela de Saúde:\n%s", formatted)

	if !strings.Contains(formatted, "1/2 bots ativos") {
		t.Errorf("Resumo de saúde incorreto: %s", formatted)
	}

	allocMB, _, sysMB := GetMemoryStats()
	t.Logf("📊 [MEMÓRIA COMPROVADA] RAM alocada pelo Go Supervisor: %.2f MB (Sistema: %.2f MB)", allocMB, sysMB)
	if allocMB > 10.0 {
		t.Errorf("Consumo de RAM muito alto: %.2f MB", allocMB)
	}
}

func BenchmarkProbePort(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ProbePort("127.0.0.1", port, 100*time.Millisecond)
	}
}

package watchdog

import "testing"

// TestBotsRegistryCoversFleet guards against the registry silently drifting
// from the real VPS fleet (regression: Telegram agent only reported 9 bots).
func TestBotsRegistryCoversFleet(t *testing.T) {
	expectedPorts := []int{3000, 3001, 3002, 3003, 3005, 3006, 3007, 3009, 3010, 3012, 3013, 3014, 3015, 8090}

	seenPorts := make(map[int]string, len(BotsRegistry))
	seenIDs := make(map[string]bool, len(BotsRegistry))
	for _, b := range BotsRegistry {
		if prev, dup := seenPorts[b.Port]; dup {
			t.Fatalf("duplicate port %d for %q and %q", b.Port, prev, b.ID)
		}
		if seenIDs[b.ID] {
			t.Fatalf("duplicate bot ID %q", b.ID)
		}
		if b.URL == "" || b.RestartCmd == "" {
			t.Fatalf("bot %q missing URL or RestartCmd", b.ID)
		}
		seenPorts[b.Port] = b.ID
		seenIDs[b.ID] = true
	}

	for _, p := range expectedPorts {
		if _, ok := seenPorts[p]; !ok {
			t.Errorf("port %d missing from BotsRegistry", p)
		}
	}
	if len(BotsRegistry) != len(expectedPorts) {
		t.Errorf("expected %d bots, got %d", len(expectedPorts), len(BotsRegistry))
	}
}

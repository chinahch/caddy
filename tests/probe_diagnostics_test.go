package probe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticLogClassificationNeverReturnsSecrets(t *testing.T) {
	raw := "user=private-uuid password=pass token=private-key authentication failed\nnode 999: address already in use uuid=private\nfailed to start kernel: secret-config\nTLS handshake certificate private-key"
	logs := classifyLogs(raw)
	b, _ := json.Marshal(logs)
	if len(logs) != 4 || strings.Contains(string(b), "private-") || strings.Contains(string(b), "secret-config") || strings.Contains(string(b), "node 999") {
		t.Fatalf("unsafe classification %s", b)
	}
	for _, l := range logs {
		if l.Scope != "machine" || l.Count != 1 {
			t.Fatal(l)
		}
	}
}
func TestDiagnosticTCPAndUDPDoNotConfuseSocketStates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sockets")
	os.WriteFile(p, []byte("header\n 0: 00000000:1F90 00000000:0000 07 rest\n"), 0600)
	if v := inspectSockets([]string{p}, 8080, "udp"); v == nil || !*v {
		t.Fatal("UDP bound socket not recognized")
	}
	if v := inspectSockets([]string{p}, 8080, "tcp"); v == nil || *v {
		t.Fatal("UDP was treated as TCP listener")
	}
	if v := inspectSockets([]string{p + "missing"}, 8080, "udp"); v != nil {
		t.Fatal("unavailable socket table reported false")
	}
}

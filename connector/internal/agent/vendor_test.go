package agent

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPinnedMTGAcceptsGeneratedConfigAndServesMetrics(t *testing.T) {
	binary := os.Getenv("MTG_TEST_BINARY")
	if binary == "" {
		t.Skip("optional pinned vendor binary not supplied")
	}
	freePort := func() int {
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}
	port, metrics := freePort(), freePort()
	spec := Spec{NodeID: 1, Kind: "mtg", UserID: 5, Port: port, MetricsPort: metrics, Credential: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"}
	p, e := spec.Plan(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	// Test-only isolation: bind the public listener to loopback and prevent
	// public-IP discovery/blocklist downloads during the local smoke test.
	p.Files["mtg.toml"] = "public-ipv4 = \"127.0.0.1\"\n" + strings.ReplaceAll(p.Files["mtg.toml"], "0.0.0.0:", "127.0.0.1:") + "\n[defense.blocklist]\nenabled = false\n"
	if e = p.Write(); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, p.Args...)
	configureChild(cmd)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { cancel(); cmd.Wait() }()
	s := NewSupervisor("", "", nil)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, e := s.get(ctx, &child{spec: spec}, "/metrics")
		if e == nil {
			if _, e = ReadMTG(strings.NewReader(string(data))); e != nil {
				t.Fatal(e)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("pinned MTG did not serve metrics from generated config")
}

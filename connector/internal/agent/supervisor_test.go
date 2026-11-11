package agent

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSupervisorStopsRealProcessAfterLeaseExpires(t *testing.T) {
	bins := t.TempDir()
	binary := filepath.Join(bins, "cloudflared")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./testdata/child")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	q, _ := OpenSpool(t.TempDir())
	m := NewSupervisor(bins, t.TempDir(), q)
	defer m.Stop()
	spec := Spec{NodeID: 1, Kind: "cloudflared", Port: 18001, MetricsPort: port, Credential: "test-only-tunnel-credential"}
	if e = m.Apply(context.Background(), Desired{Connectors: []Spec{spec}, LeaseUntil: time.Now().Add(time.Minute)}); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		states, e := m.Sample(context.Background())
		if e == nil && states[1] == "ready" {
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("test connector did not become ready")
	}
	m.Expire(time.Now().Add(2 * time.Minute))
	conn, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), time.Second)
	if e == nil {
		conn.Close()
		t.Fatal("expired connector kept running")
	}
}

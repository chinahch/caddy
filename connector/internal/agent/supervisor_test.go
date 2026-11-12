package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestRejectedNodeIsPausedWithoutStartingProcess(t *testing.T) {
	q, _ := OpenSpool(t.TempDir())
	q.Enqueue(Report{NodeID: 1, ReportID: "rejected", PeriodStart: time.Now().Add(-time.Second), PeriodEnd: time.Now(), Entries: []Entry{{UserID: 5, Up: 101}}})
	paths, _ := q.Pending()
	if e := q.Reject(paths[0], 1, 403); e != nil {
		t.Fatal(e)
	}
	m := NewSupervisor(t.TempDir(), t.TempDir(), q)
	defer m.Stop()
	spec := Spec{NodeID: 1, Kind: "mtg", UserID: 5, Port: 8443, MetricsPort: 21001, Credential: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"}
	if e := m.Apply(context.Background(), Desired{Connectors: []Spec{spec}, LeaseUntil: time.Now().Add(time.Minute)}); e != nil {
		t.Fatal("blocked node attempted to execute", e)
	}
	states, _ := m.Sample(context.Background())
	if len(m.children) != 0 || states[1] != "queue_blocked" {
		t.Fatal("rejected node not isolated")
	}
}

func TestSlowStatusCannotStarveDurableReportDelivery(t *testing.T) {
	bins := t.TempDir()
	binary := filepath.Join(bins, "cloudflared")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if b, e := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./testdata/child").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	entered := make(chan struct{}, 1)
	spec := Spec{NodeID: 1, Kind: "cloudflared", Port: 18001, MetricsPort: port, Credential: "test-private-tunnel-token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/server/connectors":
			json.NewEncoder(w).Encode(Desired{Connectors: []Spec{spec}, LeaseUntil: time.Now().Add(time.Minute)})
		case "/api/v3/server/connector/status":
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			select {
			case entered <- struct{}{}:
			default:
			}
			<-r.Context().Done()
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer server.Close()
	c, _ := NewClient(Config{PanelURL: server.URL, MachineID: 1}, "test")
	q, _ := OpenSpool(t.TempDir())
	for id := 1; id <= 64; id++ {
		if e := q.Enqueue(Report{NodeID: int64(id), ReportID: fmt.Sprintf("slow-status-%d", id), PeriodStart: time.Now().Add(-time.Second), PeriodEnd: time.Now(), Entries: []Entry{{UserID: int64(id), Up: 101}}}); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := NewSupervisor(bins, t.TempDir(), q)
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx, c) }()
	defer func() { cancel(); <-done }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("status request not reached")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending, _ := q.Pending()
		if len(pending) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("status consumed the reporting budget")
}

package machine

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/panel"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMachineAcceptsTenSecondDiscovery(t *testing.T) {
	o := &Orchestrator{}
	o.applyIntervals(panel.MachineBaseConfig{PullInterval: 10, PushInterval: 10})
	if o.pullInterval != 10*time.Second || o.pushInterval != 10*time.Second {
		t.Fatalf("panel intervals ignored: pull=%s push=%s", o.pullInterval, o.pushInterval)
	}
}
func TestEmptyMachineImmediatelyReportsAndLaterStartsAssignedNode(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	var assigned atomic.Bool
	var switchKernel atomic.Bool
	var invalid atomic.Bool
	var discoveries atomic.Int32
	status := make(chan struct{}, 4)
	var badAuth atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/server/probes" {
			if r.Method != "GET" || r.URL.Query().Get("machine_id") != "7" || r.URL.Query().Get("token") != "" || r.Header.Get("Authorization") != "Bearer fixture-private-machine-token" {
				badAuth.Store(true)
			}
			fmt.Fprint(w, `{"tasks":[]}`)
			return
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("machine_id") != "7" || r.URL.Query().Get("token") != "fixture-private-machine-token" || r.URL.Query().Get("node_id") != "11" {
				badAuth.Store(true)
			}
		} else {
			var auth struct {
				MachineID int    `json:"machine_id"`
				Token     string `json:"token"`
			}
			json.NewDecoder(r.Body).Decode(&auth)
			if auth.MachineID != 7 || auth.Token != "fixture-private-machine-token" {
				badAuth.Store(true)
			}
		}
		switch r.URL.Path {
		case "/api/v2/server/machine/nodes":
			discoveries.Add(1)
			nodes := `[]`
			if assigned.Load() {
				nodes = `[{"id":11,"type":"vless","name":"later node","kernel_type":"xray"}]`
				if switchKernel.Load() {
					nodes = `[{"id":11,"type":"shadowsocks","name":"edited node","kernel_type":"singbox"}]`
				}
			}
			fmt.Fprintf(w, `{"nodes":%s,"base_config":{"pull_interval":10,"push_interval":10}}`, nodes)
		case "/api/v2/server/machine/status":
			select {
			case status <- struct{}{}:
			default:
			}
			fmt.Fprint(w, `{"ok":true}`)
		case "/api/v2/server/handshake":
			fmt.Fprint(w, `{"websocket":{"enabled":false},"settings":{"push_interval":10,"pull_interval":10}}`)
		case "/api/v2/server/config":
			if invalid.Load() {
				w.WriteHeader(503)
				return
			}
			if switchKernel.Load() {
				fmt.Fprintf(w, `{"protocol":"shadowsocks","listen_ip":"127.0.0.1","server_port":%d,"cipher":"aes-128-gcm","kernel_type":"singbox","base_config":{"pull_interval":10,"push_interval":10}}`, port)
				return
			}
			fmt.Fprintf(w, `{"protocol":"vless","listen_ip":"127.0.0.1","server_port":%d,"network":"tcp","decryption":"none","kernel_type":"xray","base_config":{"pull_interval":10,"push_interval":10}}`, port)
		case "/api/v2/server/user":
			fmt.Fprint(w, `{"users":[{"id":1,"uuid":"a75ef122-f3c4-4b8b-9d90-1e2a03634bf1"}]}`)
		case "/api/v2/server/report":
			fmt.Fprint(w, `{"ok":true}`)
		default:
			t.Errorf("unexpected orchestrator endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cfg := &config.Config{Panel: config.PanelConfig{URL: srv.URL}, Machine: &config.MachineConfig{MachineID: 7, Token: "fixture-private-machine-token"}, Kernel: config.KernelConfig{Type: "xray", ConfigDir: t.TempDir(), LogLevel: "error"}}
	cfg.Node = config.NodeConfig{TrackInterval: 10, DeviceReportInterval: 30, PushInterval: 10, PullInterval: 10}
	cfg.WS = config.WSConfig{DiscoveryInterval: 300, StatusInterval: 10}
	o := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- o.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("orchestrator did not stop")
		}
	}()
	select {
	case <-status:
	case <-time.After(2 * time.Second):
		t.Fatal("zero-node machine has no immediate heartbeat")
	}
	assigned.Store(true)
	deadline := time.Now().Add(13 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if e == nil {
			c.Close()
			started = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !started {
		t.Fatal("assigned node did not automatically discover and start its native listener")
	}
	// Editing protocol/kernel tears down the old native service. A transiently
	// invalid saved snapshot must be retried after correction, without reinstall.
	o.mu.Lock()
	old := o.nodes[11]
	o.mu.Unlock()
	switchKernel.Store(true)
	invalid.Store(true)
	o.rediscover(ctx)
	o.mu.Lock()
	failed := o.nodes[11]
	o.mu.Unlock()
	if failed == old {
		t.Fatal("edited protocol/kernel did not replace the old service")
	}
	select {
	case <-failed.done:
	case <-time.After(3 * time.Second):
		t.Fatal("invalid snapshot did not fail startup")
	}
	invalid.Store(false)
	o.rediscover(ctx)
	o.mu.Lock()
	replacement := o.nodes[11]
	o.mu.Unlock()
	if replacement == failed {
		t.Fatal("corrected node snapshot did not retry the failed service")
	}
	if replacement.kernel.Type != "singbox" {
		t.Fatal("edited kernel did not take effect")
	}
	deadline = time.Now().Add(4 * time.Second)
	started = false
	for time.Now().Before(deadline) {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if e == nil {
			c.Close()
			started = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !started {
		t.Fatal("corrected node did not start its replacement kernel listener")
	}
	if discoveries.Load() < 2 {
		t.Fatal("machine did not poll for a later assignment")
	}
	if badAuth.Load() {
		t.Fatal("orchestrator did not preserve machine/node request identity")
	}
}

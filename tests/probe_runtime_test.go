package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/panel"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func testPort(t *testing.T) int {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}
func testInbound(t *testing.T, port int) func() {
	cfg := map[string]any{"log": map[string]any{"loglevel": "none"}, "inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": port, "protocol": "vless", "settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": "00000000-0000-4000-8000-000000000001"}}}}}, "outbounds": []any{map[string]any{"protocol": "freedom"}}}
	raw, _ := json.Marshal(cfg)
	v, e := serial.DecodeJSONConfig(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	c, e := v.Build()
	if e != nil {
		t.Fatal(e)
	}
	x, e := core.New(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = x.Start(); e != nil {
		t.Fatal(e)
	}
	return func() { x.Close() }
}
func TestRealUserInboundProbeAndWrongAuthentication(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer target.Close()
	roots := x509.NewCertPool()
	roots.AddCert(target.Certificate())
	port := testPort(t)
	closeServer := testInbound(t, port)
	defer closeServer()
	for _, k := range []string{"xray", "singbox"} {
		t.Run(k, func(t *testing.T) {
			for _, good := range []bool{true, false} {
				u := "00000000-0000-4000-8000-000000000001"
				if !good {
					u = "00000000-0000-4000-8000-000000000002"
				}
				o := panel.OutboundConfig{Tag: "probe", Protocol: "vless"}
				if k == "xray" {
					o.Settings = map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": port, "users": []any{map[string]any{"id": u, "encryption": "none"}}}}}
				} else {
					o.Settings = map[string]any{"server": "127.0.0.1", "server_port": port, "uuid": u}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				code, _ := execute(ctx, Payload{Kernel: k, Outbound: o, Target: target.URL}, &tls.Config{RootCAs: roots})
				cancel()
				if good && code != "" {
					t.Fatalf("real user roundtrip: %s", code)
				}
				if !good && code == "" {
					t.Fatal("wrong user authenticated")
				}
			}
		})
	}
}
func TestProbeTimeoutAndCleanup(t *testing.T) {
	blackhole, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer blackhole.Close()
	go func() {
		c, e := blackhole.Accept()
		if e == nil {
			defer c.Close()
			io.Copy(io.Discard, c)
		}
	}()
	p, _ := strconv.Atoi(blackhole.Addr().String()[len("127.0.0.1:"):])
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	code, _ := execute(ctx, Payload{Kernel: "singbox", Outbound: panel.OutboundConfig{Tag: "probe", Protocol: "socks", Settings: map[string]any{"server": "127.0.0.1", "server_port": p, "version": "5"}}, Target: "https://example.invalid"}, nil)
	if code != "timeout" {
		t.Fatal(code)
	}
}
func TestTaskBoundsRejectTargetsAndExpiry(t *testing.T) {
	for _, task := range []Task{{ID: "a", Lease: "l", Expires: time.Now().Add(time.Minute), Payload: Payload{Target: "https://attacker.invalid", Kernel: "xray"}}, {ID: "a", Lease: "l", Expires: time.Now().Add(-time.Minute), Payload: Payload{Target: Target, Kernel: "xray"}}} {
		r := Run(context.Background(), task)
		if r.State == "succeeded" {
			t.Fatal("unbounded task executed")
		}
	}
}

func TestProbeWireRetainsTLSStreamEnvelope(t *testing.T) {
	var p Payload
	e := json.Unmarshal([]byte(`{"kernel_type":"xray","outbound":{"tag":"probe","protocol":"vless","settings":{"vnext":[]},"stream_settings":{"security":"tls","tlsSettings":{"serverName":"public.example"}}},"target":"https://www.gstatic.com/generate_204"}`), &p)
	if e != nil || p.Outbound.StreamSettings["security"] != "tls" {
		t.Fatal("TLS stream envelope was lost", e, p.Outbound)
	}
}

package singbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/cedar2025/xboard-node/internal/panel"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singJSON "github.com/sagernet/sing/common/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestLumaSingboxDataPlane(t *testing.T) {
	if os.Getenv("LUMA_DATA_PLANE_TESTS") != "1" {
		t.Skip("opt-in isolated loopback data plane")
	}
	data, e := os.ReadFile(os.Getenv("LUMA_PROTOCOL_FIXTURES"))
	if e != nil {
		t.Fatal(e)
	}
	var fixtures []map[string]json.RawMessage
	json.Unmarshal(data, &fixtures)
	tc := lumaTestCert(t)
	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("local TLS target")) }))
	target.TLS = &tls.Config{MinVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"}}
	target.EnableHTTP2 = true
	target.StartTLS()
	defer target.Close()
	for _, f := range fixtures {
		var k, name string
		json.Unmarshal(f["kernel_type"], &k)
		json.Unmarshal(f["fixture_name"], &name)
		if k != "singbox" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(f)
			var nc panel.NodeConfig
			json.Unmarshal(raw, &nc)
			nc.ListenIP = "127.0.0.1"
			nc.ServerPort = lumaFreePort(t)
			if nc.TLS == 2 {
				nc.TLSSettings["dest"] = target.Listener.Addr().String()
			}
			cfg := buildConfig(config.KernelConfig{Type: "singbox", LogLevel: "error"}, model.NodeSpecFromPanel(&nc), []model.UserSpec{{ID: 1, UUID: "00000000-0000-4000-8000-000000000001"}}, tc)
			cfg["route"] = M{"rules": []M{{"ip_cidr": []string{"127.0.0.0/8"}, "action": "route", "outbound": "direct"}}}
			server := lumaStartSingbox(t, cfg)
			defer server.Close()
			var oc panel.OutboundConfig
			json.Unmarshal(f["fixture_outbound"], &oc)
			oc.Settings["server"] = "127.0.0.1"
			oc.Settings["server_port"] = nc.ServerPort
			if nc.TLS == 1 {
				oc.Settings["tls"].(map[string]any)["insecure"] = true
			}
			clientPort := lumaFreePort(t)
			client := lumaStartSingbox(t, M{"log": M{"level": "error"}, "inbounds": []M{{"type": "socks", "listen": "127.0.0.1", "listen_port": clientPort}}, "outbounds": []M{outboundConfigToSingbox(model.NodeSpecFromPanel(&panel.NodeConfig{CustomOutbounds: []panel.OutboundConfig{oc}}).CustomOutbounds[0])}})
			defer client.Close()
			lumaProbeSocks(t, clientPort)
		})
	}
}
func lumaStartSingbox(t *testing.T, cfg M) *box.Box {
	t.Helper()
	data, _ := json.Marshal(cfg)
	ctx := include.Context(context.Background())
	opts, e := singJSON.UnmarshalExtendedContext[option.Options](ctx, data)
	if e != nil {
		t.Fatal(e)
	}
	instance, e := box.New(box.Options{Context: ctx, Options: opts})
	if e != nil {
		t.Fatal(e)
	}
	if e = instance.Start(); e != nil {
		instance.Close()
		t.Fatal(e)
	}
	return instance
}

func lumaFreePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
func lumaProbeSocks(t *testing.T, port int) {
	t.Helper()
	echo, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer echo.Close()
	go func() {
		c, e := echo.Accept()
		if e == nil {
			defer c.Close()
			c.SetDeadline(time.Now().Add(15 * time.Second))
			io.Copy(c, c)
		}
	}()
	c, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(15 * time.Second))
	c.Write([]byte{5, 1, 0})
	hello := make([]byte, 2)
	if _, e = io.ReadFull(c, hello); e != nil || !bytes.Equal(hello, []byte{5, 0}) {
		t.Fatal("SOCKS hello", e)
	}
	request := []byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 0}
	binary.BigEndian.PutUint16(request[8:], uint16(echo.Addr().(*net.TCPAddr).Port))
	c.Write(request)
	head := make([]byte, 4)
	if _, e = io.ReadFull(c, head); e != nil || head[1] != 0 {
		t.Fatal("SOCKS CONNECT", e, head)
	}
	n := 6
	if head[3] == 4 {
		n = 18
	}
	tail := make([]byte, n)
	if _, e = io.ReadFull(c, tail); e != nil {
		t.Fatal(e)
	}
	payload := []byte("Luma actual proxy roundtrip")
	c.Write(payload)
	got := make([]byte, len(payload))
	if _, e = io.ReadFull(c, got); e != nil || !bytes.Equal(got, payload) {
		t.Fatal("proxy data plane did not return authenticated payload", e)
	}
}

package xray

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/cedar2025/xboard-node/internal/panel"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestLumaXrayDataPlane(t *testing.T) {
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
		if k != "xray" {
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
			cfg := buildConfig(config.KernelConfig{Type: "xray", LogLevel: "error"}, model.NodeSpecFromPanel(&nc), []model.UserSpec{{ID: 1, UUID: "00000000-0000-4000-8000-000000000001"}}, tc)
			cfg["routing"] = M{"rules": []M{{"type": "field", "ip": []string{"127.0.0.0/8"}, "outboundTag": "direct"}}}
			server := lumaStartXray(t, cfg)
			defer server.Close()
			var oc panel.OutboundConfig
			json.Unmarshal(f["fixture_outbound"], &oc)
			next := oc.Settings["vnext"].([]any)[0].(map[string]any)
			next["address"] = "127.0.0.1"
			next["port"] = nc.ServerPort
			if nc.TLS == 1 {
				cert, _ := pem.Decode(tc.CertPEM)
				hash := sha256.Sum256(cert.Bytes)
				oc.StreamSettings["tlsSettings"].(map[string]any)["pinnedPeerCertSha256"] = hex.EncodeToString(hash[:])
			}
			clientPort := lumaFreePort(t)
			client := lumaStartXray(t, M{"log": M{"loglevel": "error"}, "inbounds": []M{{"listen": "127.0.0.1", "port": clientPort, "protocol": "socks", "settings": M{"auth": "noauth"}}}, "outbounds": []M{outboundConfigToXray(model.NodeSpecFromPanel(&panel.NodeConfig{CustomOutbounds: []panel.OutboundConfig{oc}}).CustomOutbounds[0])}})
			defer client.Close()
			lumaProbeSocks(t, clientPort)
		})
	}
}
func lumaStartXray(t *testing.T, cfg M) *core.Instance {
	t.Helper()
	b, _ := json.Marshal(cfg)
	parsed, e := serial.DecodeJSONConfig(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	built, e := parsed.Build()
	if e != nil {
		t.Fatal(e)
	}
	instance, e := core.New(built)
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

package panel

import (
	"github.com/cedar2025/xboard-node/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLumaLandingCapabilityOnScopedClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("X-Luma-Capabilities"), "landing-v1") {
			t.Error("landing capability missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"protocol":"vless","server_port":443}`))
	}))
	defer server.Close()
	c := NewClient(config.PanelConfig{URL: server.URL, Token: "test", MachineID: 1}).ForNode(3)
	if _, e := c.GetConfig(); e != nil {
		t.Fatal(e)
	}
}

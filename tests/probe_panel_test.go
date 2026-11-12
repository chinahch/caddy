package panel

import (
	"context"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProbeAPIBearerIdentityBoundsAndSecretlessErrors(t *testing.T) {
	var badAuth atomic.Bool
	var posted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer machine-private" || r.URL.Query().Get("token") != "" {
			badAuth.Store(true)
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("machine_id") != "7" {
				badAuth.Store(true)
			}
			w.Write([]byte(`{"tasks":[{"id":"00000000-0000-4000-8000-000000000001","lease":"private-lease"}]}`))
			return
		}
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		if in["token"] != nil || in["machine_id"] != float64(7) || in["lease"] != "private-lease" {
			badAuth.Store(true)
		}
		posted.Store(true)
		w.WriteHeader(503)
		w.Write([]byte("machine-private remote-password-private"))
	}))
	defer srv.Close()
	c := NewClient(config.PanelConfig{URL: srv.URL, Token: "machine-private", MachineID: 7})
	tasks, e := c.GetProbes(context.Background())
	if e != nil || len(tasks) != 1 {
		t.Fatal(e, len(tasks))
	}
	e = c.SendProbeResult(context.Background(), "00000000-0000-4000-8000-000000000001", map[string]any{"lease": "private-lease", "state": "failed"})
	if e == nil || strings.Contains(e.Error(), "private") || badAuth.Load() || !posted.Load() {
		t.Fatal("unsafe probe API", e, badAuth.Load(), posted.Load())
	}
}
func TestProbeAPIRefusesCredentialRedirect(t *testing.T) {
	var called atomic.Bool
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called.Store(true) }))
	defer dest.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 307) }))
	defer origin.Close()
	c := NewClient(config.PanelConfig{URL: origin.URL, Token: "private", MachineID: 7})
	if _, e := c.GetProbes(context.Background()); e == nil || called.Load() {
		t.Fatal("probe credentials followed redirect", e)
	}
}

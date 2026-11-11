package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPanelRequestsUseScopedBearerAndRejectRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-node-token" || r.URL.Query().Get("token") != "" {
			t.Error("unsafe authentication")
		}
		if r.URL.Path == "/api/v3/server/connectors" {
			if r.URL.Query().Get("machine_id") != "7" {
				t.Error("machine scope missing")
			}
			json.NewEncoder(w).Encode(Desired{Connectors: []Spec{}, LeaseUntil: time.Now().Add(time.Minute)})
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c, e := NewClient(Config{PanelURL: server.URL, MachineID: 7}, "private-node-token")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Desired(context.Background()); e != nil {
		t.Fatal(e)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, server.URL, 302) }))
	defer redirect.Close()
	c, e = NewClient(Config{PanelURL: redirect.URL, MachineID: 7}, "private-node-token")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Desired(context.Background()); e == nil {
		t.Fatal("redirect followed")
	}
	if _, e = NewClient(Config{PanelURL: "http://remote.example", MachineID: 7}, "token"); e == nil {
		t.Fatal("nonlocal plaintext panel accepted")
	}
}

func TestQueueRetryPreservesExactIntegerIdentity(t *testing.T) {
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		if len(bodies) == 1 {
			w.WriteHeader(500)
		} else {
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer server.Close()
	c, _ := NewClient(Config{PanelURL: server.URL, MachineID: 1}, "test")
	q, _ := OpenSpool(t.TempDir())
	if e := q.Enqueue(Report{NodeID: 3, ReportID: "same-batch", PeriodStart: time.Now().Add(-time.Minute), PeriodEnd: time.Now(), Entries: []Entry{{UserID: 9007199254740993, Up: 101}}}); e != nil {
		t.Fatal(e)
	}
	if e := c.Flush(context.Background(), q); e == nil {
		t.Fatal("failed report acknowledged")
	}
	if e := c.Flush(context.Background(), q); e != nil {
		t.Fatal(e)
	}
	if len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) || !bytes.Contains(bodies[1], []byte("9007199254740993")) {
		t.Fatal("retry changed raw bytes")
	}
}

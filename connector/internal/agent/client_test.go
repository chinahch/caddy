package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestReportsSustain64ActiveConnectorsWithPanelLatency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c, _ := NewClient(Config{PanelURL: server.URL, MachineID: 1}, "test")
	q, _ := OpenSpool(t.TempDir())
	for round := 0; round < 3; round++ {
		for id := 1; id <= 64; id++ {
			if e := q.Enqueue(Report{NodeID: int64(id), ReportID: fmt.Sprintf("round-%d-node-%d", round, id), PeriodStart: time.Now().Add(-time.Second), PeriodEnd: time.Now(), Entries: []Entry{{UserID: int64(id), Up: 101}}}); e != nil {
				t.Fatal(e)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		e := c.Flush(ctx, q)
		cancel()
		pending, _ := q.Pending()
		if e != nil || len(pending) != 0 {
			t.Fatal("healthy 64-node queue grew", len(pending), e)
		}
	}
}
func TestRejectedNodeCannotBlockOtherReports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body Report
		json.NewDecoder(r.Body).Decode(&body)
		if body.NodeID == 1 {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c, _ := NewClient(Config{PanelURL: server.URL, MachineID: 1}, "test")
	q, _ := OpenSpool(t.TempDir())
	for id := 1; id <= 2; id++ {
		if e := q.Enqueue(Report{NodeID: int64(id), ReportID: fmt.Sprintf("node-%d", id), PeriodStart: time.Now().Add(-time.Minute), PeriodEnd: time.Now(), Entries: []Entry{{UserID: int64(id), Up: 101}}}); e != nil {
			t.Fatal(e)
		}
	}
	_ = c.Flush(context.Background(), q)
	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatal("disabled node blocked another report", len(pending))
	}
	rejected, e := os.ReadDir(filepath.Join(q.dir, "rejected", "1"))
	if e != nil || len(rejected) == 0 {
		t.Fatal("rejected report was discarded", e)
	}
}

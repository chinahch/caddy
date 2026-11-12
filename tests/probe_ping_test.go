package probe

import (
	"context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"os"
	"testing"
	"time"
)

func TestSuccessfulConnectivityMeasuresPingToLandingOnly(t *testing.T) {
	task := Task{ID: "job", Lease: "lease", Expires: time.Now().Add(time.Minute), Payload: Payload{Kernel: "xray", Target: Target, PingHost: "exit.example"}}
	called := 0
	request := func(context.Context, Payload) (string, int64) { return "", 9999 }
	ping := func(_ context.Context, host string) (*float64, string) {
		called++
		if host != "exit.example" {
			t.Fatal("wrong ping destination")
		}
		v := 12.5
		return &v, ""
	}
	r := runTask(context.Background(), task, request, ping)
	if r.State != "succeeded" || r.Ping == nil || *r.Ping != 12.5 || r.Latency != 0 || called != 1 {
		t.Fatal(r, called)
	}
	r = runTask(context.Background(), task, func(context.Context, Payload) (string, int64) { return "proxy_failed", 12 }, ping)
	if r.State != "failed" || called != 1 {
		t.Fatal("ping ran before connectivity passed")
	}
	r = runTask(context.Background(), task, request, func(context.Context, string) (*float64, string) { return nil, "icmp_timeout" })
	if r.State != "succeeded" || r.Ping != nil || r.PingError != "icmp_timeout" {
		t.Fatal("ICMP failure became proxy failure", r)
	}
	task.Payload.PingHost = ""
	r = runTask(context.Background(), task, request, ping)
	if called != 1 || r.Ping != nil {
		t.Fatal("direct exit invented latency")
	}
}

type pingReplyFixture struct {
	reply     []byte
	writes    int
	unrelated bool
}

func (f *pingReplyFixture) WriteTo(b []byte, _ net.Addr) (int, error) {
	f.writes++
	m, _ := icmp.ParseMessage(1, b)
	f.reply, _ = (&icmp.Message{Type: ipv4.ICMPTypeEchoReply, Body: m.Body}).Marshal(nil)
	return len(b), nil
}
func (f *pingReplyFixture) ReadFrom(b []byte) (int, net.Addr, error) {
	if f.unrelated {
		f.unrelated = false
		return copy(b, f.reply), &net.IPAddr{IP: net.ParseIP("127.0.0.2")}, nil
	}
	return copy(b, f.reply), &net.IPAddr{IP: net.ParseIP("127.0.0.1")}, nil
}
func (f *pingReplyFixture) SetDeadline(time.Time) error { return nil }
func (f *pingReplyFixture) Close() error                { return nil }
func TestICMPRepliesMatchDestinationIDSequenceAndPayload(t *testing.T) {
	f := &pingReplyFixture{unrelated: true}
	ms, code := pingSocket(context.Background(), f, net.ParseIP("127.0.0.1"))
	if code != "" || ms == nil || *ms < 0 || f.writes != 3 {
		t.Fatal(ms, code, f.writes)
	}
}
func TestRawICMPLoopback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	v, code := pingRTT(ctx, "127.0.0.1")
	if code == "icmp_unavailable" {
		if os.Getenv("LUMA_REQUIRE_ICMP") == "1" {
			t.Fatal("required raw ICMP unavailable")
		}
		t.Skip("raw ICMP requires host capability")
	}
	if code != "" || v == nil || *v < 0 {
		t.Fatal(v, code)
	}
}

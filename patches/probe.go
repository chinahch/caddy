// Package probe executes bounded, real HTTP proxy roundtrips, never commands.
package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/cedar2025/xboard-node/internal/kernel/singbox"
	"github.com/cedar2025/xboard-node/internal/kernel/xray"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/cedar2025/xboard-node/internal/panel"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const Target = "https://www.gstatic.com/generate_204"

type Payload struct {
	Action    string               `json:"action,omitempty"`
	NodeID    int64                `json:"node_id,omitempty"`
	Port      int                  `json:"expected_port,omitempty"`
	Transport string               `json:"transport,omitempty"`
	Users     int                  `json:"eligible_users"`
	Active    bool                 `json:"active"`
	Protocol  string               `json:"protocol,omitempty"`
	Kernel    string               `json:"kernel_type"`
	Outbound  panel.OutboundConfig `json:"outbound"`
	Target    string               `json:"target"`
	PingHost  string               `json:"ping_host,omitempty"`
}
type Task struct {
	ID      string    `json:"id"`
	Lease   string    `json:"lease"`
	Expires time.Time `json:"expires_at"`
	Timeout int       `json:"timeout_ms"`
	Payload Payload   `json:"payload"`
}
type Result struct {
	Lease       string      `json:"lease"`
	State       string      `json:"state"`
	Latency     int64       `json:"latency_ms"`
	Error       string      `json:"error_code"`
	Diagnostics *Diagnostic `json:"diagnostics,omitempty"`
	Ping        *float64    `json:"ping_ms,omitempty"`
	PingError   string      `json:"ping_error_code,omitempty"`
}

func Run(ctx context.Context, t Task) Result {
	return runTask(ctx, t, func(ctx context.Context, p Payload) (string, int64) { return execute(ctx, p, nil) }, pingRTT)
}
func runTask(ctx context.Context, t Task, request func(context.Context, Payload) (string, int64), ping func(context.Context, string) (*float64, string)) Result {
	r := Result{Lease: t.Lease, State: "failed"}
	if time.Now().After(t.Expires) {
		r.State = "expired"
		r.Error = "expired"
		return r
	}
	if t.Payload.Target != Target || len(t.Payload.PingHost) > 253 || strings.ContainsAny(t.Payload.PingHost, " /\\\r\n\t") || len(t.ID) > 40 || t.ID == "" || t.Lease == "" || len(t.Lease) > 100 || (t.Payload.Kernel != "xray" && t.Payload.Kernel != "singbox") {
		r.Error = "configuration_unavailable"
		return r
	}
	timeout := 10 * time.Second
	if until := time.Until(t.Expires); until < timeout {
		timeout = until
	}
	if t.Timeout > 0 && t.Timeout < 10000 {
		timeout = time.Duration(t.Timeout) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r.Error, _ = request(ctx, t.Payload)
	if r.Error == "" {
		r.State = "succeeded"
		if t.Payload.PingHost != "" {
			r.Ping, r.PingError = ping(ctx, t.Payload.PingHost)
		}
	}
	return r
}
func execute(ctx context.Context, p Payload, tlsConfig *tls.Config) (code string, latency int64) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return "configuration_unavailable", 0
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	out := model.NodeSpecFromPanel(&panel.NodeConfig{CustomOutbounds: []panel.OutboundConfig{p.Outbound}}).CustomOutbounds[0]
	var closeClient func()
	if p.Kernel == "xray" {
		closeClient, e = xray.StartProbe(port, out)
	} else if p.Kernel == "singbox" {
		closeClient, e = singbox.StartProbe(port, out)
	} else {
		return "unsupported", 0
	}
	if e != nil {
		return "configuration_unavailable", 0
	}
	defer closeClient()
	d, e := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", port), nil, &net.Dialer{})
	if e != nil {
		return "configuration_unavailable", 0
	}
	cd, ok := d.(proxy.ContextDialer)
	if !ok {
		return "configuration_unavailable", 0
	}
	transport := &http.Transport{DialContext: cd.DialContext, TLSClientConfig: tlsConfig, DisableKeepAlives: true, MaxResponseHeaderBytes: 8192}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, "GET", p.Target, nil)
	if e != nil {
		return "configuration_unavailable", 0
	}
	resp, e := client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return "timeout", 0
		}
		return "proxy_failed", 0
	}
	defer resp.Body.Close()
	_, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if e != nil {
		if ctx.Err() != nil {
			return "timeout", 0
		}
		return "target_failed", 0
	}
	if resp.StatusCode != 204 {
		return "target_failed", 0
	}
	return "", 0
}

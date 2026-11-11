package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PanelURL  string `json:"panel_url"`
	MachineID int64  `json:"machine_id"`
	NodeID    int64  `json:"node_id"`
	TokenEnv  string `json:"token_env"`
	BinDir    string `json:"bin_dir"`
	StateDir  string `json:"state_dir"`
}
type Desired struct {
	Connectors []Spec    `json:"connectors"`
	LeaseUntil time.Time `json:"lease_until"`
}
type Client struct {
	cfg   Config
	token string
	http  *http.Client
}

func NewClient(cfg Config, token string) (*Client, error) {
	u, e := url.Parse(cfg.PanelURL)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, errors.New("panel_url must be an HTTPS origin")
	}
	ip := net.ParseIP(u.Hostname())
	local := ip != nil && ip.IsLoopback() || u.Hostname() == "localhost"
	if u.Scheme != "https" && (u.Scheme != "http" || !local) {
		return nil, errors.New("panel_url requires HTTPS; HTTP is allowed only on loopback")
	}
	if token == "" || len(token) > 256 || (cfg.MachineID <= 0) == (cfg.NodeID <= 0) {
		return nil, errors.New("one node or machine identity and token required")
	}
	cfg.PanelURL = strings.TrimRight(cfg.PanelURL, "/")
	return &Client{cfg, token, &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.cfg.PanelURL+path, reader)
	if e != nil {
		return nil, errors.New("invalid panel request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Luma-Capabilities", "connector-v1")
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, errors.New("panel request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("panel returned HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return nil, errors.New("panel response unavailable or oversized")
	}
	return b, nil
}
func (c *Client) Desired(ctx context.Context) (Desired, error) {
	q := url.Values{}
	if c.cfg.MachineID > 0 {
		q.Set("machine_id", strconv.FormatInt(c.cfg.MachineID, 10))
	} else {
		q.Set("node_id", strconv.FormatInt(c.cfg.NodeID, 10))
	}
	b, e := c.request(ctx, "GET", "/api/v3/server/connectors?"+q.Encode(), nil)
	if e != nil {
		return Desired{}, e
	}
	var d Desired
	if json.Unmarshal(b, &d) != nil {
		return d, errors.New("invalid desired-state response")
	}
	now := time.Now()
	if !d.LeaseUntil.After(now) || d.LeaseUntil.After(now.Add(2*time.Minute)) {
		return d, errors.New("invalid connector lease")
	}
	return d, ValidateDesired(d.Connectors)
}
func (c *Client) Status(ctx context.Context, node int64, kind, state string) error {
	_, e := c.request(ctx, "POST", "/api/v3/server/connector/status", map[string]any{"machine_id": c.cfg.MachineID, "node_id": node, "kind": kind, "state": state})
	return e
}
func (c *Client) Flush(ctx context.Context, q *Spool) error {
	paths, e := q.Pending()
	if e != nil {
		return e
	}
	for i, path := range paths {
		if i >= 4 {
			break
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var report struct {
			Report
			MachineID int64 `json:"machine_id"`
		}
		if json.Unmarshal(data, &report) != nil {
			return errors.New("invalid queued report")
		}
		report.MachineID = c.cfg.MachineID
		if _, e = c.request(ctx, "POST", "/api/v3/server/connector/report", report); e != nil {
			return e
		}
		if e = q.Ack(path); e != nil {
			return e
		}
	}
	return nil
}

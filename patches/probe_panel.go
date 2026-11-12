package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Probe requests use a deadline, refuse redirects and never include remote body/errors.
func (c *Client) probeRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	u := c.baseURL + path
	var reader io.Reader
	if method == "GET" {
		q := url.Values{}
		q.Set("machine_id", fmt.Sprint(c.machineID))
		u += "?" + q.Encode()
	} else {
		raw, e := json.Marshal(body)
		if e != nil {
			return nil, errors.New("probe encoding failed")
		}
		reader = bytes.NewReader(raw)
	}
	req, e := http.NewRequestWithContext(ctx, method, u, reader)
	if e != nil {
		return nil, errors.New("probe request failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	client := *c.httpClient
	client.Timeout = 12 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("probe transport failed")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, errors.New("probe service unavailable")
	}
	return resp, nil
}
func (c *Client) GetProbes(ctx context.Context) ([]json.RawMessage, error) {
	resp, e := c.probeRequest(ctx, "GET", "/api/v3/server/probes", nil)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	var out struct {
		Tasks []json.RawMessage `json:"tasks"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 128<<10)).Decode(&out) != nil || len(out.Tasks) > 1 {
		return nil, errors.New("invalid probe tasks")
	}
	return out.Tasks, nil
}
func (c *Client) SendProbeResult(ctx context.Context, id string, r any) error {
	if len(id) != 36 {
		return errors.New("invalid probe id")
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return errors.New("invalid probe result")
	}
	body := map[string]any{}
	if json.Unmarshal(raw, &body) != nil {
		return errors.New("invalid probe result")
	}
	body["machine_id"] = c.machineID
	resp, e := c.probeRequest(ctx, "POST", "/api/v3/server/probes/"+id+"/result", body)
	if e != nil {
		return e
	}
	resp.Body.Close()
	return nil
}

package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type child struct {
	spec     Spec
	hash     [32]byte
	plan     Plan
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	done     chan struct{}
	run      string
	seq      uint64
	previous Counters
	since    time.Time
}
type Supervisor struct {
	binDir, stateDir string
	queue            *Spool
	children         map[int64]*child
	blocked          map[int64]Spec
	lease            time.Time
	local            *http.Client
}

func NewSupervisor(bins, state string, q *Spool) *Supervisor {
	return &Supervisor{binDir: bins, stateDir: state, queue: q, children: map[int64]*child{}, blocked: map[int64]Spec{}, local: &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (s *Supervisor) Apply(ctx context.Context, d Desired) error {
	if e := ValidateDesired(d.Connectors); e != nil {
		return e
	}
	desired := map[int64]Spec{}
	rejected, e := s.queue.RejectedNodes()
	if e != nil {
		return e
	}
	s.blocked = map[int64]Spec{}
	for _, v := range d.Connectors {
		if rejected[v.NodeID] {
			s.blocked[v.NodeID] = v
		} else {
			desired[v.NodeID] = v
		}
	}
	for id, c := range s.children {
		v, exists := desired[id]
		b, _ := json.Marshal(v)
		hash := sha256.Sum256(b)
		dead := false
		select {
		case <-c.done:
			dead = true
		default:
		}
		if !exists || hash != c.hash || dead {
			if c.spec.Kind == "mtg" && !dead {
				_ = s.sampleOne(ctx, c)
			}
			s.stopChild(c)
			delete(s.children, id)
		}
	}
	s.lease = d.LeaseUntil
	for _, v := range d.Connectors {
		if _, allowed := desired[v.NodeID]; !allowed {
			continue
		}
		if s.children[v.NodeID] != nil {
			continue
		}
		plan, e := v.Plan(s.binDir, s.stateDir)
		if e != nil {
			return e
		}
		if e = plan.Write(); e != nil {
			return errors.New("cannot write private connector configuration")
		}
		childCtx, cancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(childCtx, plan.Binary, plan.Args...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + plan.Dir, "LANG=C.UTF-8"}
		if runtimeSystemRoot := os.Getenv("SystemRoot"); runtimeSystemRoot != "" {
			cmd.Env = append(cmd.Env, "SystemRoot="+runtimeSystemRoot)
		}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		configureChild(cmd)
		if e = cmd.Start(); e != nil {
			cancel()
			return errors.New("cannot start allowlisted connector binary")
		}
		seed := make([]byte, 16)
		if _, e = rand.Read(seed); e != nil {
			cancel()
			cmd.Wait()
			return e
		}
		b, _ := json.Marshal(v)
		c := &child{spec: v, hash: sha256.Sum256(b), plan: plan, cmd: cmd, cancel: cancel, done: make(chan struct{}), run: hex.EncodeToString(seed), since: time.Now().UTC()}
		s.children[v.NodeID] = c
		go func() { cmd.Wait(); close(c.done) }()
	}
	return nil
}
func (s *Supervisor) stopChild(c *child) {
	c.cancel()
	select {
	case <-c.done:
	case <-time.After(4 * time.Second):
		c.cmd.Process.Kill()
		<-c.done
	}
	for name := range c.plan.Files {
		os.Remove(filepath.Join(c.plan.Dir, name))
	}
}
func (s *Supervisor) Stop() {
	for id, c := range s.children {
		s.stopChild(c)
		delete(s.children, id)
	}
}
func (s *Supervisor) Expire(now time.Time) {
	if !s.lease.IsZero() && !now.Before(s.lease) {
		s.Stop()
	}
}
func (s *Supervisor) get(ctx context.Context, c *child, path string) ([]byte, error) {
	u := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(c.spec.MetricsPort)) + path
	req, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, e
	}
	resp, e := s.local.Do(req)
	if e != nil {
		return nil, errors.New("connector health unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("connector is not ready")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return nil, errors.New("connector metrics oversized or unavailable")
	}
	return b, nil
}
func (s *Supervisor) sampleOne(ctx context.Context, c *child) error {
	b, e := s.get(ctx, c, "/metrics")
	if e != nil {
		return e
	}
	totals, e := ReadMTG(bytes.NewReader(b))
	if e != nil {
		return e
	}
	if totals.Up < c.previous.Up || totals.Down < c.previous.Down {
		return errors.New("connector counters decreased")
	}
	up, down := totals.Up-c.previous.Up, totals.Down-c.previous.Down
	now := time.Now().UTC()
	if up > 0 || down > 0 {
		r := Report{NodeID: c.spec.NodeID, ReportID: fmt.Sprintf("%s-%d", c.run, c.seq), PeriodStart: c.since, PeriodEnd: now, Entries: []Entry{{UserID: c.spec.UserID, Up: up, Down: down}}}
		if e = s.queue.Enqueue(r); e != nil {
			return fmt.Errorf("queue blocked: %w", e)
		}
		c.seq++
	}
	c.previous = totals
	c.since = now
	return nil
}
func (s *Supervisor) Sample(ctx context.Context) (map[int64]string, error) {
	states := map[int64]string{}
	for id := range s.blocked {
		states[id] = "queue_blocked"
	}
	var failure error
	for id, c := range s.children {
		select {
		case <-c.done:
			states[id] = "unhealthy"
			continue
		default:
		}
		if c.spec.Kind == "cloudflared" {
			if _, e := s.get(ctx, c, "/ready"); e != nil {
				states[id] = "starting"
			} else {
				states[id] = "ready"
			}
			continue
		}
		if e := s.sampleOne(ctx, c); e != nil {
			states[id] = "unhealthy"
			if errors.Is(e, ErrNodeBlocked) {
				states[id] = "queue_blocked"
				s.blocked[id] = c.spec
				s.stopChild(c)
				delete(s.children, id)
				continue
			}
			if strings.HasPrefix(e.Error(), "queue blocked:") {
				states[id] = "queue_blocked"
				s.stopChild(c)
				failure = errors.New("traffic queue blocked; affected connector stopped")
			}
		} else {
			states[id] = "ready"
		}
	}
	return states, failure
}
func (s *Supervisor) pauseRejected() error {
	rejected, e := s.queue.RejectedNodes()
	if e != nil {
		return e
	}
	for id, c := range s.children {
		if rejected[id] {
			s.blocked[id] = c.spec
			s.stopChild(c)
			delete(s.children, id)
		}
	}
	return nil
}
func (s *Supervisor) Run(ctx context.Context, client *Client) error {
	reportCtx, stopReports := context.WithCancel(ctx)
	wake := make(chan struct{}, 1)
	reportsDone := make(chan struct{})
	go func() { defer close(reportsDone); client.RunReports(reportCtx, s.queue, wake) }()
	defer func() { stopReports(); <-reportsDone; s.Stop() }()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		d, e := client.Desired(requestCtx)
		cancel()
		if e == nil {
			if e = s.Apply(ctx, d); e != nil {
				return e
			}
		}
		s.Expire(time.Now())
		if e = s.pauseRejected(); e != nil {
			return e
		}
		sampleCtx, stopSample := context.WithTimeout(ctx, 5*time.Second)
		states, sampleError := s.Sample(sampleCtx)
		stopSample()
		select {
		case wake <- struct{}{}:
		default:
		}
		s.Expire(time.Now())
		statuses := []Status{}
		for id, state := range states {
			if c := s.children[id]; c != nil {
				statuses = append(statuses, Status{id, c.spec.Kind, state})
			} else if spec, ok := s.blocked[id]; ok {
				statuses = append(statuses, Status{id, spec.Kind, state})
			}
		}
		statusCtx, stopStatus := context.WithTimeout(ctx, 3*time.Second)
		client.Statuses(statusCtx, statuses)
		stopStatus()
		s.Expire(time.Now())
		if sampleError != nil {
			return sampleError
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

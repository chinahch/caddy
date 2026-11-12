package machine

import (
	"context"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/probe"
	"time"
)

// One independent worker bounds machine concurrency without delaying discovery/heartbeats.
func (o *Orchestrator) runProbes(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		tasks, e := o.client.GetProbes(ctx)
		if e == nil {
			for _, raw := range tasks {
				var task probe.Task
				if json.Unmarshal(raw, &task) != nil {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				var result probe.Result
				if task.Payload.Action == "diagnostic" {
					result = o.diagnose(ctx, task)
				} else {
					result = probe.Run(ctx, task)
				}
				if ctx.Err() != nil {
					return
				}
				_ = o.client.SendProbeResult(ctx, task.ID, result)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (o *Orchestrator) diagnose(ctx context.Context, t probe.Task) probe.Result {
	r := probe.Result{Lease: t.Lease, State: "failed"}
	p := t.Payload
	if p.NodeID <= 0 || p.Port < 1 || p.Port > 65535 || (p.Transport != "tcp" && p.Transport != "udp") || p.Users < 0 {
		r.Error = "configuration_unavailable"
		return r
	}
	if time.Now().After(t.Expires) {
		r.State = "expired"
		r.Error = "expired"
		return r
	}
	deadline := time.Now().Add(10 * time.Second)
	if t.Expires.Before(deadline) {
		deadline = t.Expires
	}
	job, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	runtime := "unassigned"
	certificateState:="none"
	o.mu.Lock()
	if h, ok := o.nodes[int(p.NodeID)]; ok {
		select {
		case <-h.done:
			runtime = "exited"
		default:
			runtime = "stopped"
			if h.service != nil && h.service.ProbeKernelRunning() {
				runtime = "running"
			}
			if h.service!=nil{certificateState=h.service.ProbeCertificateState()}
		}
	}
	o.mu.Unlock()
	start := time.Now()
	r.Diagnostics = probe.CollectDiagnostics(job, p, runtime)
	if certificateState=="pending"||certificateState=="failed"||certificateState=="ready"{
		severity:="info";if certificateState=="failed"{severity="error"}else if certificateState=="pending"{severity="warning"}
		r.Diagnostics.Findings=append(r.Diagnostics.Findings,probe.Finding{Code:"certificate_"+certificateState,Message:"TLS certificate state reported by the assigned node.",Severity:severity})
	}
	r.Latency = time.Since(start).Milliseconds()
	if job.Err() != nil {
		r.Error = "timeout"
		return r
	}
	r.State = "succeeded"
	return r
}

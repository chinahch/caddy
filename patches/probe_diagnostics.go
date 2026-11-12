package probe

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Finding struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}
type LogEvidence struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	Scope   string `json:"scope"`
}
type Diagnostic struct {
	Version   int           `json:"version"`
	NodeID    int64         `json:"node_id"`
	Port      int           `json:"expected_port"`
	Transport string        `json:"transport"`
	Listening *bool         `json:"listening"`
	Users     int           `json:"eligible_users"`
	Runtime   string        `json:"runtime_state"`
	Findings  []Finding     `json:"findings"`
	Logs      []LogEvidence `json:"logs"`
}

var diagnosticMessages = map[string]string{
	"no_users": "No eligible users; the native kernel intentionally does not open a listener.", "listener_missing": "The expected local port is not listening.", "listener_present": "The expected local port is bound; this does not verify authentication or external reachability.", "listener_unknown": "Local socket inspection is unavailable on this operating system.", "runtime_stopped": "The assigned native kernel is stopped.", "runtime_exited": "The assigned node service has exited.", "runtime_unassigned": "The machine has no running service handle for this node.", "port_in_use": "Machine logs contain an address-already-in-use error; it may concern another node.", "auth_failure": "Machine logs contain an authentication failure; it may concern another node.", "config_rejected": "Machine logs contain a configuration or kernel-start rejection; it may concern another node.", "certificate": "Machine logs contain a certificate or TLS handshake error; it may concern another node.", "logs_unavailable": "Recent machine service logs were unavailable.", "machine_log_evidence": "Log classifications are machine-level evidence and are not attributed to this node.", "node_disabled": "This logical node is disabled in the panel.", "connector_only": "MTProxy uses an independent connector; native runtime status does not describe its service."}

func CollectDiagnostics(ctx context.Context, p Payload, runtime string) *Diagnostic {
	d := &Diagnostic{Version: 1, NodeID: p.NodeID, Port: p.Port, Transport: p.Transport, Users: p.Users, Runtime: runtime, Findings: []Finding{}, Logs: []LogEvidence{}}
	add := func(code, severity string) {
		d.Findings = append(d.Findings, Finding{code, diagnosticMessages[code], severity})
	}
	if !p.Active {
		add("node_disabled", "warning")
	}
	if p.Protocol == "mtproxy" {
		d.Runtime = "unsupported"
		add("connector_only", "info")
	} else {
		if p.Users == 0 {
			add("no_users", "warning")
		}
		switch runtime {
		case "stopped":
			add("runtime_stopped", "warning")
		case "exited":
			add("runtime_exited", "error")
		case "unassigned":
			add("runtime_unassigned", "warning")
		}
	}
	paths := []string{"/proc/net/tcp", "/proc/net/tcp6"}
	if p.Transport == "udp" {
		paths = []string{"/proc/net/udp", "/proc/net/udp6"}
	}
	d.Listening = inspectSockets(paths, p.Port, p.Transport)
	if d.Listening == nil {
		add("listener_unknown", "warning")
	} else if *d.Listening {
		add("listener_present", "info")
	} else {
		add("listener_missing", "error")
	}
	logs, available := recentLogs(ctx)
	if available {
		d.Logs = classifyLogs(logs)
		add("machine_log_evidence", "info")
	} else {
		add("logs_unavailable", "warning")
	}
	return d
}
func inspectSockets(paths []string, port int, transport string) *bool {
	available := false
	found := false
	want := "0A"
	if transport == "udp" {
		want = "07"
	}
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			continue
		}
		available = true
		s := bufio.NewScanner(io.LimitReader(f, 1<<20))
		for s.Scan() {
			fields := strings.Fields(s.Text())
			if len(fields) < 4 || fields[3] != want {
				continue
			}
			address := strings.Split(fields[1], ":")
			if len(address) != 2 {
				continue
			}
			p, e := strconv.ParseInt(address[1], 16, 32)
			if e == nil && int(p) == port {
				found = true
				break
			}
		}
		f.Close()
		if found {
			break
		}
	}
	if !available {
		return nil
	}
	return &found
}

// Only stable classifications leave this process. Untrusted journal text is discarded.
func classifyLogs(raw string) []LogEvidence {
	counts := map[string]int{}
	lines := strings.Split(raw, "\n")
	if len(lines) > 120 {
		lines = lines[len(lines)-120:]
	}
	for _, line := range lines {
		s := strings.ToLower(line)
		code := ""
		switch {
		case strings.Contains(s, "address already in use"):
			code = "port_in_use"
		case strings.Contains(s, "authentication failed") || strings.Contains(s, "invalid user") || strings.Contains(s, "invalid uuid"):
			code = "auth_failure"
		case strings.Contains(s, "certificate") || strings.Contains(s, "tls handshake"):
			code = "certificate"
		case strings.Contains(s, "failed to start kernel") || strings.Contains(s, "invalid config") || strings.Contains(s, "initial setup:"):
			code = "config_rejected"
		}
		if code != "" {
			counts[code]++
		}
	}
	out := []LogEvidence{}
	for _, code := range []string{"port_in_use", "auth_failure", "config_rejected", "certificate"} {
		if counts[code] > 0 {
			out = append(out, LogEvidence{code, diagnosticMessages[code], counts[code], "machine"})
		}
	}
	return out
}

type boundedLog struct{ bytes.Buffer }

func (b *boundedLog) Write(p []byte) (int, error) {
	n := len(p)
	left := (64 << 10) - b.Len()
	if left > len(p) {
		left = len(p)
	}
	if left > 0 {
		b.Buffer.Write(p[:left])
	}
	return n, nil
}
func recentLogs(ctx context.Context) (string, bool) {
	for _, binary := range []string{"/usr/bin/journalctl", "/bin/journalctl"} {
		if info, e := os.Lstat(binary); e == nil && info.Mode().IsRegular() {
			job, cancel := context.WithTimeout(ctx, 3*time.Second)
			cmd := exec.CommandContext(job, binary, "--unit=caddy.service", "--since=-10min", "--lines=120", "--no-pager", "--output=cat")
			cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
			var out boundedLog
			cmd.Stdout = &out
			cmd.Stderr = io.Discard
			e = cmd.Run()
			cancel()
			if e == nil && out.Len() > 0 {
				return out.String(), true
			}
		}
	}
	// OpenRC files may not carry timestamps: keep only ISO-timestamped lines from
	// the last ten minutes; older/untimestamped output must never look recent.
	var raw strings.Builder
	available := false
	cutoff := time.Now().Add(-10 * time.Minute)
	for _, path := range []string{"/var/log/caddy.err", "/var/log/caddy.log"} {
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() {
			continue
		}
		f, e := os.Open(path)
		if e != nil {
			continue
		}
		size := info.Size()
		if size > 64<<10 {
			f.Seek(size-(64<<10), io.SeekStart)
		}
		b, _ := io.ReadAll(io.LimitReader(f, 64<<10))
		f.Close()
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			stamp := strings.Trim(fields[0], "[]")
			at, e := time.Parse(time.RFC3339Nano, stamp)
			if e == nil && at.After(cutoff) && !at.After(time.Now().Add(time.Minute)) {
				available = true
				fmt.Fprintln(&raw, line)
			}
		}
	}
	return raw.String(), available
}

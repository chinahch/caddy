package agent

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Spec struct {
	NodeID      int64  `json:"node_id"`
	Kind        string `json:"kind"`
	UserID      int64  `json:"user_id,omitempty"`
	Port        int    `json:"port"`
	MetricsPort int    `json:"metrics_port"`
	Credential  string `json:"credential"`
	SOCKSProxy  string `json:"socks_proxy,omitempty"`
}

func (s Spec) Validate() error {
	if s.NodeID <= 0 || s.Port < 1 || s.Port > 65535 || s.MetricsPort < 1024 || s.MetricsPort > 65535 || s.Port == s.MetricsPort {
		return errors.New("invalid connector identity or ports")
	}
	switch s.Kind {
	case "cloudflared":
		if len(s.Credential) < 16 || len(s.Credential) > 8192 || strings.ContainsAny(s.Credential, " \r\n\t") {
			return errors.New("invalid tunnel credential")
		}
		if s.SOCKSProxy != "" {
			return errors.New("Cloudflare uses its attached node's outbound routes")
		}
	case "mtg":
		secret, e := hex.DecodeString(s.Credential)
		if e != nil || len(secret) < 18 || len(secret) > 270 || secret[0] != 0xee || s.UserID <= 0 {
			return errors.New("MTG requires one user and a valid FakeTLS secret")
		}
		if s.SOCKSProxy != "" {
			u, e := url.Parse(s.SOCKSProxy)
			if e != nil || u.Scheme != "socks5" || u.Hostname() == "" || u.Port() == "" || u.RawQuery != "" || u.Path != "" {
				return errors.New("MTG landing must be a plain SOCKS5 URI")
			}
		}
	default:
		return errors.New("unsupported connector kind")
	}
	return nil
}
func ValidateDesired(specs []Spec) error {
	if len(specs) > 64 {
		return errors.New("maximum 64 connectors")
	}
	ids := map[int64]bool{}
	ports := map[int]bool{}
	for _, s := range specs {
		if e := s.Validate(); e != nil {
			return e
		}
		if ids[s.NodeID] || ports[s.Port] || ports[s.MetricsPort] {
			return errors.New("duplicate connector or port collision")
		}
		ids[s.NodeID] = true
		ports[s.Port] = true
		ports[s.MetricsPort] = true
	}
	return nil
}

type Plan struct {
	Binary string
	Args   []string
	Dir    string
	Files  map[string]string
}

func (s Spec) Plan(binDir, state string) (Plan, error) {
	if e := s.Validate(); e != nil {
		return Plan{}, e
	}
	dir := filepath.Join(state, fmt.Sprintf("node-%d", s.NodeID))
	p := Plan{Binary: filepath.Join(binDir, s.Kind), Dir: dir, Files: map[string]string{}}
	metrics := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.MetricsPort))
	if s.Kind == "cloudflared" {
		p.Files["token"] = s.Credential
		p.Files["cloudflared.yml"] = "{}\n"
		p.Args = []string{"tunnel", "--config", filepath.Join(dir, "cloudflared.yml"), "--no-autoupdate", "--metrics", metrics, "run", "--token-file", filepath.Join(dir, "token")}
	} else {
		config := "secret = " + strconv.Quote(s.Credential) + "\nbind-to = " + strconv.Quote(net.JoinHostPort("0.0.0.0", strconv.Itoa(s.Port))) + "\nconcurrency = 1024\nauto-update = false\n[stats.prometheus]\nenabled = true\nbind-to = " + strconv.Quote(metrics) + "\nhttp-path = \"/metrics\"\nmetric-prefix = \"mtg\"\n"
		if s.SOCKSProxy != "" {
			u, _ := url.Parse(s.SOCKSProxy)
			u.Fragment = ""
			config += "[network]\nproxies = [" + strconv.Quote(u.String()) + "]\n"
		}
		p.Files["mtg.toml"] = config
		p.Args = []string{"run", filepath.Join(dir, "mtg.toml")}
	}
	return p, nil
}
func (p Plan) Write() error {
	if e := os.MkdirAll(p.Dir, 0700); e != nil {
		return e
	}
	if e := os.Chmod(p.Dir, 0700); e != nil {
		return e
	}
	for name, content := range p.Files {
		if e := atomicWrite(filepath.Join(p.Dir, name), []byte(content)); e != nil {
			return e
		}
	}
	return nil
}
func atomicWrite(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".pending-")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(temp, path); e != nil {
		return e
	}
	return syncDirectory(filepath.Dir(path))
}

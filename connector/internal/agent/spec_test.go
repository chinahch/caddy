package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchPlansNeverPutCredentialsInArgv(t *testing.T) {
	for _, s := range []Spec{{NodeID: 1, Kind: "cloudflared", Port: 18001, MetricsPort: 21001, Credential: "test-private-tunnel-token"}, {NodeID: 2, Kind: "mtg", UserID: 5, Port: 18443, MetricsPort: 21002, Credential: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d", SOCKSProxy: "socks5://u:p@exit.example:1080"}} {
		dir := t.TempDir()
		p, e := s.Plan("/trusted/bin", dir)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(strings.Join(p.Args, " "), s.Credential) {
			t.Fatal("secret in argv")
		}
		if filepath.Dir(p.Binary) != filepath.Clean("/trusted/bin") {
			t.Fatal("untrusted binary")
		}
		if e = p.Write(); e != nil {
			t.Fatal(e)
		}
		for name, content := range p.Files {
			b, e := os.ReadFile(filepath.Join(p.Dir, name))
			if e != nil || string(b) != content {
				t.Fatal("private config missing", e)
			}
		}
	}
	if _, e := (Spec{NodeID: 1, Kind: "sh", Port: 443, MetricsPort: 21001}).Plan("/bin", t.TempDir()); e == nil {
		t.Fatal("arbitrary command accepted")
	}
}
func TestDesiredPortsAndUserValidation(t *testing.T) {
	a := Spec{NodeID: 1, Kind: "mtg", UserID: 5, Port: 8443, MetricsPort: 21001, Credential: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"}
	b := a
	b.NodeID = 2
	if e := ValidateDesired([]Spec{a, b}); e == nil {
		t.Fatal("port collision")
	}
	a.UserID = 0
	if e := ValidateDesired([]Spec{a}); e == nil {
		t.Fatal("unattributed MTG accepted")
	}
}

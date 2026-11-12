package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachineTokenUpdatePreservesConfiguration(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yml")
	creds := filepath.Join(dir, "credentials.env")
	source := `kernel:
  ip_family:
    xray: ipv6_only
instances:
  - id: m1
    panel:
      url: https://panel.example
    machine:
      machine_id: 7
      token_env: OLD_MACHINE_TOKEN
    kernel:
      type: xray
      custom_config_xray: /private/custom.json
`
	os.WriteFile(cfg, []byte(source), 0600)
	os.WriteFile(creds, []byte("OLD_MACHINE_TOKEN=old\nOTHER=keep\n"), 0600)
	t.Setenv("CADDY_MACHINE_TOKEN", "new-private-token")
	args := []string{"--config", cfg, "--credentials", creds, "--panel-url", "https://panel.example", "--machine-id", "8"}
	if err := runMachineToken(args); err == nil {
		t.Fatal("wrong identity accepted")
	}
	args[len(args)-1] = "7"
	if err := runMachineToken(args); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(cfg)
	if string(after) != source {
		t.Fatal("configuration changed")
	}
	after, _ = os.ReadFile(creds)
	if !strings.Contains(string(after), "OLD_MACHINE_TOKEN=new-private-token\n") || !strings.Contains(string(after), "OTHER=keep\n") {
		t.Fatal("credentials not merged")
	}
}

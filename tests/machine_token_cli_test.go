package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestMachineTokenRefusesSharedAndInlineCredentials(t *testing.T) {
	for _, extra := range []string{
		"      token: inline-wins\n",
		"",
	} {
		t.Run(extra, func(t *testing.T) {
			dir := t.TempDir()
			cfg := filepath.Join(dir, "config.yml")
			creds := filepath.Join(dir, "credentials.env")
			source := "instances:\n  - panel:\n      url: https://panel.example\n    machine:\n      machine_id: 7\n      token_env: SHARED_TOKEN\n" + extra
			if extra == "" {
				source += "  - panel:\n      url: https://other.example\n      node_id: 9\n      token_env: SHARED_TOKEN\n"
			}
			os.WriteFile(cfg, []byte(source), 0600)
			os.WriteFile(creds, []byte("SHARED_TOKEN=old\n"), 0600)
			t.Setenv("CADDY_MACHINE_TOKEN", "new-private-token")
			if err := runMachineToken([]string{"--config", cfg, "--credentials", creds, "--panel-url", "https://panel.example", "--machine-id", "7"}); err == nil {
				t.Fatal("unsafe rebind accepted")
			}
			after, _ := os.ReadFile(creds)
			if string(after) != "SHARED_TOKEN=old\n" {
				t.Fatal("refused rebind mutated credentials")
			}
		})
	}
}

func TestConcurrentMachineTokenUpdatesRetainAllBindings(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yml")
	creds := filepath.Join(dir, "credentials.env")
	source := "instances:\n"
	original := ""
	for i := 1; i <= 32; i++ {
		source += fmt.Sprintf("  - panel:\n      url: https://panel.example\n    machine:\n      machine_id: %d\n      token_env: TOKEN_%d\n", i, i)
		original += fmt.Sprintf("TOKEN_%d=old\n", i)
	}
	os.WriteFile(cfg, []byte(source), 0600)
	os.WriteFile(creds, []byte(original), 0600)
	t.Setenv("CADDY_MACHINE_TOKEN", "new-private-token")
	var wg sync.WaitGroup
	for i := 1; i <= 32; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := runMachineToken([]string{"--config", cfg, "--credentials", creds, "--panel-url", "https://panel.example", "--machine-id", fmt.Sprint(id)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	after, _ := os.ReadFile(creds)
	if strings.Contains(string(after), "=old") {
		t.Fatal("concurrent credential update lost a binding")
	}
}

package main

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Update only the credential referenced by an exact existing Machine binding.
// Never rewrite config.yml or include the new token in process arguments/logs.
func runMachineToken(args []string) error {
	fs := flag.NewFlagSet("machine-token", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", defaultConfigPath, "")
	credentials := fs.String("credentials", defaultCredentialsPath, "")
	panel := fs.String("panel-url", "", "")
	id := fs.Int("machine-id", 0, "")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *id <= 0 || *panel == "" {
		return errors.New("valid machine-token connection arguments required")
	}
	token := os.Getenv("CADDY_MACHINE_TOKEN")
	if len(token) < 16 || len(token) > 256 || strings.ContainsAny(token, "\r\n\x00") {
		return errors.New("valid CADDY_MACHINE_TOKEN environment value required")
	}
	root, err := loadWritableRootConfig(*configPath)
	if err != nil {
		return errors.New("cannot read existing machine configuration")
	}
	var key string
	for _, inst := range normalizeRootInstances(root) {
		if inst.Machine != nil && inst.Machine.MachineID == *id && strings.TrimRight(inst.Panel.URL, "/") == strings.TrimRight(*panel, "/") {
			if key != "" {
				return errors.New("ambiguous existing machine identity")
			}
			key = inst.Machine.TokenEnv
		}
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) {
		return errors.New("matching machine with external credential not found; panel URL and ID must match")
	}
	if info, e := os.Lstat(*credentials); e != nil || !info.Mode().IsRegular() {
		return errors.New("existing regular credentials file required")
	}
	tmp, err := os.CreateTemp(filepath.Dir(*credentials), ".machine-token-*")
	if err != nil {
		return errors.New("cannot stage private credentials")
	}
	name := tmp.Name()
	defer os.Remove(name)
	tmp.Close()
	if err = mergeCredentials(*credentials, name, key, token); err != nil {
		return errors.New("cannot update private credentials")
	}
	if err = os.Rename(name, *credentials); err != nil {
		return errors.New("cannot replace private credentials")
	}
	return nil
}

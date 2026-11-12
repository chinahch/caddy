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

// Validate the exact binding before any credential rotation or enrollment claim.
func machineCredentialKey(configPath, panel string, id int) (string, error) {
	info, err := os.Lstat(configPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("existing regular machine configuration required")
	}
	root, err := loadWritableRootConfig(configPath)
	if err != nil {
		return "", errors.New("cannot read existing machine configuration")
	}
	var key string
	for _, inst := range normalizeRootInstances(root) {
		if inst.Machine != nil && inst.Machine.MachineID == id && strings.TrimRight(inst.Panel.URL, "/") == strings.TrimRight(panel, "/") {
			if inst.Machine.Token != "" {
				return "", errors.New("inline machine token overrides environment credentials; migrate the private configuration before rebinding")
			}
			if key != "" {
				return "", errors.New("ambiguous existing machine identity")
			}
			key = inst.Machine.TokenEnv
		}
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) {
		return "", errors.New("matching machine with external credential not found; panel URL and ID must match")
	}
	for _, inst := range normalizeRootInstances(root) {
		selected := inst.Machine != nil && inst.Machine.MachineID == id && strings.TrimRight(inst.Panel.URL, "/") == strings.TrimRight(panel, "/")
		if !selected && (inst.Panel.TokenEnv == key || (inst.Machine != nil && inst.Machine.TokenEnv == key)) {
			return "", errors.New("credential key is shared by another binding; separate its private credentials before rebinding")
		}
	}
	return key, nil
}

// Caller holds the credential lock. Staging succeeds before a remote claim.
func stageMachineCredential(credentials, key, token string) (string, error) {
	if info, e := os.Lstat(credentials); e != nil || !info.Mode().IsRegular() {
		return "", errors.New("existing regular credentials file required")
	}
	tmp, err := os.CreateTemp(filepath.Dir(credentials), ".machine-token-*")
	if err != nil {
		return "", errors.New("cannot stage private credentials")
	}
	name := tmp.Name()
	tmp.Close()
	if err = mergeCredentials(credentials, name, key, token); err != nil {
		os.Remove(name)
		return "", errors.New("cannot update private credentials")
	}
	return name, nil
}

// Update only the credential referenced by an exact existing Machine binding.
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
	key, err := machineCredentialKey(*configPath, *panel, *id)
	if err != nil {
		return err
	}
	unlock, err := lockMachineCredentials(*credentials)
	if err != nil {
		return errors.New("cannot lock private credentials")
	}
	defer unlock()
	name, err := stageMachineCredential(*credentials, key, token)
	if err != nil {
		return err
	}
	defer os.Remove(name)
	if err = os.Rename(name, *credentials); err != nil {
		return errors.New("cannot replace private credentials")
	}
	return nil
}

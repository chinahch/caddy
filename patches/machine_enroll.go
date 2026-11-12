package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type machineEnrollment struct {
	Panel     string `json:"panel"`
	MachineID int    `json:"machine_id"`
	CodeHash  string `json:"code_hash"`
	Token     string `json:"token"`
	Complete  bool   `json:"complete"`
	Fresh     bool   `json:"fresh"`
}

func privateEnrollmentWrite(path string, state machineEnrollment) error {
	raw, _ := json.Marshal(state)
	f, err := os.CreateTemp(filepath.Dir(path), ".enrollment-state-*")
	if err != nil {
		return errors.New("cannot stage private enrollment state")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		return errors.New("cannot persist private enrollment state")
	}
	return nil
}

func validEnrollmentSecret(value string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(value) == 43 && len(raw) == 32
}

// No raw response, request body, HTTP error or URL is included in an error.
// Redirects are refused, including 307/308 which would forward both secrets.
func claimMachineEnrollment(panel, code, token string, id int) error {
	body, _ := json.Marshal(struct {
		MachineID int    `json:"machine_id"`
		Code      string `json:"code"`
		Token     string `json:"token"`
	}{id, code, token})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, panel+"/api/server/enroll", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			var result struct {
				OK        bool `json:"ok"`
				MachineID int  `json:"machine_id"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decodeErr == nil && result.OK && result.MachineID == id {
				return nil
			}
			if resp.StatusCode < 500 && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("machine enrollment refused (HTTP %d); existing configuration preserved", resp.StatusCode)
			}
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return errors.New("machine enrollment timed out; retry the identical command")
			case <-time.After(150 * time.Millisecond):
			}
		}
	}
	return errors.New("machine enrollment unavailable; retry the identical command")
}

// The separately installed connector has its own ownership and service. Never
// revoke its matching credential while silently leaving that worker behind.
func guardMachineConnector(paths []string, panel string, id int) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("cannot inspect connector identity; enrollment refused before claim")
		}
		raw, err := os.ReadFile(path)
		var worker struct {
			Panel     string `json:"panel_url"`
			MachineID int    `json:"machine_id"`
			NodeID    int    `json:"node_id"`
		}
		if err != nil || json.Unmarshal(raw, &worker) != nil || worker.Panel == "" || (worker.MachineID <= 0) == (worker.NodeID <= 0) {
			return errors.New("ambiguous connector identity; enrollment refused before claim")
		}
		if strings.TrimRight(worker.Panel, "/") == panel && worker.MachineID == id {
			return errors.New("matching connector worker requires coordinated credential migration before enrollment; existing connection preserved")
		}
	}
	return nil
}

func runMachineEnroll(args []string) error {
	fs := flag.NewFlagSet("machine-enroll", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", defaultConfigPath, "")
	credentials := fs.String("credentials", defaultCredentialsPath, "")
	panel := fs.String("panel-url", "", "")
	id := fs.Int("machine-id", 0, "")
	root := fs.String("install-root", "/etc/caddy", "")
	meta := fs.String("meta", "", "")
	kernel := fs.String("kernel", "singbox", "")
	health := fs.Int("health-port", 65530, "")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *id <= 0 {
		return errors.New("valid machine-enroll connection arguments required")
	}
	parsed, err := url.Parse(*panel)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Trim(parsed.Path, "/") != "" {
		return errors.New("panel must be an HTTP(S) origin")
	}
	*panel = strings.TrimRight(*panel, "/")
	if err = guardMachineConnector([]string{"/etc/caddy-connect/connect.json", filepath.Join(*root, "connect.json")}, *panel, *id); err != nil {
		return err
	}
	code := os.Getenv("CADDY_ENROLLMENT_CODE")
	if !validEnrollmentSecret(code) {
		return errors.New("valid CADDY_ENROLLMENT_CODE environment value required")
	}
	if *kernel != "singbox" && *kernel != "xray" || *health < 1 || *health > 65535 {
		return errors.New("valid kernel and health port required")
	}
	if err = os.MkdirAll(*root, 0700); err != nil {
		return errors.New("cannot create private install directory")
	}
	statePath := filepath.Join(*root, ".machine-enrollment.json")
	unlockState, err := lockMachineCredentials(statePath)
	if err != nil {
		return errors.New("cannot lock enrollment state")
	}
	defer unlockState()
	sum := sha256.Sum256([]byte(code))
	codeHash := hex.EncodeToString(sum[:])
	var state machineEnrollment
	if info, e := os.Lstat(statePath); e == nil {
		if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return errors.New("regular private enrollment state required")
		}
		raw, e := os.ReadFile(statePath)
		if e != nil || json.Unmarshal(raw, &state) != nil || !validEnrollmentSecret(state.Token) {
			return errors.New("cannot read private enrollment state")
		}
		if state.Panel != *panel || state.MachineID != *id {
			return errors.New("enrollment panel and machine identity conflict")
		}
		if state.CodeHash != codeHash {
			if !state.Complete {
				// A fresh administrator command can renew an expired capability.
				// Keep the durable candidate/identity, including partially installed
				// credentials, so retry never creates a second permanent secret.
				state.CodeHash = codeHash
				if err = privateEnrollmentWrite(statePath, state); err != nil {
					return err
				}
			} else {
				state = machineEnrollment{}
			}
		}
	} else if !os.IsNotExist(e) {
		return errors.New("cannot inspect private enrollment state")
	}
	info, cfgErr := os.Lstat(*configPath)
	existing := cfgErr == nil
	if existing && !info.Mode().IsRegular() {
		return errors.New("regular machine configuration required")
	}
	if cfgErr != nil && !os.IsNotExist(cfgErr) {
		return errors.New("cannot inspect machine configuration")
	}
	var key string
	if existing {
		key, err = machineCredentialKey(*configPath, *panel, *id)
		if err != nil {
			return err
		}
	} else {
		// Only this command's durable state may explain partial fresh credentials.
		if _, e := os.Lstat(*credentials); e == nil && (!state.Fresh || state.Token == "") {
			return errors.New("credentials without matching machine configuration; refusing overwrite")
		}
	}
	if state.Token == "" {
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			return errors.New("cannot generate machine credential")
		}
		state = machineEnrollment{Panel: *panel, MachineID: *id, CodeHash: codeHash, Token: base64.RawURLEncoding.EncodeToString(raw), Fresh: !existing}
		if err = privateEnrollmentWrite(statePath, state); err != nil {
			return err
		}
	}
	unlockCreds, err := lockMachineCredentials(*credentials)
	if err != nil {
		return errors.New("cannot lock private credentials")
	}
	defer unlockCreds()
	if err = guardMachineRuntimeEnvironment(*credentials); err != nil {
		return err
	}
	var stagedCreds, stagedConfig, stagedMeta string
	if existing {
		stagedCreds, err = stageMachineCredential(*credentials, key, state.Token)
		if err != nil {
			return err
		}
		defer os.Remove(stagedCreds)
		if state.Fresh && !state.Complete && *meta != "" {
			f, e := os.CreateTemp(*root, ".machine-enrollment-meta-*")
			if e != nil {
				return errors.New("cannot stage pending installation metadata")
			}
			stagedMeta = f.Name()
			f.Close()
			defer os.Remove(stagedMeta)
			rootConfig, e := loadWritableRootConfig(*configPath)
			if e != nil {
				return errors.New("cannot inspect pending installation metadata")
			}
			if e = writeInstallMetaVersioned(stagedMeta, rootConfig, "v0.1.2-preview", ""); e != nil {
				return errors.New("cannot stage pending installation metadata")
			}
		}
	} else {
		stage, err := os.MkdirTemp(*root, ".machine-enroll-stage-*")
		if err != nil {
			return errors.New("cannot stage machine configuration")
		}
		defer os.RemoveAll(stage)
		stagedConfig = filepath.Join(stage, "config.yml")
		stagedCreds = filepath.Join(stage, "credentials.env")
		stagedMeta = filepath.Join(stage, "install-meta.json")
		err = runConfigInit([]string{"--mode", "machine", "--panel-url", *panel, "--machine-id", strconv.Itoa(*id), "--token", state.Token, "--kernel", *kernel, "--health-port", strconv.Itoa(*health), "--output", stagedConfig, "--credentials-out", stagedCreds, "--meta", stagedMeta, "--install-root", *root, "--version", "v0.1.2-preview"})
		if err != nil {
			return errors.New("cannot stage machine configuration")
		}
		if raw, e := os.ReadFile(*credentials); e == nil {
			wanted, _ := os.ReadFile(stagedCreds)
			if !bytes.Equal(raw, wanted) {
				return errors.New("partial credentials conflict with pending enrollment; refusing overwrite")
			}
		}
	}
	// All existing-binding checks and local staging have succeeded before claim.
	if err = claimMachineEnrollment(*panel, code, state.Token, *id); err != nil {
		return err
	}
	if err = os.Rename(stagedCreds, *credentials); err != nil {
		return errors.New("cannot install claimed credential; retry the identical command")
	}
	if !existing {
		if err = os.Rename(stagedConfig, *configPath); err != nil {
			return errors.New("cannot install claimed configuration; retry the identical command")
		}
	}
	if *meta != "" && stagedMeta != "" {
		if err = os.Rename(stagedMeta, *meta); err != nil {
			return errors.New("cannot install metadata; retry the identical command")
		}
	}
	state.Complete = true
	return privateEnrollmentWrite(statePath, state)
}
